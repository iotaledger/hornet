package inx

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

func WrapMessage(msg *iotago.Message) (*WrappedMessage, error) {
	bytes, err := msg.Serialize(serializer.DeSeriModeNoValidation, iotago.ZeroRentParas)
	if err != nil {
		return nil, err
	}
	return &WrappedMessage{
		Data: bytes,
	}, nil
}

func (w *WrappedMessage) UnwrapMessage() (*iotago.Message, error) {
	msg := &iotago.Message{}
	if _, err := msg.Deserialize(w.GetData(), serializer.DeSeriModeNoValidation, iotago.ZeroRentParas); err != nil {
		return nil, err
	}
	return msg, nil
}

func StreamMessageWithBytes(messageID hornet.MessageID, data []byte) *StreamedMessage {
	s := &StreamedMessage{
		MessageId: make([]byte, len(messageID)),
		Message: &WrappedMessage{
			Data: make([]byte, len(data)),
		},
	}
	copy(s.MessageId, messageID[:])
	copy(s.Message.Data, data)
	return s
}

func (s *StreamedMessage) MessageID() hornet.MessageID {
	return hornet.MessageIDFromSlice(s.GetMessageId())
}

func (s *StreamedMessage) UnwrapMessage() (*iotago.Message, error) {
	return s.GetMessage().UnwrapMessage()
}

func (s *StreamedMessage) MustUnwrapMessage() *iotago.Message {
	msg, err := s.GetMessage().UnwrapMessage()
	if err != nil {
		panic(err)
	}
	return msg
}

func SubmitMessageRequestWithMessage(msg *iotago.Message) (*SubmitMessageRequest, error) {
	wrapped, err := WrapMessage(msg)
	if err != nil {
		return nil, err
	}
	return &SubmitMessageRequest{
		Message: wrapped,
	}, nil
}

func (r *SubmitMessageResponse) MessageID() hornet.MessageID {
	return hornet.MessageIDFromSlice(r.GetMessageId())
}
