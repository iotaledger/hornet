package mqtt

import (
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	iotago "github.com/iotaledger/iota.go/v3"
)

func publishOnTopic(topic string, payload interface{}) {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		Plugin.LogWarn(err)
		return
	}

	deps.MQTTBroker.Send(topic, jsonPayload)
}

func publishConfirmedMilestone(cachedMs *storage.CachedMilestone) {
	defer cachedMs.Release(true)
	publishMilestoneOnTopic(topicMilestonesConfirmed, cachedMs.Milestone())
}

func publishLatestMilestone(cachedMs *storage.CachedMilestone) {
	defer cachedMs.Release(true)
	publishMilestoneOnTopic(topicMilestonesLatest, cachedMs.Milestone())
}

func publishMilestoneOnTopic(topic string, milestone *storage.Milestone) {
	if deps.MQTTBroker.HasSubscribers(topic) {
		publishOnTopic(topic, &milestonePayload{
			Index: uint32(milestone.Index),
			Time:  milestone.Timestamp.Unix(),
		})
	}
}

func publishReceipt(r *iotago.Receipt) {
	if deps.MQTTBroker.HasSubscribers(topicReceipts) {
		publishOnTopic(topicReceipts, r)
	}
}

func publishMessage(cachedMessage *storage.CachedMessage) {
	defer cachedMessage.Release(true)

	if deps.MQTTBroker.HasSubscribers(topicMessages) {
		deps.MQTTBroker.Send(topicMessages, cachedMessage.Message().Data())
	}
}

func publishTransactionIncludedMessage(transactionID *iotago.TransactionID, messageID hornet.MessageID) {
	transactionTopic := strings.ReplaceAll(topicTransactionsIncludedMessage, "{transactionId}", hex.EncodeToString(transactionID[:]))
	if deps.MQTTBroker.HasSubscribers(transactionTopic) {
		cachedMessage := deps.Storage.CachedMessageOrNil(messageID)
		if cachedMessage != nil {
			deps.MQTTBroker.Send(transactionTopic, cachedMessage.Message().Data())
			cachedMessage.Release(true)
		}
	}
}

func publishMessageMetadata(cachedMetadata *storage.CachedMetadata) {
	defer cachedMetadata.Release(true)

	metadata := cachedMetadata.Metadata()

	messageID := metadata.MessageID().ToHex()
	singleMessageTopic := strings.ReplaceAll(topicMessagesMetadata, "{messageId}", messageID)
	hasSingleMessageTopicSubscriber := deps.MQTTBroker.HasSubscribers(singleMessageTopic)

	hasAllMessagesTopicSubscriber := deps.MQTTBroker.HasSubscribers(topicMessagesReferenced)

	if hasSingleMessageTopicSubscriber || hasAllMessagesTopicSubscriber {

		var referencedByMilestone *milestone.Index = nil
		referenced, referencedIndex := metadata.ReferencedWithIndex()
		if referenced {
			referencedByMilestone = &referencedIndex
		}

		if !hasSingleMessageTopicSubscriber && (hasAllMessagesTopicSubscriber && !referenced) {
			// the topicMessagesReferenced only cares about referenced messages,
			// so skip this if no one is subscribed to this particular message
			return
		}

		messageMetadataResponse := &messageMetadataPayload{
			MessageID:                  metadata.MessageID().ToHex(),
			Parents:                    metadata.Parents().ToHex(),
			Solid:                      metadata.IsSolid(),
			ReferencedByMilestoneIndex: referencedByMilestone,
		}

		if metadata.IsMilestone() {
			messageMetadataResponse.MilestoneIndex = referencedByMilestone
		}

		if referenced {
			inclusionState := "noTransaction"

			conflict := metadata.Conflict()

			if conflict != storage.ConflictNone {
				inclusionState = "conflicting"
				messageMetadataResponse.ConflictReason = &conflict
			} else if metadata.IsIncludedTxInLedger() {
				inclusionState = "included"
			}

			messageMetadataResponse.LedgerInclusionState = &inclusionState
		} else if metadata.IsSolid() {
			// determine info about the quality of the tip if not referenced
			cmi := deps.SyncManager.ConfirmedMilestoneIndex()
			ycri, ocri, err := dag.ConeRootIndexes(Plugin.Daemon().ContextStopped(), deps.Storage, cachedMetadata.Retain(), cmi)
			if err != nil {
				if !errors.Is(err, common.ErrOperationAborted) {
					Plugin.LogWarn(err)
				}
				// do not publish the message if calculation was aborted or failed
				return
			}

			// if none of the following checks is true, the tip is non-lazy, so there is no need to promote or reattach
			shouldPromote := false
			shouldReattach := false

			if (cmi - ocri) > milestone.Index(deps.BelowMaxDepth) {
				// if the OCRI to CMI delta is over BelowMaxDepth/below-max-depth, then the tip is lazy and should be reattached
				shouldPromote = false
				shouldReattach = true
			} else if (cmi - ycri) > milestone.Index(deps.MaxDeltaMsgYoungestConeRootIndexToCMI) {
				// if the CMI to YCRI delta is over CfgTipSelMaxDeltaMsgYoungestConeRootIndexToCMI, then the tip is lazy and should be promoted
				shouldPromote = true
				shouldReattach = false
			} else if (cmi - ocri) > milestone.Index(deps.MaxDeltaMsgOldestConeRootIndexToCMI) {
				// if the OCRI to CMI delta is over CfgTipSelMaxDeltaMsgOldestConeRootIndexToCMI, the tip is semi-lazy and should be promoted
				shouldPromote = true
				shouldReattach = false
			}

			messageMetadataResponse.ShouldPromote = &shouldPromote
			messageMetadataResponse.ShouldReattach = &shouldReattach
		}

		// Serialize here instead of using publishOnTopic to avoid double JSON marshaling
		jsonPayload, err := json.Marshal(messageMetadataResponse)
		if err != nil {
			Plugin.LogWarn(err)
			return
		}

		if hasSingleMessageTopicSubscriber {
			deps.MQTTBroker.Send(singleMessageTopic, jsonPayload)
		}
		if referenced && hasAllMessagesTopicSubscriber {
			deps.MQTTBroker.Send(topicMessagesReferenced, jsonPayload)
		}
	}
}

func payloadForOutput(ledgerIndex milestone.Index, output *utxo.Output) *outputPayload {
	rawOutputJSON, err := output.Output().MarshalJSON()
	if err != nil {
		return nil
	}

	rawRawOutputJSON := json.RawMessage(rawOutputJSON)
	transactionID := output.OutputID().TransactionID()

	return &outputPayload{
		MessageID:                output.MessageID().ToHex(),
		TransactionID:            hex.EncodeToString(transactionID[:]),
		Spent:                    false,
		OutputIndex:              output.OutputID().Index(),
		RawOutput:                &rawRawOutputJSON,
		MilestoneIndexBooked:     output.MilestoneIndex(),
		MilestoneTimestampBooked: output.MilestoneTimestamp(),
		LedgerIndex:              ledgerIndex,
	}
}

func payloadForSpent(ledgerIndex milestone.Index, spent *utxo.Spent) *outputPayload {
	payload := payloadForOutput(ledgerIndex, spent.Output())
	if payload != nil {
		payload.Spent = true
		payload.MilestoneIndexSpent = spent.MilestoneIndex()
		payload.TransactionIDSpent = hex.EncodeToString(spent.TargetTransactionID()[:])
		payload.MilestoneTimestampSpent = spent.MilestoneTimestamp()
	}
	return payload
}

func publishOutput(ledgerIndex milestone.Index, output *utxo.Output) {
	outputsTopic := strings.ReplaceAll(topicOutputs, "{outputId}", output.OutputID().ToHex())
	outputsTopicHasSubscribers := deps.MQTTBroker.HasSubscribers(outputsTopic)

	if outputsTopicHasSubscribers {
		if payload := payloadForOutput(ledgerIndex, output); payload != nil {
			publishOnTopic(outputsTopic, payload)
		}
	}

	// If this is the first output in a transaction (index 0), then check if someone is observing the transaction that generated this output
	if output.OutputID().Index() == 0 {
		transactionID := output.OutputID().TransactionID()
		publishTransactionIncludedMessage(&transactionID, output.MessageID())
	}
}

func publishSpent(ledgerIndex milestone.Index, spent *utxo.Spent) {
	outputsTopic := strings.ReplaceAll(topicOutputs, "{outputId}", spent.OutputID().ToHex())
	outputsTopicHasSubscribers := deps.MQTTBroker.HasSubscribers(outputsTopic)

	if outputsTopicHasSubscribers {
		if payload := payloadForSpent(ledgerIndex, spent); payload != nil {
			publishOnTopic(outputsTopic, payload)
		}
	}
}

func messageIDFromTopic(topicName string) hornet.MessageID {
	if strings.HasPrefix(topicName, "messages/") && strings.HasSuffix(topicName, "/metadata") {
		messageIDHex := strings.Replace(topicName, "messages/", "", 1)
		messageIDHex = strings.Replace(messageIDHex, "/metadata", "", 1)

		messageID, err := hornet.MessageIDFromHex(messageIDHex)
		if err != nil {
			return nil
		}
		return messageID
	}
	return nil
}

func transactionIDFromTopic(topicName string) *iotago.TransactionID {
	if strings.HasPrefix(topicName, "transactions/") && strings.HasSuffix(topicName, "/included-message") {
		transactionIDHex := strings.Replace(topicName, "transactions/", "", 1)
		transactionIDHex = strings.Replace(transactionIDHex, "/included-message", "", 1)

		decoded, err := hex.DecodeString(transactionIDHex)
		if err != nil || len(decoded) != iotago.TransactionIDLength {
			return nil
		}
		transactionID := &iotago.TransactionID{}
		copy(transactionID[:], decoded)
		return transactionID
	}
	return nil
}

func outputIDFromTopic(topicName string) *iotago.OutputID {
	if strings.HasPrefix(topicName, "outputs/") {
		outputIDHex := strings.Replace(topicName, "outputs/", "", 1)
		outputID, err := iotago.OutputIDFromHex(outputIDHex)
		if err != nil {
			return nil
		}
		return &outputID
	}
	return nil
}
