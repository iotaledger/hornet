package inx

import (
	"github.com/gohornet/hornet/pkg/inx"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

func INXMessageWithBytes(messageID hornet.MessageID, data []byte) *inx.Message {
	s := &inx.Message{
		MessageId: make([]byte, len(messageID)),
		Message: &inx.Raw{
			Data: make([]byte, len(data)),
		},
	}
	copy(s.MessageId, messageID[:])
	copy(s.Message.Data, data)
	return s
}

func INXOutputWithOutput(o *utxo.Output) (*inx.LedgerOutput, error) {
	outputID := o.OutputID()
	messageID := o.MessageID()
	outputBytes, err := o.Output().Serialize(serializer.DeSeriModeNoValidation, iotago.ZeroRentParas)
	if err != nil {
		return nil, err
	}
	l := &inx.LedgerOutput{
		OutputId:           make([]byte, len(outputID)),
		MessageId:          make([]byte, len(messageID)),
		MilestoneIndex:     uint32(o.MilestoneIndex()),
		MilestoneTimestamp: o.MilestoneTimestamp(),
		Output: &inx.Raw{
			Data: make([]byte, len(outputBytes)),
		},
	}
	copy(l.OutputId, outputID[:])
	copy(l.MessageId, messageID[:])
	copy(l.Output.Data, outputBytes)
	return l, nil
}

func INXSpentWithSpent(s *utxo.Spent) (*inx.LedgerSpent, error) {

	output, err := INXOutputWithOutput(s.Output())
	if err != nil {
		return nil, err
	}
	transactionID := s.TargetTransactionID()
	l := &inx.LedgerSpent{
		Output:                  output,
		TargetTransactionId:     make([]byte, len(transactionID)),
		SpentMilestoneIndex:     uint32(s.MilestoneIndex()),
		SpentMilestoneTimestamp: s.MilestoneTimestamp(),
	}
	copy(l.TargetTransactionId, transactionID[:])
	return l, nil
}

func INXLedgerUpdated(index milestone.Index, newOutputs utxo.Outputs, newSpents utxo.Spents) (*inx.LedgerUpdate, error) {

	u := &inx.LedgerUpdate{
		MilestoneIndex: uint32(index),
		Created:        make([]*inx.LedgerOutput, len(newOutputs)),
		Consumed:       make([]*inx.LedgerSpent, len(newSpents)),
	}

	for i, o := range newOutputs {
		output, err := INXOutputWithOutput(o)
		if err != nil {
			return nil, err
		}
		u.Created[i] = output
	}

	for i, s := range newSpents {
		spent, err := INXSpentWithSpent(s)
		if err != nil {
			return nil, err
		}
		u.Consumed[i] = spent
	}

	return u, nil
}
