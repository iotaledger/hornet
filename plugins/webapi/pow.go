package webapi

import (
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"

	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/pow"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/batchhasher"

	"github.com/gohornet/hornet/packages/config"
)

func init() {
	addEndpoint("attachToTangle", attachToTangle, implementedAPIcalls)
}

var (
	powSet  = false
	powFunc pow.ProofOfWorkFunc
	powType string

	powLock = &sync.Mutex{}
)

func attachToTangle(i interface{}, c *gin.Context, abortSignal <-chan struct{}) {

	mwm := config.NodeConfig.GetInt(config.CfgProtocolMWM)

	aTT := &AttachToTangle{}
	e := ErrorReturn{}

	err := mapstructure.Decode(i, aTT)
	if err != nil {
		e.Error = "Internal error"
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	// Reject unnecessarily high MWM
	if aTT.MinWeightMagnitude > mwm {
		e.Error = fmt.Sprintf("MWM too high. MWM: %v, Max allowed: %v", aTT.MinWeightMagnitude, mwm)
		c.JSON(http.StatusBadRequest, e)
		return
	}

	// Set the fastest PoW method
	if !powSet {
		powType, powFunc = pow.GetFastestProofOfWorkImpl()
		powSet = true
		log.Infof("PoW method: \"%v\"", powType)
	}

	txs, err := transaction.AsTransactionObjects(aTT.Trytes, nil)
	if err != nil {
		e.Error = fmt.Sprint(err)
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	// Reject bundles with invalid tx amount
	if uint64(len(txs)) != txs[0].LastIndex+1 {
		e.Error = fmt.Sprintf("Invalid bundle length. Received txs: %v, Bundle requires: %v", len(txs), txs[0].LastIndex+1)
		c.JSON(http.StatusBadRequest, e)
		return
	}

	// Sort transactions (highest to lowest index)
	sort.Slice(txs, func(i, j int) bool {
		return txs[i].CurrentIndex > txs[j].CurrentIndex
	})

	// Check transaction indexes
	for i, j := uint64(0), uint64(len(txs)-1); j > 0; i, j = i+1, j-1 {
		if txs[i].CurrentIndex != j {
			e.Error = fmt.Sprintf("Invalid transaction index.")
			c.JSON(http.StatusBadRequest, e)
			return
		}
	}

	var prev trinary.Hash
	for i := 0; i < len(txs); i++ {

		switch {
		case i == 0:
			txs[i].TrunkTransaction = aTT.TrunkTransaction
			txs[i].BranchTransaction = aTT.BranchTransaction
		default:
			txs[i].TrunkTransaction = prev
			txs[i].BranchTransaction = aTT.TrunkTransaction
		}

		txs[i].AttachmentTimestamp = time.Now().UnixNano() / int64(time.Millisecond)
		txs[i].AttachmentTimestampLowerBound = consts.LowerBoundAttachmentTimestamp
		txs[i].AttachmentTimestampUpperBound = consts.UpperBoundAttachmentTimestamp

		// Convert tx to trytes
		trytes, err := transaction.TransactionToTrytes(&txs[i])
		if err != nil {
			e.Error = fmt.Sprint(err)
			c.JSON(http.StatusInternalServerError, e)
			return
		}

		// Do the PoW
		txs[i].Nonce, err = powFunc(trytes, aTT.MinWeightMagnitude)
		if err != nil {
			e.Error = fmt.Sprint(err)
			c.JSON(http.StatusInternalServerError, e)
			return
		}

		// Convert tx to trits
		txTrits, err := transaction.TransactionToTrits(&txs[i])
		if err != nil {
			e.Error = fmt.Sprint(err)
			c.JSON(http.StatusInternalServerError, e)
			return
		}

		// Calculate the transaction hash with the batched hasher
		hashTrits := batchhasher.CURLP81.Hash(txTrits)
		txs[i].Hash = trinary.MustTritsToTrytes(hashTrits)

		prev = txs[i].Hash

		// Check tx
		if !transaction.HasValidNonce(&txs[i], uint64(aTT.MinWeightMagnitude)) {
			e.Error = fmt.Sprint(err)
			c.JSON(http.StatusInternalServerError, e)
			return
		}
	}

	// Reverse the transactions the same way IRI does (for whatever reason)
	for i, j := 0, len(txs)-1; i < j; i, j = i+1, j-1 {
		txs[i], txs[j] = txs[j], txs[i]
	}

	powedTxTrytes := transaction.MustTransactionsToTrytes(txs)

	c.JSON(http.StatusOK, AttachToTangleReturn{Trytes: powedTxTrytes})
}
