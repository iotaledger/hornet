package hornet

import (
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/pkg/compressed"
)

type Transaction struct {
	objectstorage.StorableObjectFlags

	txHashOnce     syncutils.Once
	trunkHashOnce  syncutils.Once
	branchHashOnce syncutils.Once
	bundleHashOnce syncutils.Once
	tagOnce        syncutils.Once
	addressOnce    syncutils.Once

	txHash     Hash
	trunkHash  Hash
	branchHash Hash
	bundleHash Hash
	tag        Hash
	address    Hash

	// Decompressed iota.go Transaction containing Hash
	Tx *transaction.Transaction

	// Compressed bytes as received via gossip
	RawBytes []byte

	// TxTimestamp or, if available, AttachmentTimestamp
	timestamp int64
}

func NewTransaction(txHash Hash) *Transaction {
	return &Transaction{
		txHash: txHash,
	}
}

func NewTransactionFromTx(transaction *transaction.Transaction, transactionBytes []byte) *Transaction {
	tx := &Transaction{
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
		timestamp = transaction.AttachmentTimestamp / 1000
	}
	return timestamp
}

func (tx *Transaction) GetTxHash() Hash {
	tx.txHashOnce.Do(func() {
		tx.txHash = trinary.MustTrytesToBytes(tx.Tx.Hash)[:49]
	})

	return tx.txHash
}

func (tx *Transaction) GetTrunkHash() Hash {
	tx.trunkHashOnce.Do(func() {
		tx.trunkHash = trinary.MustTrytesToBytes(tx.Tx.TrunkTransaction)[:49]
	})

	return tx.trunkHash
}

func (tx *Transaction) GetBranchHash() Hash {
	tx.branchHashOnce.Do(func() {
		tx.branchHash = trinary.MustTrytesToBytes(tx.Tx.BranchTransaction)[:49]
	})

	return tx.branchHash
}

func (tx *Transaction) GetBundleHash() Hash {
	tx.bundleHashOnce.Do(func() {
		tx.bundleHash = trinary.MustTrytesToBytes(tx.Tx.Bundle)[:49]
	})
	return tx.bundleHash
}

func (tx *Transaction) GetTag() Hash {
	tx.tagOnce.Do(func() {
		tx.tag = trinary.MustTrytesToBytes(tx.Tx.Tag)[:17]
	})
	return tx.tag
}

func (tx *Transaction) GetAddress() Hash {
	tx.addressOnce.Do(func() {
		tx.address = trinary.MustTrytesToBytes(tx.Tx.Address)[:49]
	})
	return tx.address
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

func (tx *Transaction) IsValue() bool {
	return tx.Tx.Value != 0
}

// ObjectStorage interface

func (tx *Transaction) Update(_ objectstorage.StorableObject) {
	panic("Transaction should never be updated")
}

func (tx *Transaction) ObjectStorageKey() []byte {
	return tx.GetTxHash()
}

func (tx *Transaction) ObjectStorageValue() (data []byte) {

	/*
		x bytes RawBytes
	*/

	return tx.RawBytes
}

func (tx *Transaction) UnmarshalObjectStorageValue(data []byte) (consumedBytes int, err error) {

	/*
		x bytes RawBytes
	*/

	tx.RawBytes = data
	transactionHash := trinary.MustBytesToTrytes(tx.txHash, 81)

	transaction, err := compressed.TransactionFromCompressedBytes(tx.RawBytes, transactionHash)
	if err != nil {
		panic(err)
	}
	tx.Tx = transaction

	tx.timestamp = getTimestampFromTx(transaction)

	return len(data), nil
}
