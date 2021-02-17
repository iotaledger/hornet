package mqtt

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"strings"

	iotago "github.com/iotaledger/iota.go/v2"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/plugins/urts"
)

func publishOnTopic(topic string, payload interface{}) {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Warn(err.Error())
		return
	}

	mqttBroker.Send(topic, jsonPayload)
}

func publishSolidMilestone(cachedMs *storage.CachedMilestone) {
	defer cachedMs.Release(true)
	publishMilestoneOnTopic(topicMilestonesSolid, cachedMs.GetMilestone())
}

func publishLatestMilestone(cachedMs *storage.CachedMilestone) {
	defer cachedMs.Release(true)
	publishMilestoneOnTopic(topicMilestonesLatest, cachedMs.GetMilestone())
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
		mqttBroker.Send(topicMessages, cachedMessage.GetMessage().GetData())
	}

	indexation := cachedMessage.GetMessage().GetIndexation()
	if indexation != nil {
		indexationTopic := strings.ReplaceAll(topicMessagesIndexation, "{index}", indexation.Index)
		if mqttBroker.HasSubscribers(indexationTopic) {
			mqttBroker.Send(indexationTopic, cachedMessage.GetMessage().GetData())
		}
	}

}

func publishMessageMetadata(cachedMetadata *storage.CachedMetadata) {
	defer cachedMetadata.Release(true)

	metadata := cachedMetadata.GetMetadata()

	messageID := metadata.GetMessageID().ToHex()
	singleMessageTopic := strings.ReplaceAll(topicMessagesMetadata, "{messageId}", messageID)
	hasSingleMessageTopicSubscriber := mqttBroker.HasSubscribers(singleMessageTopic)

	hasAllMessagesTopicSubscriber := mqttBroker.HasSubscribers(topicMessagesReferenced)

	if hasSingleMessageTopicSubscriber || hasAllMessagesTopicSubscriber {

		var referencedByMilestone *milestone.Index = nil
		referenced, referencedIndex := metadata.GetReferenced()
		if referenced {
			referencedByMilestone = &referencedIndex
		}

		if !hasSingleMessageTopicSubscriber && (hasAllMessagesTopicSubscriber && !referenced) {
			// the topicMessagesReferenced only cares about referenced messages,
			// so skip this if no one is subscribed to this particular message
			return
		}

		messageMetadataResponse := &messageMetadataPayload{
			MessageID:                  metadata.GetMessageID().ToHex(),
			Parents:                    metadata.GetParents().ToHex(),
			Solid:                      metadata.IsSolid(),
			ReferencedByMilestoneIndex: referencedByMilestone,
		}

		if metadata.IsMilestone() {
			messageMetadataResponse.MilestoneIndex = referencedByMilestone
		}

		if referenced {
			inclusionState := "noTransaction"

			conflict := metadata.GetConflict()

			if conflict != storage.ConflictNone {
				inclusionState = "conflicting"
				messageMetadataResponse.ConflictReason = &conflict
			} else if metadata.IsIncludedTxInLedger() {
				inclusionState = "included"
			}

			messageMetadataResponse.LedgerInclusionState = &inclusionState
		} else if metadata.IsSolid() {
			// determine info about the quality of the tip if not referenced
			lsmi := deps.Storage.GetSolidMilestoneIndex()
			ycri, ocri := dag.GetConeRootIndexes(deps.Storage, cachedMetadata.Retain(), lsmi)

			// if none of the following checks is true, the tip is non-lazy, so there is no need to promote or reattach
			shouldPromote := false
			shouldReattach := false

			if (lsmi - ocri) > milestone.Index(deps.NodeConfig.Int(urts.CfgTipSelBelowMaxDepth)) {
				// if the OCRI to LSMI delta is over BelowMaxDepth/below-max-depth, then the tip is lazy and should be reattached
				shouldPromote = false
				shouldReattach = true
			} else if (lsmi - ycri) > milestone.Index(deps.NodeConfig.Int(urts.CfgTipSelMaxDeltaMsgYoungestConeRootIndexToLSMI)) {
				// if the LSMI to YCRI delta is over CfgTipSelMaxDeltaMsgYoungestConeRootIndexToLSMI, then the tip is lazy and should be promoted
				shouldPromote = true
				shouldReattach = false
			} else if (lsmi - ocri) > milestone.Index(deps.NodeConfig.Int(urts.CfgTipSelMaxDeltaMsgOldestConeRootIndexToLSMI)) {
				// if the OCRI to LSMI delta is over CfgTipSelMaxDeltaMsgOldestConeRootIndexToLSMI, the tip is semi-lazy and should be promoted
				shouldPromote = true
				shouldReattach = false
			}

			messageMetadataResponse.ShouldPromote = &shouldPromote
			messageMetadataResponse.ShouldReattach = &shouldReattach
		}

		// Serialize here instead of using publishOnTopic to avoid double JSON marshalling
		jsonPayload, err := json.Marshal(messageMetadataResponse)
		if err != nil {
			log.Warn(err.Error())
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

func payloadForOutput(output *utxo.Output, spent bool) *outputPayload {

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
		RawOutput:     &rawRawOutputJSON,
	}
}

func publishOutput(output *utxo.Output, spent bool) {

	outputsTopic := strings.ReplaceAll(topicOutputs, "{outputId}", output.OutputID().ToHex())
	outputsTopicHasSubscribers := mqttBroker.HasSubscribers(outputsTopic)

	// Since we do not know on which network we are running (Mainnet vs Testnet), we have to check if anyone is subscribed to the bech32 address on both Mainnet and Testnet
	addressBech32Topic := strings.ReplaceAll(topicAddressesOutput, "{address}", output.Address().Bech32(deps.Bech32HRP))
	addressBech32TopicHasSubscribers := mqttBroker.HasSubscribers(addressBech32Topic)

	addressEd25519Topic := strings.ReplaceAll(topicAddressesEd25519Output, "{address}", output.Address().String())
	addressEd25519TopicHasSubscribers := mqttBroker.HasSubscribers(addressEd25519Topic)

	if outputsTopicHasSubscribers || addressEd25519TopicHasSubscribers || addressBech32TopicHasSubscribers {
		if payload := payloadForOutput(output, spent); payload != nil {

			// Serialize here instead of using publishOnTopic to avoid double JSON marshalling
			jsonPayload, err := json.Marshal(payload)
			if err != nil {
				log.Warn(err.Error())
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
}

func messageIdFromTopic(topicName string) hornet.MessageID {
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

func outputIdFromTopic(topicName string) *iotago.UTXOInputID {
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
