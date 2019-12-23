package graph

import (
	"container/ring"
	"fmt"
	"strconv"

	socketio "github.com/googollee/go-socket.io"

	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/parameter"
	"github.com/gohornet/hornet/packages/syncutils"
)

const (
	TX_BUFFER_SIZE = 1800
)

var (
	txRingBuffer *ring.Ring // transactions
	snRingBuffer *ring.Ring // confirmed transactions
	msRingBuffer *ring.Ring // Milestones

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

func initRingBuffers() {
	txRingBuffer = ring.New(TX_BUFFER_SIZE)
	snRingBuffer = ring.New(TX_BUFFER_SIZE)
	msRingBuffer = ring.New(20)
}

func onConnectHandler(s socketio.Conn) error {
	infoMsg := "Graph client connection established"
	if s != nil {
		infoMsg = fmt.Sprintf("%s (ID: %v)", infoMsg, s.ID())
	}
	log.Info(infoMsg)
	socketioServer.JoinRoom("broadcast", s)

	config := &wsConfig{NetworkName: parameter.NodeConfig.GetString("graph.networkName")}

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

	s.Emit("config", config)
	s.Emit("inittx", initTxs)
	s.Emit("initsn", initSns)
	s.Emit("initms", initMs)
	s.Emit("donation", "0")
	s.Emit("donations", []int{})
	s.Emit("donation-address", "-")

	return nil
}

func onErrorHandler(s socketio.Conn, e error) {
	errorMsg := "Graph meet error"
	if e != nil {
		errorMsg = fmt.Sprintf("%s: %s", errorMsg, e.Error())
	}
	log.Error(errorMsg)
}

func onDisconnectHandler(s socketio.Conn, msg string) {
	infoMsg := "Graph client connection closed"
	if s != nil {
		infoMsg = fmt.Sprintf("%s (ID: %v)", infoMsg, s.ID())
	}
	log.Info(fmt.Sprintf("%s: %s", infoMsg, msg))
	socketioServer.LeaveAllRooms(s)
}

func onNewTx(tx *hornet.Transaction) {
	wsTx := &wsTransaction{
		Hash:              tx.GetHash(),
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

	broadcastLock.Lock()
	socketioServer.BroadcastToRoom("broadcast", "tx", wsTx)
	broadcastLock.Unlock()
}

func onConfirmedTx(tx *hornet.Transaction, msIndex milestone_index.MilestoneIndex, confTime int64) {
	snTx := &wsTransactionSn{
		Hash:              tx.GetHash(),
		Address:           tx.Tx.Address,
		TrunkTransaction:  tx.Tx.TrunkTransaction,
		BranchTransaction: tx.Tx.BranchTransaction,
		Bundle:            tx.Tx.Bundle,
	}

	snRingBufferLock.Lock()
	snRingBuffer.Value = snTx
	snRingBuffer = snRingBuffer.Next()
	snRingBufferLock.Unlock()

	broadcastLock.Lock()
	socketioServer.BroadcastToRoom("broadcast", "sn", snTx)
	broadcastLock.Unlock()
}

func onNewMilestone(bundle *tangle.Bundle) {
	msHash := bundle.GetMilestoneHash()

	msRingBufferLock.Lock()
	msRingBuffer.Value = msHash
	msRingBuffer = msRingBuffer.Next()
	msRingBufferLock.Unlock()

	broadcastLock.Lock()
	socketioServer.BroadcastToRoom("broadcast", "ms", msHash)
	broadcastLock.Unlock()
}
