package graph

import (
	"container/ring"
	"strconv"

	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

const (
	txBufferSize          = 1800
	msBufferSize          = 20
	broadcastQueueSize    = 20000
	clientSendChannelSize = 1000
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
	txRingBuffer = ring.New(txBufferSize)
	snRingBuffer = ring.New(txBufferSize)
	msRingBuffer = ring.New(msBufferSize)
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

		hub.BroadcastMsg(&wsMessage{Type: "tx", Data: wsTx})
	})
}

func onConfirmedTx(cachedTx *tangle.CachedTransaction, _ milestone.Index, _ int64) {

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

		hub.BroadcastMsg(&wsMessage{Type: "sn", Data: snTx})
	})
}

func onNewMilestone(cachedBndl *tangle.CachedBundle) {

	cachedBndl.ConsumeBundle(func(bndl *tangle.Bundle) {
		msHash := bndl.GetMilestoneHash().Trytes()

		msRingBufferLock.Lock()
		msRingBuffer.Value = msHash
		msRingBuffer = msRingBuffer.Next()
		msRingBufferLock.Unlock()

		hub.BroadcastMsg(&wsMessage{Type: "ms", Data: msHash})
	})
}
