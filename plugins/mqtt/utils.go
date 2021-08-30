package mqtt

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	iotago "github.com/iotaledger/iota.go/v2"
)

func publishOnTopic(topic string, payload interface{}) {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		Plugin.LogWarn(err)
		return
	}

	mqttBroker.Send(topic, jsonPayload)
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
	if mqttBroker.HasSubscribers(topic) {
		publishOnTopic(topic, &milestonePayload{
			Index: uint32(milestone.Index),
			Time:  milestone.Timestamp.Unix(),
		})
	}
}

func publishReceipt(r *iotago.Receipt) {
	if mqttBroker.HasSubscribers(topicReceipts) {
		publishOnTopic(topicReceipts, r)
	}
}

func publishMessage(cachedMessage *storage.CachedMessage) {
	defer cachedMessage.Release(true)

	if mqttBroker.HasSubscribers(topicMessages) {
		mqttBroker.Send(topicMessages, cachedMessage.Message().Data())
	}

	indexation := cachedMessage.Message().Indexation()
	if indexation != nil {
		indexationTopic := strings.ReplaceAll(topicMessagesIndexation, "{index}", hex.EncodeToString(indexation.Index))
		if mqttBroker.HasSubscribers(indexationTopic) {
			mqttBroker.Send(indexationTopic, cachedMessage.Message().Data())
		}
	}
}

func publishTransactionIncludedMessage(transactionID *iotago.TransactionID, messageID hornet.MessageID) {
	transactionTopic := strings.ReplaceAll(topicTransactionsIncludedMessage, "{transactionId}", hex.EncodeToString(transactionID[:]))
	if mqttBroker.HasSubscribers(transactionTopic) {
		cachedMessage := deps.Storage.CachedMessageOrNil(messageID)
		if cachedMessage != nil {
			mqttBroker.Send(transactionTopic, cachedMessage.Message().Data())
			cachedMessage.Release(true)
		}
	}
}

func publishMessageMetadata(cachedMetadata *storage.CachedMetadata) {
	defer cachedMetadata.Release(true)

	metadata := cachedMetadata.Metadata()

	messageID := metadata.MessageID().ToHex()
	singleMessageTopic := strings.ReplaceAll(topicMessagesMetadata, "{messageId}", messageID)
	hasSingleMessageTopicSubscriber := mqttBroker.HasSubscribers(singleMessageTopic)

	hasAllMessagesTopicSubscriber := mqttBroker.HasSubscribers(topicMessagesReferenced)

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
			cmi := deps.Storage.ConfirmedMilestoneIndex()
			ycri, ocri := dag.ConeRootIndexes(deps.Storage, cachedMetadata.Retain(), cmi)

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
			mqttBroker.Send(singleMessageTopic, jsonPayload)
		}
		if hasAllMessagesTopicSubscriber {
			mqttBroker.Send(topicMessagesReferenced, jsonPayload)
		}
	}
}

func payloadForOutput(ledgerIndex milestone.Index, output *utxo.Output, spent bool) *outputPayload {

	var rawOutput iotago.Output
	switch output.OutputType() {
	case iotago.OutputSigLockedSingleOutput:
		rawOutput = &iotago.SigLockedSingleOutput{
			Address: output.Address(),
			Amount:  output.Amount(),
		}
	case iotago.OutputSigLockedDustAllowanceOutput:
		rawOutput = &iotago.SigLockedDustAllowanceOutput{
			Address: output.Address(),
			Amount:  output.Amount(),
		}
	default:
		return nil
	}

	rawOutputJSON, err := rawOutput.MarshalJSON()
	if err != nil {
		return nil
	}

	rawRawOutputJSON := json.RawMessage(rawOutputJSON)

	return &outputPayload{
		MessageID:     output.MessageID().ToHex(),
		TransactionID: hex.EncodeToString(output.OutputID()[:iotago.TransactionIDLength]),
		Spent:         spent,
		OutputIndex:   binary.LittleEndian.Uint16(output.OutputID()[iotago.TransactionIDLength : iotago.TransactionIDLength+iotago.UInt16ByteSize]),
		LedgerIndex:   ledgerIndex,
		RawOutput:     &rawRawOutputJSON,
	}
}

func publishOutput(ledgerIndex milestone.Index, output *utxo.Output, spent bool) {

	outputsTopic := strings.ReplaceAll(topicOutputs, "{outputId}", output.OutputID().ToHex())
	outputsTopicHasSubscribers := mqttBroker.HasSubscribers(outputsTopic)

	addressBech32Topic := strings.ReplaceAll(topicAddressesOutput, "{address}", output.Address().Bech32(deps.Bech32HRP))
	addressBech32TopicHasSubscribers := mqttBroker.HasSubscribers(addressBech32Topic)

	addressEd25519Topic := strings.ReplaceAll(topicAddressesEd25519Output, "{address}", output.Address().String())
	addressEd25519TopicHasSubscribers := mqttBroker.HasSubscribers(addressEd25519Topic)

	if outputsTopicHasSubscribers || addressEd25519TopicHasSubscribers || addressBech32TopicHasSubscribers {
		if payload := payloadForOutput(ledgerIndex, output, spent); payload != nil {

			// Serialize here instead of using publishOnTopic to avoid double JSON marshaling
			jsonPayload, err := json.Marshal(payload)
			if err != nil {
				Plugin.LogWarn(err)
				return
			}

			if outputsTopicHasSubscribers {
				mqttBroker.Send(outputsTopic, jsonPayload)
			}

			if addressBech32TopicHasSubscribers {
				mqttBroker.Send(addressBech32Topic, jsonPayload)
			}

			if addressEd25519TopicHasSubscribers {
				mqttBroker.Send(addressEd25519Topic, jsonPayload)
			}
		}
	}

	if !spent {
		// If this is the first output in a transaction (index 0), then check if someone is observing the transaction that generated this output
		if binary.LittleEndian.Uint16(output.OutputID()[iotago.TransactionIDLength:]) == 0 {
			transactionID := &iotago.TransactionID{}
			copy(transactionID[:], output.OutputID()[:iotago.TransactionIDLength])
			publishTransactionIncludedMessage(transactionID, output.MessageID())
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

func outputIDFromTopic(topicName string) *iotago.UTXOInputID {
	if strings.HasPrefix(topicName, "outputs/") {
		outputIDHex := strings.Replace(topicName, "outputs/", "", 1)

		bytes, err := hex.DecodeString(outputIDHex)
		if err != nil {
			return nil
		}

		if len(bytes) == iotago.TransactionIDLength+iotago.UInt16ByteSize {
			outputID := &iotago.UTXOInputID{}
			copy(outputID[:], bytes)
			return outputID
		}
	}
	return nil
}
