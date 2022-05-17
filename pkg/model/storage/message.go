package storage

import (
	"fmt"
	"sync"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

type Block struct {
	objectstorage.StorableObjectFlags

	// Key
	blockID hornet.BlockID

	// Value
	data        []byte
	messageOnce sync.Once
	message     *iotago.Block
}

func NewMessage(iotaMsg *iotago.Block, deSeriMode serializer.DeSerializationMode, protoParas *iotago.ProtocolParameters) (*Block, error) {

	data, err := iotaMsg.Serialize(deSeriMode, protoParas)
	if err != nil {
		return nil, err
	}

	msgHash, err := iotaMsg.ID()
	if err != nil {
		return nil, err
	}
	blockID := hornet.BlockIDFromArray(*msgHash)

	msg := &Block{blockID: blockID, data: data}

	msg.messageOnce.Do(func() {
		msg.message = iotaMsg
	})

	return msg, nil
}

func MessageFromBytes(data []byte, deSeriMode serializer.DeSerializationMode, protoParas *iotago.ProtocolParameters) (*Block, error) {

	iotaMsg := &iotago.Block{}
	if _, err := iotaMsg.Deserialize(data, deSeriMode, protoParas); err != nil {
		return nil, err
	}

	msgHash, err := iotaMsg.ID()
	if err != nil {
		return nil, err
	}
	blockID := hornet.BlockIDFromArray(*msgHash)

	msg := &Block{blockID: blockID, data: data}

	msg.messageOnce.Do(func() {
		msg.message = iotaMsg
	})

	return msg, nil
}

func (msg *Block) BlockID() hornet.BlockID {
	return msg.blockID
}

func (msg *Block) Data() []byte {
	return msg.data
}

func (msg *Block) Block() *iotago.Block {
	msg.messageOnce.Do(func() {
		iotaMsg := &iotago.Block{}
		// No need to verify the message again here
		if _, err := iotaMsg.Deserialize(msg.data, serializer.DeSeriModeNoValidation, nil); err != nil {
			panic(fmt.Sprintf("failed to deserialize message: %v, error: %s", msg.blockID.ToHex(), err))
		}

		msg.message = iotaMsg
	})
	return msg.message
}

func (msg *Block) ProtocolVersion() byte {
	return msg.Block().ProtocolVersion
}

func (msg *Block) Parents() hornet.BlockIDs {
	return hornet.BlockIDsFromSliceOfArrays(msg.Block().Parents)
}

func (msg *Block) IsMilestone() bool {
	switch msg.Block().Payload.(type) {
	case *iotago.Milestone:
		return true
	default:
	}

	return false
}

func (msg *Block) Milestone() *iotago.Milestone {
	switch milestonePayload := msg.Block().Payload.(type) {
	case *iotago.Milestone:
		return milestonePayload
	default:
	}

	return nil
}

func (msg *Block) IsTransaction() bool {
	switch msg.Block().Payload.(type) {
	case *iotago.Transaction:
		return true
	default:
	}

	return false
}

func (msg *Block) TaggedData() *iotago.TaggedData {

	switch payload := msg.Block().Payload.(type) {
	case *iotago.TaggedData:
		return payload
	default:
		return nil
	}
}

func (msg *Block) Transaction() *iotago.Transaction {
	switch payload := msg.Block().Payload.(type) {
	case *iotago.Transaction:
		return payload
	default:
		return nil
	}
}

func (msg *Block) TransactionEssence() *iotago.TransactionEssence {
	if transaction := msg.Transaction(); transaction != nil {
		return transaction.Essence
	}
	return nil
}

func (msg *Block) TransactionEssenceTaggedData() *iotago.TaggedData {

	if essence := msg.TransactionEssence(); essence != nil {
		switch payload := essence.Payload.(type) {
		case *iotago.TaggedData:
			return payload
		default:
			return nil
		}
	}
	return nil
}

func (msg *Block) TransactionEssenceUTXOInputs() []*iotago.OutputID {

	var inputs []*iotago.OutputID
	if essence := msg.TransactionEssence(); essence != nil {
		for _, input := range essence.Inputs {
			switch utxoInput := input.(type) {
			case *iotago.UTXOInput:
				id := utxoInput.ID()
				inputs = append(inputs, &id)
			default:
				return nil
			}
		}
	}
	return inputs
}

func (msg *Block) SignatureForInputIndex(inputIndex uint16) *iotago.Ed25519Signature {

	if transaction := msg.Transaction(); transaction != nil {
		switch unlockBlock := transaction.Unlocks[inputIndex].(type) {
		case *iotago.SignatureUnlock:
			switch signature := unlockBlock.Signature.(type) {
			case *iotago.Ed25519Signature:
				return signature
			default:
				return nil
			}
		case *iotago.ReferenceUnlock:
			return msg.SignatureForInputIndex(unlockBlock.Reference)
		default:
			return nil
		}
	}
	return nil
}

// ObjectStorage interface

func (msg *Block) Update(_ objectstorage.StorableObject) {
	panic(fmt.Sprintf("Block should never be updated: %v", msg.blockID.ToHex()))
}

func (msg *Block) ObjectStorageKey() []byte {
	return msg.blockID
}

func (msg *Block) ObjectStorageValue() []byte {
	return msg.data
}
