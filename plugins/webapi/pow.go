package webapi

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"

	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/plugins/curl"
	"github.com/gohornet/hornet/plugins/pow"
)

func init() {
	addEndpoint("attachToTangle", attachToTangle, implementedAPIcalls)
}

func attachToTangle(i interface{}, c *gin.Context, _ <-chan struct{}) {
	e := ErrorReturn{}
	query := &AttachToTangle{}

	mwm := config.NodeConfig.GetInt(config.CfgCoordinatorMWM)

	if err := mapstructure.Decode(i, query); err != nil {
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	// mwm is an optional parameter
	if query.MinWeightMagnitude == 0 {
		query.MinWeightMagnitude = mwm
	}

	// Reject wrong MWM
	if query.MinWeightMagnitude != mwm {
		e.Error = fmt.Sprintf("Wrong MinWeightMagnitude. requested: %d, expected: %d", query.MinWeightMagnitude, mwm)
		c.JSON(http.StatusBadRequest, e)
		return
	}

	// Reject empty requests
	if len(query.Trytes) == 0 {
		e.Error = "No trytes given."
		c.JSON(http.StatusBadRequest, e)
		return
	}

	txs, err := transaction.AsTransactionObjects(query.Trytes, nil)
	if err != nil {
		e.Error = err.Error()
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
			e.Error = fmt.Sprintf("Invalid transaction index. Got: %d, expected: %d", txs[i].CurrentIndex, j)
			c.JSON(http.StatusBadRequest, e)
			return
		}
	}

	var prev trinary.Hash
	for i := 0; i < len(txs); i++ {

		switch {
		case i == 0:
			txs[i].TrunkTransaction = query.TrunkTransaction
			txs[i].BranchTransaction = query.BranchTransaction
		default:
			txs[i].TrunkTransaction = prev
			txs[i].BranchTransaction = query.TrunkTransaction
		}

		txs[i].AttachmentTimestamp = time.Now().UnixNano() / int64(time.Millisecond)
		txs[i].AttachmentTimestampLowerBound = consts.LowerBoundAttachmentTimestamp
		txs[i].AttachmentTimestampUpperBound = consts.UpperBoundAttachmentTimestamp

		// Convert tx to trytes
		trytes, err := transaction.TransactionToTrytes(&txs[i])
		if err != nil {
			e.Error = err.Error()
			c.JSON(http.StatusInternalServerError, e)
			return
		}

		// Do the PoW
		ts := time.Now()
		txs[i].Nonce, err = pow.Handler().DoPoW(trytes, query.MinWeightMagnitude)
		if err != nil {
			e.Error = err.Error()
			c.JSON(http.StatusInternalServerError, e)
			return
		}
		log.Debugf("PoW method: \"%s\", MWM: %d, took %v", pow.Handler().GetPoWType(), mwm, time.Since(ts).Truncate(time.Millisecond))

		// Convert tx to trits
		txTrits, err := transaction.TransactionToTrits(&txs[i])
		if err != nil {
			e.Error = err.Error()
			c.JSON(http.StatusInternalServerError, e)
			return
		}

		// Calculate the transaction hash with the batched hasher
		hashTrits, err := curl.Hasher().Hash(txTrits)
		if err != nil {
			e.Error = err.Error()
			c.JSON(http.StatusInternalServerError, e)
			return
		}

		txs[i].Hash = trinary.MustTritsToTrytes(hashTrits)

		prev = txs[i].Hash

		// Check tx
		if !transaction.HasValidNonce(&txs[i], uint64(query.MinWeightMagnitude)) {
			e.Error = err.Error()
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
