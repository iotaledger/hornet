package tangle

import (
	"time"

	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/gohornet/hornet/packages/model/hornet"
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
	objectstorage.StorableObject

	TxHash []byte

	cachedRawData  *hornet.CachedTransactionRawData
	cachedMetaData *hornet.CachedTransactionMetaData

	// Decompressed iota.go Transaction containing Hash
	Tx *transaction.Transaction

	// TxTimestamp or, if available, AttachmentTimestamp
	timestamp int64
}

func NewTransactionFromAPI(transaction *transaction.Transaction, transactionBytes []byte) *Transaction {

	txHashBytes := trinary.MustTrytesToBytes(transaction.Hash)[:49]

	transactionRawData := &hornet.TransactionRawData{
		TxHash:   txHashBytes,
		RawBytes: transactionBytes,
	}

	cachedTransactionRawData, rawDataIsNew := txRawStorage.StoreIfAbsent(transactionRawData)
	if !rawDataIsNew {
		return nil
	}

	transactionMetaData := &hornet.TransactionMetaData{
		TxHash:                  txHashBytes,
		Metadata:                bitmask.BitMask(byte(0)),
		ReqMilestoneIndex:       0,
		SolidificationTimestamp: 0,
		ConfirmationIndex:       0,
	}

	cachedTransactionMetaData, metaDataIsNew := txMetaStorage.StoreIfAbsent(transactionMetaData)
	if !metaDataIsNew {
		return nil
	}

	tx := &Transaction{
		TxHash:         txHashBytes,
		cachedRawData:  &hornet.CachedTransactionRawData{CachedObject: cachedTransactionRawData},
		cachedMetaData: &hornet.CachedTransactionMetaData{CachedObject: cachedTransactionMetaData},
		Tx:             transaction,
		timestamp:      getTimestampFromTx(transaction),
	}

	return tx
}

func NewTransactionFromGossip(transaction *transaction.Transaction, transactionBytes []byte, requested bool, reqMilestoneIndex milestone_index.MilestoneIndex) *Transaction {
	txHashBytes := trinary.MustTrytesToBytes(transaction.Hash)[:49]

	transactionRawData := &hornet.TransactionRawData{
		TxHash:   txHashBytes,
		RawBytes: transactionBytes,
	}

	cachedTransactionRawData, rawDataIsNew := txRawStorage.StoreIfAbsent(transactionRawData)
	if !rawDataIsNew {
		return nil
	}

	metadata := bitmask.BitMask(byte(0))
	if requested {
		metadata = metadata.SetFlag(HORNET_TX_METADATA_REQUESTED)
	}

	transactionMetaData := &hornet.TransactionMetaData{
		TxHash:                  txHashBytes,
		Metadata:                metadata,
		ReqMilestoneIndex:       reqMilestoneIndex,
		SolidificationTimestamp: 0,
		ConfirmationIndex:       0,
	}

	cachedTransactionMetaData, metaDataIsNew := txMetaStorage.StoreIfAbsent(transactionMetaData)
	if !metaDataIsNew {
		return nil
	}

	tx := &Transaction{
		TxHash:         txHashBytes,
		cachedRawData:  &hornet.CachedTransactionRawData{CachedObject: cachedTransactionRawData},
		cachedMetaData: &hornet.CachedTransactionMetaData{CachedObject: cachedTransactionMetaData},
		Tx:             transaction,
		timestamp:      getTimestampFromTx(transaction),
	}

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

func (tx *Transaction) GetRawBytes() []byte {
	return tx.cachedRawData.GetTransactionRawData().RawBytes
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
	return tx.cachedMetaData.GetTransactionMetaData().SolidificationTimestamp
}

func (tx *Transaction) IsTail() bool {
	return tx.Tx.CurrentIndex == 0
}

func (tx *Transaction) IsHead() bool {
	return tx.Tx.CurrentIndex == tx.Tx.LastIndex
}

func (tx *Transaction) IsSolid() bool {
	tx.cachedMetaData.GetTransactionMetaData().RLock()
	defer tx.cachedMetaData.GetTransactionMetaData().RUnlock()

	return tx.cachedMetaData.GetTransactionMetaData().Metadata.HasFlag(HORNET_TX_METADATA_SOLID)
}

func (tx *Transaction) SetSolid(solid bool) {
	tx.cachedMetaData.GetTransactionMetaData().Lock()
	defer tx.cachedMetaData.GetTransactionMetaData().Unlock()

	if solid != tx.cachedMetaData.GetTransactionMetaData().Metadata.HasFlag(HORNET_TX_METADATA_SOLID) {
		if solid {
			tx.cachedMetaData.GetTransactionMetaData().SolidificationTimestamp = int32(time.Now().Unix())
		} else {
			tx.cachedMetaData.GetTransactionMetaData().SolidificationTimestamp = 0
		}
		tx.cachedMetaData.GetTransactionMetaData().Metadata = tx.cachedMetaData.GetTransactionMetaData().Metadata.ModifyFlag(HORNET_TX_METADATA_SOLID, solid)
		tx.cachedMetaData.GetTransactionMetaData().SetModified(true)
	}
}

func (tx *Transaction) GetConfirmed() (bool, milestone_index.MilestoneIndex) {
	tx.cachedMetaData.GetTransactionMetaData().RLock()
	defer tx.cachedMetaData.GetTransactionMetaData().RUnlock()

	return tx.cachedMetaData.GetTransactionMetaData().Metadata.HasFlag(HORNET_TX_METADATA_CONFIRMED), tx.cachedMetaData.GetTransactionMetaData().ConfirmationIndex
}

func (tx *Transaction) SetConfirmed(confirmed bool, confirmationIndex milestone_index.MilestoneIndex) {
	tx.cachedMetaData.GetTransactionMetaData().Lock()
	defer tx.cachedMetaData.GetTransactionMetaData().Unlock()

	if (confirmed != tx.cachedMetaData.GetTransactionMetaData().Metadata.HasFlag(HORNET_TX_METADATA_CONFIRMED)) || (tx.cachedMetaData.GetTransactionMetaData().ConfirmationIndex != confirmationIndex) {
		tx.cachedMetaData.GetTransactionMetaData().Metadata = tx.cachedMetaData.GetTransactionMetaData().Metadata.ModifyFlag(HORNET_TX_METADATA_CONFIRMED, confirmed)
		tx.cachedMetaData.GetTransactionMetaData().ConfirmationIndex = confirmationIndex
		tx.cachedMetaData.GetTransactionMetaData().SetModified(true)
	}
}

func (tx *Transaction) IsRequested() (bool, milestone_index.MilestoneIndex) {
	tx.cachedMetaData.GetTransactionMetaData().RLock()
	defer tx.cachedMetaData.GetTransactionMetaData().RUnlock()

	requested := tx.cachedMetaData.GetTransactionMetaData().Metadata.HasFlag(HORNET_TX_METADATA_REQUESTED)
	return requested, tx.cachedMetaData.GetTransactionMetaData().ReqMilestoneIndex
}

func (tx *Transaction) SetRequested(requested bool, reqMilestoneIndex milestone_index.MilestoneIndex) {
	tx.cachedMetaData.GetTransactionMetaData().Lock()
	defer tx.cachedMetaData.GetTransactionMetaData().Unlock()

	if requested != tx.cachedMetaData.GetTransactionMetaData().Metadata.HasFlag(HORNET_TX_METADATA_REQUESTED) {
		tx.cachedMetaData.GetTransactionMetaData().Metadata = tx.cachedMetaData.GetTransactionMetaData().Metadata.ModifyFlag(HORNET_TX_METADATA_REQUESTED, requested)
		if requested {
			tx.cachedMetaData.GetTransactionMetaData().ReqMilestoneIndex = reqMilestoneIndex
		} else {
			tx.cachedMetaData.GetTransactionMetaData().ReqMilestoneIndex = 0
		}

		tx.cachedMetaData.GetTransactionMetaData().SetModified(true)
	}
}

func (tx *Transaction) GetMetadata() byte {
	tx.cachedMetaData.GetTransactionMetaData().RLock()
	defer tx.cachedMetaData.GetTransactionMetaData().RUnlock()

	return byte(tx.cachedMetaData.GetTransactionMetaData().Metadata)
}
