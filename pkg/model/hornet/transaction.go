package hornet

import (
	"fmt"
	"sync"

	"github.com/iotaledger/iota.go/transaction"

	"github.com/gohornet/hornet/pkg/compressed"
	"github.com/iotaledger/hive.go/objectstorage"
)

type Transaction struct {
	objectstorage.StorableObjectFlags

	txHashOnce     sync.Once
	trunkHashOnce  sync.Once
	branchHashOnce sync.Once
	bundleHashOnce sync.Once
	tagOnce        sync.Once
	addressOnce    sync.Once

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
		tx.txHash = HashFromHashTrytes(tx.Tx.Hash)
	})

	return tx.txHash
}

func (tx *Transaction) GetTrunkHash() Hash {
	tx.trunkHashOnce.Do(func() {
		tx.trunkHash = HashFromHashTrytes(tx.Tx.TrunkTransaction)
	})

	return tx.trunkHash
}

func (tx *Transaction) GetBranchHash() Hash {
	tx.branchHashOnce.Do(func() {
		tx.branchHash = HashFromHashTrytes(tx.Tx.BranchTransaction)
	})

	return tx.branchHash
}

func (tx *Transaction) GetBundleHash() Hash {
	tx.bundleHashOnce.Do(func() {
		tx.bundleHash = HashFromHashTrytes(tx.Tx.Bundle)
	})
	return tx.bundleHash
}

func (tx *Transaction) GetTag() Hash {
	tx.tagOnce.Do(func() {
		tx.tag = HashFromTagTrytes(tx.Tx.Tag)
	})
	return tx.tag
}

func (tx *Transaction) GetAddress() Hash {
	tx.addressOnce.Do(func() {
		tx.address = HashFromAddressTrytes(tx.Tx.Address)
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
	panic(fmt.Sprintf("Transaction should never be updated: %v", tx.txHash.Trytes()))
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
	transactionHash := tx.txHash.Trytes()

	transaction, err := compressed.TransactionFromCompressedBytes(tx.RawBytes, transactionHash)
	if err != nil {
		panic(err)
	}
	tx.Tx = transaction

	tx.timestamp = getTimestampFromTx(transaction)

	return len(data), nil
}
