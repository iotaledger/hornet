package webapi

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

func init() {
	addEndpoint("getLedgerDiff", getLedgerDiff, implementedAPIcalls)
	addEndpoint("getLedgerDiffExt", getLedgerDiffExt, implementedAPIcalls)
	addEndpoint("getLedgerState", getLedgerState, implementedAPIcalls)
}

func getLedgerDiff(i interface{}, c *gin.Context, abortSignal <-chan struct{}) {
	e := ErrorReturn{}
	query := &GetLedgerDiff{}

	if err := mapstructure.Decode(i, query); err != nil {
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	smi := tangle.GetSolidMilestoneIndex()
	requestedIndex := milestone.Index(query.MilestoneIndex)
	if requestedIndex > smi {
		e.Error = fmt.Sprintf("Invalid milestone index supplied, lsmi is %d", smi)
		c.JSON(http.StatusBadRequest, e)
		return
	}

	diff, err := tangle.GetLedgerDiffForMilestone(requestedIndex, abortSignal)
	if err != nil {
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	c.JSON(http.StatusOK, GetLedgerDiffReturn{Diff: diff, MilestoneIndex: query.MilestoneIndex})
}

func getLedgerDiffExt(i interface{}, c *gin.Context, _ <-chan struct{}) {
	e := ErrorReturn{}
	query := &GetLedgerDiffExt{}

	if err := mapstructure.Decode(i, query); err != nil {
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	smi := tangle.GetSolidMilestoneIndex()
	requestedIndex := milestone.Index(query.MilestoneIndex)
	if requestedIndex > smi {
		e.Error = fmt.Sprintf("Invalid milestone index supplied, lsmi is %d", smi)
		c.JSON(http.StatusBadRequest, e)
		return
	}

	confirmedTxWithValue, confirmedBundlesWithValue, ledgerChanges, err := getMilestoneStateDiff(requestedIndex)
	if err != nil {
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	result := GetLedgerDiffExtReturn{}

	result.ConfirmedTxWithValue = confirmedTxWithValue
	result.ConfirmedBundlesWithValue = confirmedBundlesWithValue
	result.Diff = ledgerChanges
	result.MilestoneIndex = query.MilestoneIndex

	c.JSON(http.StatusOK, result)
}

func getMilestoneStateDiff(milestoneIndex milestone.Index) (confirmedTxWithValue []*TxHashWithValue, confirmedBundlesWithValue []*BundleWithValue, totalLedgerChanges map[string]int64, err error) {

	cachedReqMs := tangle.GetMilestoneOrNil(milestoneIndex) // bundle +1
	if cachedReqMs == nil {
		return nil, nil, nil, errors.New("milestone not found")
	}

	txsToConfirm := make(map[string]struct{})
	txsToTraverse := make(map[string]struct{})
	totalLedgerChanges = make(map[string]int64)

	txsToTraverse[cachedReqMs.GetBundle().GetTailHash()] = struct{}{}

	cachedReqMs.Release(true) // bundle -1

	// Collect all tx to check by traversing the tangle
	// Loop as long as new transactions are added in every loop cycle
	for len(txsToTraverse) != 0 {

		for txHash := range txsToTraverse {
			delete(txsToTraverse, txHash)

			if _, checked := txsToConfirm[txHash]; checked {
				// Tx was already checked => ignore
				continue
			}

			if tangle.SolidEntryPointsContain(txHash) {
				// Ignore solid entry points (snapshot milestone included)
				continue
			}

			cachedTx := tangle.GetCachedTransactionOrNil(txHash) // tx +1
			if cachedTx == nil {
				return nil, nil, nil, fmt.Errorf("getMilestoneStateDiff: Transaction not found: %v", txHash)
			}

			confirmed, at := cachedTx.GetMetadata().GetConfirmed()
			if confirmed {
				if at != milestoneIndex {
					// ignore all tx that were confirmed by another milestone
					cachedTx.Release(true) // tx -1
					continue
				}
			} else {
				cachedTx.Release(true) // tx -1
				return nil, nil, nil, fmt.Errorf("getMilestoneStateDiff: Transaction not confirmed yet: %v", txHash)
			}

			// Mark the approvees to be traversed
			txsToTraverse[cachedTx.GetTransaction().GetTrunk()] = struct{}{}
			txsToTraverse[cachedTx.GetTransaction().GetBranch()] = struct{}{}

			if !cachedTx.GetTransaction().IsTail() {
				cachedTx.Release(true) // tx -1
				continue
			}

			txBundle := cachedTx.GetTransaction().Tx.Bundle

			cachedBndl := tangle.GetCachedBundleOrNil(txHash) // bundle +1
			if cachedBndl == nil {
				cachedTx.Release(true) // tx -1
				return nil, nil, nil, fmt.Errorf("getMilestoneStateDiff: Tx: %v, Bundle not found: %v", txHash, txBundle)
			}

			if !cachedBndl.GetBundle().IsValid() {
				cachedTx.Release(true)   // tx -1
				cachedBndl.Release(true) // bundle -1
				return nil, nil, nil, fmt.Errorf("getMilestoneStateDiff: Tx: %v, Bundle not valid: %v", txHash, txBundle)
			}

			if !cachedBndl.GetBundle().IsValueSpam() {
				ledgerChanges := cachedBndl.GetBundle().GetLedgerChanges()

				var txsWithValue []*TxWithValue

				cachedTxs := cachedBndl.GetBundle().GetTransactions() // tx +1
				for _, cachedTx := range cachedTxs {
					// hornetTx is being retained during the loop, so safe to use the pointer here
					hornetTx := cachedTx.GetTransaction()
					if hornetTx.Tx.Value != 0 {
						confirmedTxWithValue = append(confirmedTxWithValue, &TxHashWithValue{TxHash: hornetTx.GetHash(), TailTxHash: cachedBndl.GetBundle().GetTailHash(), BundleHash: hornetTx.Tx.Bundle, Address: hornetTx.Tx.Address, Value: hornetTx.Tx.Value})
					}
					txsWithValue = append(txsWithValue, &TxWithValue{TxHash: hornetTx.GetHash(), Address: hornetTx.Tx.Address, Index: hornetTx.Tx.CurrentIndex, Value: hornetTx.Tx.Value})
				}
				cachedTxs.Release(true) // tx -1
				for address, change := range ledgerChanges {
					totalLedgerChanges[address] += change
				}

				cachedBundleHeadTx := cachedBndl.GetBundle().GetHead() // tx +1
				confirmedBundlesWithValue = append(confirmedBundlesWithValue, &BundleWithValue{BundleHash: cachedTx.GetTransaction().Tx.Bundle, TailTxHash: cachedBndl.GetBundle().GetTailHash(), Txs: txsWithValue, LastIndex: cachedBundleHeadTx.GetTransaction().Tx.CurrentIndex})
				cachedBundleHeadTx.Release(true) // tx -1
			}
			cachedTx.Release(true)   // tx -1
			cachedBndl.Release(true) // bundle -1

			// we only add the tail transaction to the txsToConfirm set, in order to not
			// accidentally skip cones, in case the other transactions (non-tail) of the bundle do not
			// reference the same trunk transaction (as seen from the PoV of the bundle).
			// if we wouldn't do it like this, we have a high chance of computing an
			// inconsistent ledger state.
			txsToConfirm[txHash] = struct{}{}
		}
	}

	return confirmedTxWithValue, confirmedBundlesWithValue, totalLedgerChanges, nil
}

func getLedgerState(i interface{}, c *gin.Context, abortSignal <-chan struct{}) {
	e := ErrorReturn{}
	query := &GetLedgerState{}

	if err := mapstructure.Decode(i, query); err != nil {
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	balances, index, err := tangle.GetLedgerStateForMilestone(query.TargetIndex, abortSignal)
	if err != nil {
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	c.JSON(http.StatusOK, GetLedgerStateReturn{Balances: balances, MilestoneIndex: index})
}
