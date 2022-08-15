package storage

import (
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/core/objectstorage"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

type Block struct {
	objectstorage.StorableObjectFlags

	// Key
	blockID iotago.BlockID

	// Value
	data      []byte
	blockOnce sync.Once
	block     *iotago.Block
}

func NewBlock(iotaBlock *iotago.Block, deSeriMode serializer.DeSerializationMode, protoParams *iotago.ProtocolParameters) (*Block, error) {

	data, err := iotaBlock.Serialize(deSeriMode, protoParams)
	if err != nil {
		return nil, err
	}

	blockID, err := iotaBlock.ID()
	if err != nil {
		return nil, err
	}

	block := &Block{blockID: blockID, data: data}

	block.blockOnce.Do(func() {
		block.block = iotaBlock
	})

	return block, nil
}

func BlockFromBytes(data []byte, deSeriMode serializer.DeSerializationMode, protoParams *iotago.ProtocolParameters) (*Block, error) {

	iotaBlock := &iotago.Block{}
	if _, err := iotaBlock.Deserialize(data, deSeriMode, protoParams); err != nil {
		return nil, err
	}

	blockID, err := iotaBlock.ID()
	if err != nil {
		return nil, err
	}

	block := &Block{blockID: blockID, data: data}

	block.blockOnce.Do(func() {
		block.block = iotaBlock
	})

	return block, nil
}

func (blk *Block) BlockID() iotago.BlockID {
	return blk.blockID
}

func (blk *Block) Data() []byte {
	return blk.data
}

func (blk *Block) Block() *iotago.Block {
	blk.blockOnce.Do(func() {
		iotaBlock := &iotago.Block{}
		// No need to verify the block again here
		if _, err := iotaBlock.Deserialize(blk.data, serializer.DeSeriModeNoValidation, nil); err != nil {
			panic(fmt.Sprintf("failed to deserialize block: %v, error: %s", blk.blockID.ToHex(), err))
		}

		blk.block = iotaBlock
	})

	return blk.block
}

func (blk *Block) ProtocolVersion() byte {
	return blk.Block().ProtocolVersion
}

func (blk *Block) Parents() iotago.BlockIDs {
	return blk.Block().Parents
}

func (blk *Block) IsMilestone() bool {
	switch blk.Block().Payload.(type) {
	case *iotago.Milestone:
		return true
	default:
	}

	return false
}

func (blk *Block) Milestone() *iotago.Milestone {
	switch milestonePayload := blk.Block().Payload.(type) {
	case *iotago.Milestone:
		return milestonePayload
	default:
	}

	return nil
}

func (blk *Block) IsTransaction() bool {
	switch blk.Block().Payload.(type) {
	case *iotago.Transaction:
		return true
	default:
	}

	return false
}

func (blk *Block) TaggedData() *iotago.TaggedData {

	switch payload := blk.Block().Payload.(type) {
	case *iotago.TaggedData:
		return payload
	default:
		return nil
	}
}

func (blk *Block) Transaction() *iotago.Transaction {
	switch payload := blk.Block().Payload.(type) {
	case *iotago.Transaction:
		return payload
	default:
		return nil
	}
}

func (blk *Block) TransactionEssence() *iotago.TransactionEssence {
	if transaction := blk.Transaction(); transaction != nil {
		return transaction.Essence
	}

	return nil
}

func (blk *Block) TransactionEssenceTaggedData() *iotago.TaggedData {

	if essence := blk.TransactionEssence(); essence != nil {
		switch payload := essence.Payload.(type) {
		case *iotago.TaggedData:
			return payload
		default:
			return nil
		}
	}

	return nil
}

func (blk *Block) TransactionEssenceUTXOInputs() iotago.OutputIDs {

	var inputs iotago.OutputIDs
	if essence := blk.TransactionEssence(); essence != nil {
		for _, input := range essence.Inputs {
			switch utxoInput := input.(type) {
			case *iotago.UTXOInput:
				id := utxoInput.ID()
				inputs = append(inputs, id)
			default:
				return nil
			}
		}
	}

	return inputs
}

// ObjectStorage interface

func (blk *Block) Update(_ objectstorage.StorableObject) {
	panic(fmt.Sprintf("Block should never be updated: %v", blk.blockID.ToHex()))
}

func (blk *Block) ObjectStorageKey() []byte {
	return blk.blockID[:]
}

func (blk *Block) ObjectStorageValue() []byte {
	return blk.data
}
