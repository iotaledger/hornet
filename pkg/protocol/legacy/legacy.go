package legacy

import (
	"bytes"
	"encoding/binary"

	"github.com/gohornet/hornet/pkg/consts"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/protocol/message"
	"github.com/gohornet/hornet/pkg/protocol/tlv"
)

// FeatureSet denotes the version bit for legacy messages support.
const FeatureSet = 1 << 0

// FeatureSetName is the name of the legacy feature set.
const FeatureSetName = "Legacy-Gossip"

func init() {
	if err := message.RegisterType(MessageTypeTransactionAndRequest, TransactionAndRequestMessageDefinition); err != nil {
		panic(err)
	}
}

const (
	// The transaction payload + requested transaction hash gossipping packet.
	// In reality most of this packets won't take up their full 1604 bytes as the
	// signature message fragment of the tx is truncated.
	MessageTypeTransactionAndRequest message.Type = 2
)

const (
	// The amount of bytes used for the requested transaction hash.
	RequestedTransactionHashMsgBytesLength = 49
)

var (
	TransactionAndRequestMessageDefinition = &message.Definition{
		ID:             MessageTypeTransactionAndRequest,
		MaxBytesLength: RequestedTransactionHashMsgBytesLength + consts.NonSigTxPartBytesLength + consts.SigDataMaxBytesLength,
		VariableLength: true,
	}
)

// NewTransactionAndRequestMessage creates a new transaction and request message.
func NewTransactionAndRequestMessage(truncatedTxData []byte, requestedHash hornet.Hash) ([]byte, error) {

	msgBytesLength := uint16(len(truncatedTxData) + RequestedTransactionHashMsgBytesLength)
	buf := bytes.NewBuffer(make([]byte, 0, tlv.HeaderMessageDefinition.MaxBytesLength+msgBytesLength))

	if err := tlv.WriteHeader(buf, MessageTypeTransactionAndRequest, msgBytesLength); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.BigEndian, truncatedTxData); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.BigEndian, requestedHash[:RequestedTransactionHashMsgBytesLength]); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
