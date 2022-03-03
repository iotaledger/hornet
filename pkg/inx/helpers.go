package inx

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

func NewMessageId(messageID hornet.MessageID) *MessageId {
	id := &MessageId{
		Id: make([]byte, len(messageID)),
	}
	copy(id.Id, messageID[:])
	return id
}

func NewMessageWithBytes(messageID hornet.MessageID, data []byte) *Message {
	m := &Message{
		MessageId: NewMessageId(messageID),
		Message: &RawMessage{
			Data: make([]byte, len(data)),
		},
	}
	copy(m.Message.Data, data)
	return m
}

func NewMessageMetadata(messageID hornet.MessageID, metadata *storage.MessageMetadata) *MessageMetadata {
	m := &MessageMetadata{
		MessageId: NewMessageId(messageID),
		Parents:   metadata.Parents().ToSliceOfSlices(),
		Solid:     metadata.IsSolid(),
	}

	referenced, msIndex := metadata.ReferencedWithIndex()
	if referenced {
		m.ReferencedByMilestoneIndex = uint32(msIndex)
		inclusionState := MessageMetadata_NO_TRANSACTION
		conflict := metadata.Conflict()
		if conflict != storage.ConflictNone {
			inclusionState = MessageMetadata_CONFLICTING
			m.ConflictReason = MessageMetadata_ConflictReason(conflict)
		} else if metadata.IsIncludedTxInLedger() {
			inclusionState = MessageMetadata_INCLUDED
		}
		m.LedgerInclusionState = inclusionState

		if metadata.IsMilestone() {
			m.MilestoneIndex = uint32(msIndex)
		}
	}

	return m
}

func NewOutputId(outputID *iotago.OutputID) *OutputId {
	id := &OutputId{
		Id: make([]byte, len(outputID)),
	}
	copy(id.Id, outputID[:])
	return id
}

func NewLedgerOutput(o *utxo.Output) (*LedgerOutput, error) {
	outputBytes, err := o.Output().Serialize(serializer.DeSeriModeNoValidation, iotago.ZeroRentParas)
	if err != nil {
		return nil, err
	}
	l := &LedgerOutput{
		OutputId:           NewOutputId(o.OutputID()),
		MessageId:          NewMessageId(o.MessageID()),
		MilestoneIndex:     uint32(o.MilestoneIndex()),
		MilestoneTimestamp: o.MilestoneTimestamp(),
		Output:             make([]byte, len(outputBytes)),
	}
	copy(l.Output, outputBytes)
	return l, nil
}

func NewLedgerSpent(s *utxo.Spent) (*LedgerSpent, error) {
	output, err := NewLedgerOutput(s.Output())
	if err != nil {
		return nil, err
	}
	transactionID := s.TargetTransactionID()
	l := &LedgerSpent{
		Output:                  output,
		TargetTransactionId:     make([]byte, len(transactionID)),
		SpentMilestoneIndex:     uint32(s.MilestoneIndex()),
		SpentMilestoneTimestamp: s.MilestoneTimestamp(),
	}
	copy(l.TargetTransactionId, transactionID[:])
	return l, nil
}

func NewLedgerUpdate(index milestone.Index, newOutputs utxo.Outputs, newSpents utxo.Spents) (*LedgerUpdate, error) {
	u := &LedgerUpdate{
		MilestoneIndex: uint32(index),
		Created:        make([]*LedgerOutput, len(newOutputs)),
		Consumed:       make([]*LedgerSpent, len(newSpents)),
	}
	for i, o := range newOutputs {
		output, err := NewLedgerOutput(o)
		if err != nil {
			return nil, err
		}
		u.Created[i] = output
	}
	for i, s := range newSpents {
		spent, err := NewLedgerSpent(s)
		if err != nil {
			return nil, err
		}
		u.Consumed[i] = spent
	}
	return u, nil
}

func NewMilestone(milestone *storage.Milestone) *Milestone {
	return &Milestone{
		MilestoneIndex:     uint32(milestone.Index),
		MilestoneTimestamp: uint32(milestone.Timestamp.Unix()),
		MessageId:          NewMessageId(milestone.MessageID),
	}
}
