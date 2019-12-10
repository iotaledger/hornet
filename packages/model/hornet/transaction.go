package hornet

import (
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/bitutils"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/syncutils"
)

const (
	HORNET_TX_METADATA_SOLID     = 0
	HORNET_TX_METADATA_CONFIRMED = 1
	HORNET_TX_METADATA_REQUESTED = 2
)

func TransactionCaller(handler interface{}, params ...interface{}) {
	handler.(func(tx *Transaction))(params[0].(*Transaction))
}

func NewTransactionCaller(handler interface{}, params ...interface{}) {
	handler.(func(tx *Transaction, firstSeenLatestMilestoneIndex milestone_index.MilestoneIndex, latestSolidMilestoneIndex milestone_index.MilestoneIndex))(params[0].(*Transaction), params[1].(milestone_index.MilestoneIndex), params[2].(milestone_index.MilestoneIndex))
}

func TransactionConfirmedCaller(handler interface{}, params ...interface{}) {
	handler.(func(tx *Transaction, msIndex milestone_index.MilestoneIndex, confTime int64))(params[0].(*Transaction), params[1].(milestone_index.MilestoneIndex), params[2].(int64))
}

type Transaction struct {
	// Decompressed iota.go Transaction containing Hash
	Tx *transaction.Transaction

	// Compressed bytes as received from IRI
	RawBytes []byte

	// TxTimestamp or, if available, AttachmentTimestamp
	timestamp int64

	// The index of the milestone which confirmed this tx
	confirmationIndex milestone_index.MilestoneIndex

	// Metadata
	metadataMutex syncutils.RWMutex
	metadata      bitutils.BitMask

	// Status
	statusMutex syncutils.RWMutex
	modified    bool
}

func NewTransactionFromAPI(transaction *transaction.Transaction, transactionBytes []byte) *Transaction {
	return &Transaction{
		Tx:                transaction,
		RawBytes:          transactionBytes,
		timestamp:         getTimestampFromTx(transaction),
		confirmationIndex: 0,
		metadata:          bitutils.BitMask(byte(0)),
		modified:          true,
	}
}

func NewTransactionFromGossip(transaction *transaction.Transaction, transactionBytes []byte, requested bool) *Transaction {
	metadata := bitutils.BitMask(byte(0))
	if requested {
		metadata = metadata.SettingFlag(HORNET_TX_METADATA_REQUESTED)
	}

	return &Transaction{
		Tx:                transaction,
		RawBytes:          transactionBytes,
		timestamp:         getTimestampFromTx(transaction),
		confirmationIndex: 0,
		metadata:          metadata,
		modified:          true,
	}
}

func NewTransactionFromDatabase(transaction *transaction.Transaction, transactionBytes []byte, confirmationIndex milestone_index.MilestoneIndex, metadata byte) *Transaction {
	return &Transaction{
		Tx:                transaction,
		RawBytes:          transactionBytes,
		timestamp:         getTimestampFromTx(transaction),
		confirmationIndex: confirmationIndex,
		metadata:          bitutils.BitMask(metadata),
		modified:          false,
	}
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
		tx.metadata = tx.metadata.ModifyingFlag(HORNET_TX_METADATA_SOLID, solid)
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
		tx.metadata = tx.metadata.ModifyingFlag(HORNET_TX_METADATA_CONFIRMED, confirmed)
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
		tx.metadata = tx.metadata.ModifyingFlag(HORNET_TX_METADATA_REQUESTED, requested)
		tx.SetModified(true)
	}
}

func (tx *Transaction) GetMetadata() byte {
	tx.metadataMutex.RLock()
	defer tx.metadataMutex.RUnlock()

	return byte(tx.metadata)
}

func (tx *Transaction) IsModified() bool {
	tx.statusMutex.RLock()
	defer tx.statusMutex.RUnlock()

	return tx.modified
}

func (tx *Transaction) SetModified(modified bool) {
	tx.statusMutex.Lock()
	defer tx.statusMutex.Unlock()

	tx.modified = modified
}
