package hornet

import (
	"encoding/binary"
	"time"

	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/packages/compressed"
	"github.com/gohornet/hornet/packages/model/milestone_index"
)

const (
	HORNET_TX_METADATA_SOLID     = 0
	HORNET_TX_METADATA_CONFIRMED = 1
	HORNET_TX_METADATA_REQUESTED = 2
)

func TransactionCaller(handler interface{}, params ...interface{}) {
	handler.(func(tx *Transaction))(params[0].(*Transaction))
}

type Transaction struct {
	objectstorage.StorableObjectFlags
	TxHash []byte

	// Decompressed iota.go Transaction containing Hash
	Tx *transaction.Transaction

	// Compressed bytes as received from IRI
	RawBytes []byte

	// TxTimestamp or, if available, AttachmentTimestamp
	timestamp int64

	// Unix time when the Tx became solid (needed for local modifiers for tipselection)
	solidificationTimestamp int32

	// The index of the milestone which confirmed this tx
	confirmationIndex milestone_index.MilestoneIndex

	// Metadata
	metadataMutex syncutils.RWMutex
	metadata      bitmask.BitMask
}

func NewTransactionFromAPI(transaction *transaction.Transaction, transactionBytes []byte) *Transaction {
	tx := &Transaction{
		TxHash:            trinary.MustTrytesToBytes(transaction.Hash),
		Tx:                transaction,
		RawBytes:          transactionBytes,
		timestamp:         getTimestampFromTx(transaction),
		confirmationIndex: 0,
		metadata:          bitmask.BitMask(byte(0)),
	}
	tx.SetModified(true)
	return tx
}

func NewTransactionFromGossip(transaction *transaction.Transaction, transactionBytes []byte, requested bool) *Transaction {
	metadata := bitmask.BitMask(byte(0))
	if requested {
		metadata = metadata.SetFlag(HORNET_TX_METADATA_REQUESTED)
	}

	tx := &Transaction{
		TxHash:            trinary.MustTrytesToBytes(transaction.Hash),
		Tx:                transaction,
		RawBytes:          transactionBytes,
		timestamp:         getTimestampFromTx(transaction),
		confirmationIndex: 0,
		metadata:          metadata,
	}
	tx.SetModified(true)
	return tx
}

func getTimestampFromTx(transaction *transaction.Transaction) int64 {
	// Timestamp = Seconds elapsed since 00:00:00 UTC 1 January 1970
	timestamp := int64(transaction.Timestamp)
	if transaction.AttachmentTimestamp != 0 {
		// AttachmentTimestamp = Milliseconds elapsed since 00:00:00 UTC 1 January 1970
		timestamp = int64(transaction.AttachmentTimestamp / 1000)
	}
	return timestamp
}

func (tx *Transaction) GetHash() trinary.Hash {
	return tx.Tx.Hash
}

func (tx *Transaction) GetTrunk() trinary.Hash {
	return tx.Tx.TrunkTransaction
}

func (tx *Transaction) GetBranch() trinary.Hash {
	return tx.Tx.BranchTransaction
}

func (tx *Transaction) GetTimestamp() int64 {
	return tx.timestamp
}

func (tx *Transaction) GetSolidificationTimestamp() int32 {
	return tx.solidificationTimestamp
}

func (tx *Transaction) IsTail() bool {
	return tx.Tx.CurrentIndex == 0
}

func (tx *Transaction) IsHead() bool {
	return tx.Tx.CurrentIndex == tx.Tx.LastIndex
}

func (tx *Transaction) IsSolid() bool {
	tx.metadataMutex.RLock()
	defer tx.metadataMutex.RUnlock()
	s := tx.metadata.HasFlag(HORNET_TX_METADATA_SOLID)
	return s
}

func (tx *Transaction) SetSolid(solid bool) {
	tx.metadataMutex.Lock()
	defer tx.metadataMutex.Unlock()

	if solid != tx.metadata.HasFlag(HORNET_TX_METADATA_SOLID) {
		tx.solidificationTimestamp = int32(time.Now().Unix())
		tx.metadata = tx.metadata.ModifyFlag(HORNET_TX_METADATA_SOLID, solid)
		tx.SetModified(true)
	}
}

func (tx *Transaction) GetConfirmed() (bool, milestone_index.MilestoneIndex) {
	tx.metadataMutex.RLock()
	defer tx.metadataMutex.RUnlock()

	return tx.metadata.HasFlag(HORNET_TX_METADATA_CONFIRMED), tx.confirmationIndex
}

func (tx *Transaction) SetConfirmed(confirmed bool, confirmationIndex milestone_index.MilestoneIndex) {
	tx.metadataMutex.Lock()
	defer tx.metadataMutex.Unlock()

	if (confirmed != tx.metadata.HasFlag(HORNET_TX_METADATA_CONFIRMED)) || (tx.confirmationIndex != confirmationIndex) {
		tx.metadata = tx.metadata.ModifyFlag(HORNET_TX_METADATA_CONFIRMED, confirmed)
		tx.confirmationIndex = confirmationIndex
		tx.SetModified(true)
	}
}

func (tx *Transaction) IsRequested() bool {
	tx.metadataMutex.RLock()
	defer tx.metadataMutex.RUnlock()
	r := tx.metadata.HasFlag(HORNET_TX_METADATA_REQUESTED)
	return r
}

func (tx *Transaction) SetRequested(requested bool) {
	tx.metadataMutex.Lock()
	defer tx.metadataMutex.Unlock()

	if requested != tx.metadata.HasFlag(HORNET_TX_METADATA_REQUESTED) {
		tx.metadata = tx.metadata.ModifyFlag(HORNET_TX_METADATA_REQUESTED, requested)
		tx.SetModified(true)
	}
}

func (tx *Transaction) GetMetadata() byte {
	tx.metadataMutex.RLock()
	defer tx.metadataMutex.RUnlock()

	return byte(tx.metadata)
}

// ObjectStorage interface

func (tx *Transaction) Update(other objectstorage.StorableObject) {
	if obj, ok := other.(*Transaction); !ok {
		panic("invalid object passed to Transaction.Update()")
	} else {
		tx.confirmationIndex = obj.confirmationIndex
		tx.timestamp = obj.timestamp
		tx.solidificationTimestamp = obj.solidificationTimestamp
		tx.Tx = obj.Tx
		tx.RawBytes = obj.RawBytes
		tx.metadata = obj.metadata
	}
}

func (tx *Transaction) GetStorageKey() []byte {
	return tx.TxHash
}

func (tx *Transaction) MarshalBinary() (data []byte, err error) {

	/*
		1 byte  metadata bitmask
		4 bytes uint32 confirmationIndex
		4 bytes uint32 solidificationTimestamp
		x bytes RawBytes
	*/

	value := make([]byte, 9, 9+len(tx.RawBytes))
	confirmed, confirmationIndex := tx.GetConfirmed()

	if !confirmed {
		confirmationIndex = 0
	}

	value[0] = tx.GetMetadata()
	binary.LittleEndian.PutUint32(value[1:], uint32(confirmationIndex))
	binary.LittleEndian.PutUint32(value[5:], uint32(tx.GetSolidificationTimestamp()))
	value = append(value, tx.RawBytes...)

	return value, nil
}

func (tx *Transaction) UnmarshalBinary(data []byte) error {

	tx.RawBytes = data[9:]
	transactionHash := trinary.MustBytesToTrytes(tx.TxHash, 81)

	transaction, err := compressed.TransactionFromCompressedBytes(tx.RawBytes, transactionHash)
	if err != nil {
		panic(err)
	}
	tx.Tx = transaction

	tx.metadata = bitmask.BitMask(data[0])
	tx.confirmationIndex = milestone_index.MilestoneIndex(binary.LittleEndian.Uint32(data[1:5]))
	tx.solidificationTimestamp = int32(binary.LittleEndian.Uint32(data[5:9]))
	tx.timestamp = getTimestampFromTx(transaction)

	return nil
}
