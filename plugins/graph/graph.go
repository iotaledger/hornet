package graph

import (
	"container/ring"
	"strconv"

	"github.com/gorilla/websocket"

	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/packages/config"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
)

const (
	TX_BUFFER_SIZE = 1800
)

var (
	txRingBuffer *ring.Ring // transactions
	snRingBuffer *ring.Ring // confirmed transactions
	msRingBuffer *ring.Ring // Milestones

	upgrader = websocket.Upgrader{}
	clients  = make(map[*websocket.Conn]bool)

	clientsLock      = syncutils.Mutex{}
	broadcastLock    = syncutils.Mutex{}
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
	msRingBuffer = ring.New(20)
}

func onConnect(c *websocket.Conn) {
	log.Info("WebSocket client connection established")

	config := &wsConfig{NetworkName: config.NodeConfig.GetString(config.CfgGraphNetworkName)}

	var initTxs []*wsTransaction
	txRingBuffer.Do(func(tx interface{}) {
		if tx != nil {
			initTxs = append(initTxs, tx.(*wsTransaction))
		}
	})

	var initSns []*wsTransactionSn
	snRingBuffer.Do(func(sn interface{}) {
		if sn != nil {
			initSns = append(initSns, sn.(*wsTransactionSn))
		}
	})

	var initMs []string
	msRingBuffer.Do(func(ms interface{}) {
		if ms != nil {
			initMs = append(initMs, ms.(string))
		}
	})

	c.WriteJSON(&wsMessage{Type: "config", Data: config})
	c.WriteJSON(&wsMessage{Type: "inittx", Data: initTxs})
	c.WriteJSON(&wsMessage{Type: "initsn", Data: initSns})
	c.WriteJSON(&wsMessage{Type: "initms", Data: initMs})
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

		msg := &wsMessage{Type: "tx", Data: wsTx}
		select {
		case broadcast <- msg:
		default:
		}

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

		msg := &wsMessage{Type: "sn", Data: snTx}
		select {
		case broadcast <- msg:
		default:
		}
	})
}

func onNewMilestone(cachedBndl *tangle.CachedBundle) {
	msHash := cachedBndl.GetBundle().GetMilestoneHash()
	cachedBndl.Release(true) // bundle -1

	msRingBufferLock.Lock()
	msRingBuffer.Value = msHash
	msRingBuffer = msRingBuffer.Next()
	msRingBufferLock.Unlock()

	msg := &wsMessage{Type: "ms", Data: msHash}
	select {
	case broadcast <- msg:
	default:
	}
}
