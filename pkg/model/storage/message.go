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
	data      []byte
	blockOnce sync.Once
	block     *iotago.Block
}

func NewBlock(iotaBlock *iotago.Block, deSeriMode serializer.DeSerializationMode, protoParas *iotago.ProtocolParameters) (*Block, error) {

	data, err := iotaBlock.Serialize(deSeriMode, protoParas)
	if err != nil {
		return nil, err
	}

	blockHash, err := iotaBlock.ID()
	if err != nil {
		return nil, err
	}
	blockID := hornet.BlockIDFromArray(*blockHash)

	msg := &Block{blockID: blockID, data: data}

	msg.blockOnce.Do(func() {
		msg.block = iotaBlock
	})

	return msg, nil
}

func BlockFromBytes(data []byte, deSeriMode serializer.DeSerializationMode, protoParas *iotago.ProtocolParameters) (*Block, error) {

	iotaBlock := &iotago.Block{}
	if _, err := iotaBlock.Deserialize(data, deSeriMode, protoParas); err != nil {
		return nil, err
	}

	msgHash, err := iotaBlock.ID()
	if err != nil {
		return nil, err
	}
	blockID := hornet.BlockIDFromArray(*msgHash)

	block := &Block{blockID: blockID, data: data}

	block.blockOnce.Do(func() {
		block.block = iotaBlock
	})

	return block, nil
}

func (msg *Block) BlockID() hornet.BlockID {
	return msg.blockID
}

func (msg *Block) Data() []byte {
	return msg.data
}

func (msg *Block) Block() *iotago.Block {
	msg.blockOnce.Do(func() {
		iotaMsg := &iotago.Block{}
		// No need to verify the block again here
		if _, err := iotaMsg.Deserialize(msg.data, serializer.DeSeriModeNoValidation, nil); err != nil {
			panic(fmt.Sprintf("failed to deserialize block: %v, error: %s", msg.blockID.ToHex(), err))
		}

		msg.block = iotaMsg
	})
	return msg.block
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
