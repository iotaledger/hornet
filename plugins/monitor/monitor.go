package monitor

import (
	"container/ring"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	socketio "github.com/googollee/go-socket.io"

	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/syncutils"
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
	if s != nil {
		errorMsg = fmt.Sprintf("%s (ID: %v)", errorMsg, s.ID())
	}
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

func onNewTx(tx *hornet.Transaction) {

	txHash := tx.GetHash()

	wsTx := &wsTransaction{
		Hash:       txHash,
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

	if _, exists := txPointerMap[txHash]; exists {
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
}

func onConfirmedTx(tx *hornet.Transaction, msIndex milestone_index.MilestoneIndex, confTime int64) {

	txHash := tx.GetHash()

	if tx.Tx.CurrentIndex == 0 {
		// Tail Tx => Check if there are other bundles (Reattachments)
		bundleBucket, _ := tangle.GetBundleBucket(tx.Tx.Bundle)
		bundles := bundleBucket.Bundles()
		if bundleBucket != nil && (len(bundles) > 1) {
			ledgerChanges, _ := bundles[0].GetLedgerChanges()
			isValue := len(ledgerChanges) > 0
			if isValue {
				// Mark all different Txs in all bundles as reattachment
				for _, bundle := range bundleBucket.Bundles() {
					for _, tx := range bundle.GetTransactions() {
						reattachmentWorkerPool.TrySubmit(tx)
					}
				}
			}
		}
	}

	txRingBufferLock.Lock()
	if wsTx, exists := txPointerMap[txHash]; exists {
		wsTx.Confirmed = true
		wsTx.ConfTime = confTime * 1000
	}
	txRingBufferLock.Unlock()

	update := wsUpdate{
		Hash:     txHash,
		ConfTime: confTime * 1000,
	}

	broadcastLock.Lock()
	socketioServer.BroadcastToRoom("broadcast", "update", update)
	broadcastLock.Unlock()
}

func onNewMilestone(bundle *tangle.Bundle) {

	tailTx := bundle.GetTail()

	confTime := tailTx.GetTimestamp() * 1000

	txRingBufferLock.Lock()
	for _, tx := range bundle.GetTransactions() {
		if wsTx, exists := txPointerMap[tx.GetHash()]; exists {
			wsTx.Confirmed = true
			wsTx.Milestone = "t"
			wsTx.ConfTime = confTime
		}
	}
	txRingBufferLock.Unlock()

	broadcastLock.Lock()
	for _, tx := range bundle.GetTransactions() {
		update := wsNewMile{
			Hash:      tx.GetHash(),
			Milestone: "t",
			ConfTime:  confTime,
		}

		socketioServer.BroadcastToRoom("broadcast", "updateMilestone", update)
	}
	broadcastLock.Unlock()
}

func onReattachment(tx *hornet.Transaction) {

	txHash := tx.GetHash()

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
