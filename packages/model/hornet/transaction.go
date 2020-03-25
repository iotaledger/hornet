package hornet

import (
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/packages/compressed"
	"github.com/gohornet/hornet/packages/metrics"
	"github.com/gohornet/hornet/packages/model/milestone_index"
)

func TransactionCaller(handler interface{}, params ...interface{}) {
	handler.(func(tx *Transaction))(params[0].(*Transaction))
}

func RequestedTransactionCaller(handler interface{}, params ...interface{}) {
	handler.(func(tx *Transaction, requested bool, requestedIndex milestone_index.MilestoneIndex, neighborMetrics *metrics.NeighborMetrics))(params[0].(*Transaction), params[1].(bool), params[2].(milestone_index.MilestoneIndex), params[3].(*metrics.NeighborMetrics))
}

type Transaction struct {
	objectstorage.StorableObjectFlags

	TxHash []byte

	// Decompressed iota.go Transaction containing Hash
	Tx *transaction.Transaction

	// Compressed bytes as received via gossip
	RawBytes []byte

	// TxTimestamp or, if available, AttachmentTimestamp
	timestamp int64
}

func NewTransaction(transaction *transaction.Transaction, transactionBytes []byte) *Transaction {
	tx := &Transaction{
		TxHash:    trinary.MustTrytesToBytes(transaction.Hash)[:49],
		Tx:        transaction,
		RawBytes:  transactionBytes,
		timestamp: getTimestampFromTx(transaction),
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

func (tx *Transaction) IsTail() bool {
	return tx.Tx.CurrentIndex == 0
}

func (tx *Transaction) IsHead() bool {
	return tx.Tx.CurrentIndex == tx.Tx.LastIndex
}

// ObjectStorage interface

func (tx *Transaction) Update(other objectstorage.StorableObject) {
	panic("Transaction should never be updated")
}

func (tx *Transaction) ObjectStorageKey() []byte {
	return tx.TxHash
}

func (tx *Transaction) ObjectStorageValue() (data []byte) {

	/*
		x bytes RawBytes
	*/

	return tx.RawBytes
}

func (tx *Transaction) UnmarshalObjectStorageValue(data []byte) error {

	/*
		x bytes RawBytes
	*/

	tx.RawBytes = data
	transactionHash := trinary.MustBytesToTrytes(tx.TxHash, 81)

	transaction, err := compressed.TransactionFromCompressedBytes(tx.RawBytes, transactionHash)
	if err != nil {
		panic(err)
	}
	tx.Tx = transaction

	tx.timestamp = getTimestampFromTx(transaction)

	return nil
}
