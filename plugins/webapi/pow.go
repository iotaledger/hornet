package webapi

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/pow"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/mitchellh/mapstructure"
	"github.com/gohornet/hornet/packages/curl"
	"github.com/iotaledger/hive.go/parameter"
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

func attachToTangle(i interface{}, c *gin.Context) {

	mwm := parameter.NodeConfig.GetInt("protocol.mwm")

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
		hashTrits := curl.CURLP81.Hash(txTrits)
		txs[i].Hash = trinary.MustTritsToTrytes(hashTrits)

		prev = txs[i].Hash

		// Check tx
		if !transaction.HasValidNonce(&txs[i], uint64(aTT.MinWeightMagnitude)) {
			e.Error = fmt.Sprint(err)
			c.JSON(http.StatusInternalServerError, e)
			return
		}
	}

	powedTxTrytes := transaction.MustTransactionsToTrytes(txs)

	c.JSON(http.StatusOK, AttachToTangleReturn{Trytes: powedTxTrytes})
}
