package main

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gohornet/hornet/pkg/inx"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

func (s *Server) PublishRawOnTopicIfSubscribed(topic string, payload []byte) {
	if s.MQTTBroker.HasSubscribers(topic) {
		s.MQTTBroker.Send(topic, payload)
	}
}

func (s *Server) PublishOnTopicIfSubscribed(topic string, payload interface{}) {
	if s.MQTTBroker.HasSubscribers(topic) {
		s.PublishOnTopic(topic, payload)
	}
}

func (s *Server) PublishOnTopic(topic string, payload interface{}) {
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return
	}

	s.MQTTBroker.Send(topic, jsonPayload)
}

func (s *Server) PublishMilestoneOnTopic(topic string, milestone *inx.Milestone) {
	s.PublishOnTopicIfSubscribed(topic, &milestonePayload{
		Index: milestone.GetMilestoneIndex(),
		Time:  milestone.GetMilestoneTimestamp(),
	})
}

func (s *Server) PublishReceipt(r *inx.RawReceipt) {
	receipt, err := r.UnwrapReceipt(serializer.DeSeriModeNoValidation)
	if err != nil {
		return
	}
	s.PublishOnTopicIfSubscribed(topicReceipts, receipt)
}

func (s *Server) PublishMessage(msg *inx.RawMessage) {

	message, err := msg.UnwrapMessage(serializer.DeSeriModeNoValidation)
	if err != nil {
		return
	}

	s.PublishRawOnTopicIfSubscribed(topicMessages, msg.GetData())

	switch payload := message.Payload.(type) {
	case *iotago.Transaction:
		s.PublishRawOnTopicIfSubscribed(topicMessagesTransaction, msg.GetData())

		switch p := payload.Essence.Payload.(type) {
		case *iotago.TaggedData:
			s.PublishRawOnTopicIfSubscribed(topicMessagesTransactionTaggedData, msg.GetData())
			if len(p.Tag) > 0 {
				txTaggedDataTagTopic := strings.ReplaceAll(topicMessagesTransactionTaggedDataTag, "{tag}", iotago.EncodeHex(p.Tag))
				s.PublishRawOnTopicIfSubscribed(txTaggedDataTagTopic, msg.GetData())
			}
		}

	case *iotago.TaggedData:
		s.PublishRawOnTopicIfSubscribed(topicMessagesTaggedData, msg.GetData())
		if len(payload.Tag) > 0 {
			taggedDataTagTopic := strings.ReplaceAll(topicMessagesTaggedDataTag, "{tag}", iotago.EncodeHex(payload.Tag))
			s.PublishRawOnTopicIfSubscribed(taggedDataTagTopic, msg.GetData())
		}

	case *iotago.Milestone:
		s.PublishRawOnTopicIfSubscribed(topicMessagesMilestone, msg.GetData())
	}
}

func (s *Server) hasSubscriberForTransactionIncludedMessage(transactionID *iotago.TransactionID) bool {
	transactionTopic := strings.ReplaceAll(topicTransactionsIncludedMessage, "{transactionId}", transactionID.ToHex())
	return s.MQTTBroker.HasSubscribers(transactionTopic)
}

func (s *Server) PublishTransactionIncludedMessage(transactionID *iotago.TransactionID, message *inx.RawMessage) {
	transactionTopic := strings.ReplaceAll(topicTransactionsIncludedMessage, "{transactionId}", transactionID.ToHex())
	s.PublishRawOnTopicIfSubscribed(transactionTopic, message.GetData())
}

func (s *Server) PublishMessageMetadata(metadata *inx.MessageMetadata) {

	messageID := metadata.UnwrapMessageID().ToHex()
	singleMessageTopic := strings.ReplaceAll(topicMessagesMetadata, "{messageId}", messageID)
	hasSingleMessageTopicSubscriber := s.MQTTBroker.HasSubscribers(singleMessageTopic)
	hasAllMessagesTopicSubscriber := s.MQTTBroker.HasSubscribers(topicMessagesReferenced)

	if !hasSingleMessageTopicSubscriber && !hasAllMessagesTopicSubscriber {
		return
	}

	response := &messageMetadataPayload{
		MessageID: messageID,
		Parents:   hornet.MessageIDsFromSliceOfSlices(metadata.GetParents()).ToHex(),
		Solid:     metadata.GetSolid(),
	}

	referencedByIndex := milestone.Index(metadata.GetReferencedByMilestoneIndex())

	referenced := referencedByIndex > 0

	if referenced {
		response.ReferencedByMilestoneIndex = &referencedByIndex

		var inclusionState string
		switch metadata.GetLedgerInclusionState() {
		case inx.MessageMetadata_NO_TRANSACTION:
			inclusionState = "noTransaction"
		case inx.MessageMetadata_CONFLICTING:
			inclusionState = "conflicting"
			conflict := metadata.GetConflictReason()
			response.ConflictReason = &conflict
		case inx.MessageMetadata_INCLUDED:
			inclusionState = "included"
		}
		response.LedgerInclusionState = &inclusionState

		milestoneIndex := milestone.Index(metadata.GetMilestoneIndex())
		if milestoneIndex > 0 {
			response.MilestoneIndex = &milestoneIndex
		}
	} else if metadata.GetSolid() {
		shouldPromote := metadata.GetShouldPromote()
		shouldReattach := metadata.GetShouldReattach()
		response.ShouldPromote = &shouldPromote
		response.ShouldReattach = &shouldReattach
	}

	// Serialize here instead of using publishOnTopic to avoid double JSON marshaling
	jsonPayload, err := json.Marshal(response)
	if err != nil {
		return
	}

	if hasSingleMessageTopicSubscriber {
		s.MQTTBroker.Send(singleMessageTopic, jsonPayload)
	}
	if referenced && hasAllMessagesTopicSubscriber {
		s.MQTTBroker.Send(topicMessagesReferenced, jsonPayload)
	}
}

func payloadForOutput(ledgerIndex milestone.Index, output *inx.LedgerOutput, iotaOutput iotago.Output) *outputPayload {
	rawOutputJSON, err := iotaOutput.MarshalJSON()
	if err != nil {
		return nil
	}

	outputID := output.GetOutputId().Unwrap()
	rawRawOutputJSON := json.RawMessage(rawOutputJSON)
	transactionID := outputID.TransactionID()

	return &outputPayload{
		MessageID:                output.GetMessageId().Unwrap().ToHex(),
		TransactionID:            transactionID.ToHex(),
		Spent:                    false,
		OutputIndex:              outputID.Index(),
		RawOutput:                &rawRawOutputJSON,
		MilestoneIndexBooked:     milestone.Index(output.GetMilestoneIndexBooked()),
		MilestoneTimestampBooked: output.GetMilestoneTimestampBooked(),
		LedgerIndex:              ledgerIndex,
	}
}

func payloadForSpent(ledgerIndex milestone.Index, spent *inx.LedgerSpent, iotaOutput iotago.Output) *outputPayload {
	payload := payloadForOutput(ledgerIndex, spent.GetOutput(), iotaOutput)
	if payload != nil {
		payload.Spent = true
		payload.MilestoneIndexSpent = milestone.Index(spent.GetMilestoneIndexSpent())
		payload.TransactionIDSpent = spent.UnwrapTransactionIDSpent().ToHex()
		payload.MilestoneTimestampSpent = spent.GetMilestoneTimestampSpent()
	}
	return payload
}

func (s *Server) PublishOnUnlockConditionTopics(baseTopic string, output iotago.Output, payloadFunc func() interface{}) {

	topicFunc := func(condition unlockCondition, addressString string) string {
		topic := strings.ReplaceAll(baseTopic, "{condition}", string(condition))
		return strings.ReplaceAll(topic, "{address}", addressString)
	}

	unlockConditions, err := output.UnlockConditions().Set()
	if err != nil {
		return
	}

	// this tracks the addresses used by any unlock condition
	// so that after checking all conditions we can see if anyone is subscribed to the wildcard
	addressesToPublishForAny := make(map[string]struct{})

	address := unlockConditions.Address()
	if address != nil {
		addr := address.Address.Bech32(s.ProtocolParams.NetworkPrefix())
		s.PublishOnTopicIfSubscribed(topicFunc(unlockConditionAddress, addr), payloadFunc())
		addressesToPublishForAny[addr] = struct{}{}
	}

	storageReturn := unlockConditions.StorageDepositReturn()
	if storageReturn != nil {
		addr := storageReturn.ReturnAddress.Bech32(s.ProtocolParams.NetworkPrefix())
		s.PublishOnTopicIfSubscribed(topicFunc(unlockConditionStorageReturn, addr), payloadFunc())
		addressesToPublishForAny[addr] = struct{}{}
	}

	expiration := unlockConditions.Expiration()
	if expiration != nil {
		addr := expiration.ReturnAddress.Bech32(s.ProtocolParams.NetworkPrefix())
		s.PublishOnTopicIfSubscribed(topicFunc(unlockConditionExpirationReturn, addr), payloadFunc())
		addressesToPublishForAny[addr] = struct{}{}
	}

	stateController := unlockConditions.StateControllerAddress()
	if stateController != nil {
		addr := stateController.Address.Bech32(s.ProtocolParams.NetworkPrefix())
		s.PublishOnTopicIfSubscribed(topicFunc(unlockConditionStateController, addr), payloadFunc())
		addressesToPublishForAny[addr] = struct{}{}
	}

	governor := unlockConditions.GovernorAddress()
	if governor != nil {
		addr := governor.Address.Bech32(s.ProtocolParams.NetworkPrefix())
		s.PublishOnTopicIfSubscribed(topicFunc(unlockConditionGovernor, addr), payloadFunc())
		addressesToPublishForAny[addr] = struct{}{}
	}

	immutableAlias := unlockConditions.ImmutableAlias()
	if immutableAlias != nil {
		addr := immutableAlias.Address.Bech32(s.ProtocolParams.NetworkPrefix())
		s.PublishOnTopicIfSubscribed(topicFunc(unlockConditionImmutableAlias, addr), payloadFunc())
		addressesToPublishForAny[addr] = struct{}{}
	}

	for addr := range addressesToPublishForAny {
		s.PublishOnTopicIfSubscribed(topicFunc(unlockConditionAny, addr), payloadFunc())
	}
}

func (s *Server) PublishOutput(ledgerIndex milestone.Index, output *inx.LedgerOutput) {

	iotaOutput, err := output.UnwrapOutput(serializer.DeSeriModeNoValidation)
	if err != nil {
		return
	}

	var payload *outputPayload
	payloadFunc := func() interface{} {
		if payload == nil {
			payload = payloadForOutput(ledgerIndex, output, iotaOutput)
		}
		return payload
	}

	outputID := output.GetOutputId().Unwrap()
	outputsTopic := strings.ReplaceAll(topicOutputs, "{outputId}", outputID.ToHex())
	s.PublishOnTopicIfSubscribed(outputsTopic, payloadFunc())

	// If this is the first output in a transaction (index 0), then check if someone is observing the transaction that generated this output
	if outputID.Index() == 0 {
		transactionID := outputID.TransactionID()
		if s.hasSubscriberForTransactionIncludedMessage(&transactionID) {
			s.fetchAndPublishTransactionInclusionWithMessage(context.Background(), &transactionID, output.GetMessageId().Unwrap())
		}
	}

	s.PublishOnUnlockConditionTopics(topicOutputsByUnlockConditionAndAddress, iotaOutput, payloadFunc)
}

func (s *Server) PublishSpent(ledgerIndex milestone.Index, spent *inx.LedgerSpent) {

	iotaOutput, err := spent.GetOutput().UnwrapOutput(serializer.DeSeriModeNoValidation)
	if err != nil {
		return
	}

	var payload *outputPayload
	payloadFunc := func() interface{} {
		if payload == nil {
			payload = payloadForSpent(ledgerIndex, spent, iotaOutput)
		}
		return payload
	}

	outputsTopic := strings.ReplaceAll(topicOutputs, "{outputId}", spent.GetOutput().GetOutputId().Unwrap().ToHex())
	s.PublishOnTopicIfSubscribed(outputsTopic, payloadFunc())

	s.PublishOnUnlockConditionTopics(topicSpentOutputsByUnlockConditionAndAddress, iotaOutput, payloadFunc)
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

		decoded, err := iotago.DecodeHex(transactionIDHex)
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
	if strings.HasPrefix(topicName, "outputs/") && !strings.HasPrefix(topicName, "outputs/unlock") {
		outputIDHex := strings.Replace(topicName, "outputs/", "", 1)
		outputID, err := iotago.OutputIDFromHex(outputIDHex)
		if err != nil {
			return nil
		}
		return &outputID
	}
	return nil
}
