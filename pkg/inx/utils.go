package inx

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/pkg/errors"
)

// Request & Response

func SubmitMessageRequestWithMessage(msg *iotago.Message) (*SubmitMessageRequest, error) {
	wrapped, err := WrapMessage(msg)
	if err != nil {
		return nil, err
	}
	return &SubmitMessageRequest{
		Message: wrapped,
	}, nil
}

func (x *SubmitMessageResponse) UnwrapMessageID() hornet.MessageID {
	return hornet.MessageIDFromSlice(x.GetMessageId())
}

// Message

func (x *Message) UnwrapMessageID() hornet.MessageID {
	return hornet.MessageIDFromSlice(x.GetMessageId())
}

func (x *Message) UnwrapMessage(deSeriMode serializer.DeSerializationMode) (*iotago.Message, error) {
	return x.GetMessage().UnwrapMessage(deSeriMode)
}

func (x *Message) MustUnwrapMessage(deSeriMode serializer.DeSerializationMode) *iotago.Message {
	msg, err := x.GetMessage().UnwrapMessage(deSeriMode)
	if err != nil {
		panic(err)
	}
	return msg
}

// Ledger

func (x *LedgerOutput) UnwrapOutputID() *iotago.OutputID {
	id := &iotago.OutputID{}
	copy(id[:], x.GetOutputId())
	return id
}

func (x *LedgerOutput) UnwrapMessageID() hornet.MessageID {
	return hornet.MessageIDFromSlice(x.GetMessageId())
}

func (x *LedgerOutput) UnwrapOutput(deSeriMode serializer.DeSerializationMode) (iotago.Output, error) {
	return x.Output.UnwrapOutput(deSeriMode)
}

func (x *LedgerOutput) MustUnwrapOutput(deSeriMode serializer.DeSerializationMode) iotago.Output {
	output, err := x.Output.UnwrapOutput(deSeriMode)
	if err != nil {
		panic(err)
	}
	return output
}

func (x *LedgerSpent) UnwrapTargetTransactionID() *iotago.TransactionID {
	id := &iotago.TransactionID{}
	copy(id[:], x.GetTargetTransactionId())
	return id
}

// Raw bytes

func WrapMessage(msg *iotago.Message) (*Raw, error) {
	bytes, err := msg.Serialize(serializer.DeSeriModeNoValidation, iotago.ZeroRentParas)
	if err != nil {
		return nil, err
	}
	return &Raw{
		Data: bytes,
	}, nil
}

func (x *Raw) UnwrapMessage(deSeriMode serializer.DeSerializationMode) (*iotago.Message, error) {
	msg := &iotago.Message{}
	if _, err := msg.Deserialize(x.GetData(), deSeriMode, iotago.ZeroRentParas); err != nil {
		return nil, err
	}
	return msg, nil
}

func (x *Raw) UnwrapOutput(deSeriMode serializer.DeSerializationMode) (iotago.Output, error) {
	data := x.GetData()
	if len(data) == 0 {
		return nil, errors.New("invalid output length")
	}

	output, err := iotago.OutputSelector(uint32(data[0]))
	if err != nil {
		return nil, err
	}

	_, err = output.Deserialize(data, deSeriMode, iotago.ZeroRentParas)
	if err != nil {
		return nil, err
	}
	return output, nil
}
