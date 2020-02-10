package monitor

import (
	"container/ring"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	socketio "github.com/googollee/go-socket.io"

	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
)

const (
	TX_BUFFER_SIZE = 50000
)

var (
	txRingBuffer *ring.Ring
	txPointerMap map[string]*wsTransaction

	txRingBufferLock = syncutils.Mutex{}
	broadcastLock    = syncutils.Mutex{}
)

type (
	wsTransaction struct {
		Hash       string `json:"hash"`
		Address    string `json:"address"`
		Value      int64  `json:"value"`
		Tag        string `json:"tag"`
		Confirmed  bool   `json:"confirmed"`
		Reattached bool   `json:"reattached"`
		Bundle     string `json:"bundle"`
		ReceivedAt int64  `json:"receivedAt"`
		ConfTime   int64  `json:"ctime"`
		Milestone  string `json:"milestone"`
		//TrunkTransaction  string `json:"trunk"`
		//BranchTransaction string `json:"branch"`
	}

	getRecentTransactions struct {
		TXHistory []*wsTransaction `json:"txHistory"`
	}

	wsReattachment struct {
		Hash string `json:"hash"`
	}

	wsUpdate struct {
		Hash     string `json:"hash"`
		ConfTime int64  `json:"ctime"`
	}

	wsNewMile struct {
		Hash      string `json:"hash"`
		Milestone string `json:"milestone"`
		ConfTime  int64  `json:"ctime"`
	}
)

func initRingBuffer() {
	txRingBuffer = ring.New(TX_BUFFER_SIZE)
	txPointerMap = make(map[string]*wsTransaction)
}

func onConnectHandler(s socketio.Conn) error {
	infoMsg := "Monitor client connection established"
	if s != nil {
		infoMsg = fmt.Sprintf("%s (ID: %v)", infoMsg, s.ID())
	}
	log.Info(infoMsg)
	socketioServer.JoinRoom("broadcast", s)
	return nil
}

func onErrorHandler(s socketio.Conn, e error) {
	errorMsg := "Monitor meet error"
	if e != nil {
		errorMsg = fmt.Sprintf("%s: %s", errorMsg, e.Error())
	}
	log.Error(errorMsg)
}

func onDisconnectHandler(s socketio.Conn, msg string) {
	infoMsg := "Monitor client connection closed"
	if s != nil {
		infoMsg = fmt.Sprintf("%s (ID: %v)", infoMsg, s.ID())
	}
	log.Info(fmt.Sprintf("%s: %s", infoMsg, msg))
	socketioServer.LeaveAllRooms(s)
}

func onNewTx(cachedTx *tangle.CachedTransaction) {

	cachedTx.ConsumeTransaction(func(tx *hornet.Transaction) {
		wsTx := &wsTransaction{
			Hash:       tx.Tx.Hash,
			Address:    tx.Tx.Address,
			Value:      tx.Tx.Value,
			Tag:        tx.Tx.Tag,
			Bundle:     tx.Tx.Bundle,
			ReceivedAt: time.Now().Unix() * 1000,
			ConfTime:   1111111111111,
			Milestone:  "f",
			//TrunkTransaction:  tx.Tx.TrunkTransaction,
			//BranchTransaction: tx.Tx.BranchTransaction,
		}

		txRingBufferLock.Lock()

		if _, exists := txPointerMap[tx.Tx.Hash]; exists {
			// Tx already exists => ignore
			txRingBufferLock.Unlock()
			return
		}

		// Delete old element from map
		if txRingBuffer.Value != nil {
			oldTx := txRingBuffer.Value.(*wsTransaction)
			delete(txPointerMap, oldTx.Hash)
		}

		// Set new element in ringbuffer
		txRingBuffer.Value = wsTx
		txRingBuffer = txRingBuffer.Next()

		// Add new element to map
		txPointerMap[wsTx.Hash] = wsTx

		txRingBufferLock.Unlock()

		broadcastLock.Lock()
		socketioServer.BroadcastToRoom("broadcast", "newTX", wsTx)
		broadcastLock.Unlock()
	})
}

func onConfirmedTx(cachedTx *tangle.CachedTransaction, msIndex milestone_index.MilestoneIndex, confTime int64) {

	cachedTx.ConsumeTransaction(func(tx *hornet.Transaction) {
		if tx.Tx.CurrentIndex == 0 {
			// Tail Tx => Check if this is a value Tx
			cachedBndl := tangle.GetBundleOfTailTransactionOrNil(tx.Tx.Hash) // bundle +1
			if cachedBndl != nil {
				if !cachedBndl.GetBundle().IsValueSpam() {
					ledgerChanges := cachedBndl.GetBundle().GetLedgerChanges()
					if len(ledgerChanges) > 0 {
						// Mark all different Txs in all bundles as reattachment
						reattachmentWorkerPool.TrySubmit(tx.Tx.Bundle)
					}
				}
				cachedBndl.Release() // bundle -1
			}
		}

		txRingBufferLock.Lock()
		if wsTx, exists := txPointerMap[tx.Tx.Hash]; exists {
			wsTx.Confirmed = true
			wsTx.ConfTime = confTime * 1000
		}
		txRingBufferLock.Unlock()

		update := wsUpdate{
			Hash:     tx.Tx.Hash,
			ConfTime: confTime * 1000,
		}

		broadcastLock.Lock()
		socketioServer.BroadcastToRoom("broadcast", "update", update)
		broadcastLock.Unlock()
	})
}

func onNewMilestone(cachedBndl *tangle.CachedBundle) {

	cachedTailTx := cachedBndl.GetBundle().GetTail() // tx +1
	confTime := cachedTailTx.GetTransaction().GetTimestamp() * 1000
	cachedTailTx.Release() // tx -1

	cachedTxs := cachedBndl.GetBundle().GetTransactions() // tx +1
	cachedBndl.Release()                                  // bundle -1

	txRingBufferLock.Lock()
	for _, cachedTx := range cachedTxs {
		if wsTx, exists := txPointerMap[cachedTx.GetTransaction().GetHash()]; exists {
			wsTx.Confirmed = true
			wsTx.Milestone = "t"
			wsTx.ConfTime = confTime
		}
	}
	txRingBufferLock.Unlock()

	broadcastLock.Lock()
	for _, cachedTx := range cachedTxs {
		update := wsNewMile{
			Hash:      cachedTx.GetTransaction().GetHash(),
			Milestone: "t",
			ConfTime:  confTime,
		}

		socketioServer.BroadcastToRoom("broadcast", "updateMilestone", update)
	}
	broadcastLock.Unlock()

	cachedTxs.Release() // tx -1
}

func onReattachment(txHash trinary.Hash) {

	txRingBufferLock.Lock()
	if wsTx, exists := txPointerMap[txHash]; exists {
		wsTx.Reattached = true
	}
	txRingBufferLock.Unlock()

	update := wsReattachment{
		Hash: txHash,
	}

	broadcastLock.Lock()
	socketioServer.BroadcastToRoom("broadcast", "updateReattach", update)
	broadcastLock.Unlock()
}

func setupResponse(c *gin.Context) {
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	c.Header("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
}

func handleAPI(c *gin.Context) {
	setupResponse(c)

	amount := 15000
	amountStr := c.Query("amount")
	if amountStr != "" {
		amountParsed, err := strconv.Atoi(amountStr)
		if err == nil {
			amount = amountParsed
		}
	}

	var txs []*wsTransaction

	txRingBufferLock.Lock()

	txPointer := txRingBuffer
	for txCount := 0; txCount < amount; txCount++ {
		txPointer = txPointer.Prev()

		if (txPointer == nil) || (txPointer == txRingBuffer) || (txPointer.Value == nil) {
			break
		}

		txs = append(txs, txPointer.Value.(*wsTransaction))
	}

	txRingBufferLock.Unlock()

	response := &getRecentTransactions{}
	response.TXHistory = txs

	c.JSON(http.StatusOK, response)
}
