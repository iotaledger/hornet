package monitor

import (
	"container/ring"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

var (
	txRingBuffer *ring.Ring
	txPointerMap map[trinary.Hash]*wsTransaction

	txRingBufferLock = syncutils.Mutex{}
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
	wsMessage struct {
		Type string      `json:"type"`
		Data interface{} `json:"data"`
	}
)

func initRingBuffer() {
	txRingBuffer = ring.New(txBufferSize)
	txPointerMap = make(map[trinary.Hash]*wsTransaction)
}

func onNewTx(cachedTx *tangle.CachedTransaction) {

	cachedTx.ConsumeTransaction(func(tx *hornet.Transaction, metadata *hornet.TransactionMetadata) {
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

		hub.BroadcastMsg(&wsMessage{Type: "newTX", Data: wsTx})
	})
}

func onConfirmedTx(cachedTx *tangle.CachedTransaction, _ milestone.Index, confTime int64) {

	cachedTx.ConsumeTransaction(func(tx *hornet.Transaction, metadata *hornet.TransactionMetadata) {
		if tx.Tx.CurrentIndex == 0 {
			// Tail Tx => Check if this is a value Tx
			cachedBndl := tangle.GetCachedBundleOrNil(tx.GetTxHash()) // bundle +1
			if cachedBndl != nil {
				if !cachedBndl.GetBundle().IsValueSpam() {
					ledgerChanges := cachedBndl.GetBundle().GetLedgerChanges()
					if len(ledgerChanges) > 0 {
						// Mark all different Txs in all bundles as reattachment
						reattachmentWorkerPool.TrySubmit(tx.Tx.Bundle)
					}
				}
				cachedBndl.Release(true) // bundle -1
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

		hub.BroadcastMsg(&wsMessage{Type: "update", Data: update})
	})
}

func onNewMilestone(cachedBndl *tangle.CachedBundle) {

	cachedTailTx := cachedBndl.GetBundle().GetTail() // tx +1
	confTime := cachedTailTx.GetTransaction().GetTimestamp() * 1000
	cachedTailTx.Release(true) // tx -1

	cachedTxs := cachedBndl.GetBundle().GetTransactions() // tx +1
	cachedBndl.Release(true)                              // bundle -1

	txRingBufferLock.Lock()
	for _, cachedTx := range cachedTxs {
		if wsTx, exists := txPointerMap[cachedTx.GetTransaction().Tx.Hash]; exists {
			wsTx.Confirmed = true
			wsTx.Milestone = "t"
			wsTx.ConfTime = confTime
		}
	}
	txRingBufferLock.Unlock()

	for _, cachedTx := range cachedTxs {
		update := wsNewMile{
			Hash:      cachedTx.GetTransaction().Tx.Hash,
			Milestone: "t",
			ConfTime:  confTime,
		}
		hub.BroadcastMsg(&wsMessage{Type: "updateMilestone", Data: update})
	}

	cachedTxs.Release(true) // tx -1
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

	hub.BroadcastMsg(&wsMessage{Type: "updateReattach", Data: update})
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
