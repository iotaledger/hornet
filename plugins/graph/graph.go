package graph

import (
	"container/ring"
	"strconv"

	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
)

const (
	TX_BUFFER_SIZE       = 1800
	MS_BUFFER_SIZE       = 20
	BROADCAST_QUEUE_SIZE = 100
)

var (
	txRingBuffer *ring.Ring // transactions
	snRingBuffer *ring.Ring // confirmed transactions
	msRingBuffer *ring.Ring // Milestones

	txRingBufferLock = syncutils.Mutex{}
	snRingBufferLock = syncutils.Mutex{}
	msRingBufferLock = syncutils.Mutex{}
)

type wsTransaction struct {
	Hash              string `json:"hash"`
	Address           string `json:"address"`
	Value             string `json:"value"`
	Tag               string `json:"tag"`
	Timestamp         string `json:"timestamp"`
	CurrentIndex      string `json:"current_index"`
	LastIndex         string `json:"last_index"`
	Bundle            string `json:"bundle_hash"`
	TrunkTransaction  string `json:"transaction_trunk"`
	BranchTransaction string `json:"transaction_branch"`
}

type wsConfig struct {
	NetworkName string `json:"networkName"`
}

type wsTransactionSn struct {
	Hash              string `json:"hash"`
	Address           string `json:"address"`
	TrunkTransaction  string `json:"transaction_trunk"`
	BranchTransaction string `json:"transaction_branch"`
	Bundle            string `json:"bundle"`
}

type wsMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

func initRingBuffers() {
	txRingBuffer = ring.New(TX_BUFFER_SIZE)
	snRingBuffer = ring.New(TX_BUFFER_SIZE)
	msRingBuffer = ring.New(MS_BUFFER_SIZE)
}

func onNewTx(cachedTx *tangle.CachedTransaction) {

	cachedTx.ConsumeTransaction(func(tx *hornet.Transaction, metadata *hornet.TransactionMetadata) {

		wsTx := &wsTransaction{
			Hash:              tx.Tx.Hash,
			Address:           tx.Tx.Address,
			Value:             strconv.FormatInt(tx.Tx.Value, 10),
			Tag:               tx.Tx.Tag,
			Timestamp:         strconv.FormatInt(int64(tx.Tx.Timestamp), 10),
			CurrentIndex:      strconv.FormatInt(int64(tx.Tx.CurrentIndex), 10),
			LastIndex:         strconv.FormatInt(int64(tx.Tx.LastIndex), 10),
			Bundle:            tx.Tx.Bundle,
			TrunkTransaction:  tx.Tx.TrunkTransaction,
			BranchTransaction: tx.Tx.BranchTransaction,
		}

		txRingBufferLock.Lock()
		txRingBuffer.Value = wsTx
		txRingBuffer = txRingBuffer.Next()
		txRingBufferLock.Unlock()

		hub.broadcastMsg(&wsMessage{Type: "tx", Data: wsTx})
	})
}

func onConfirmedTx(cachedTx *tangle.CachedTransaction, msIndex milestone_index.MilestoneIndex, confTime int64) {

	cachedTx.ConsumeTransaction(func(tx *hornet.Transaction, metadata *hornet.TransactionMetadata) {
		snTx := &wsTransactionSn{
			Hash:              tx.Tx.Hash,
			Address:           tx.Tx.Address,
			TrunkTransaction:  tx.Tx.TrunkTransaction,
			BranchTransaction: tx.Tx.BranchTransaction,
			Bundle:            tx.Tx.Bundle,
		}

		snRingBufferLock.Lock()
		snRingBuffer.Value = snTx
		snRingBuffer = snRingBuffer.Next()
		snRingBufferLock.Unlock()

		hub.broadcastMsg(&wsMessage{Type: "sn", Data: snTx})
	})
}

func onNewMilestone(cachedBndl *tangle.CachedBundle) {

	cachedBndl.ConsumeBundle(func(bndl *tangle.Bundle) {
		msHash := bndl.GetMilestoneHash()

		msRingBufferLock.Lock()
		msRingBuffer.Value = msHash
		msRingBuffer = msRingBuffer.Next()
		msRingBufferLock.Unlock()

		hub.broadcastMsg(&wsMessage{Type: "ms", Data: msHash})
	})
}
