package webapi

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
)

func init() {
	addEndpoint("getLedgerDiff", getLedgerDiff, implementedAPIcalls)
	addEndpoint("getLedgerDiffExt", getLedgerDiffExt, implementedAPIcalls)
}

func getLedgerDiff(i interface{}, c *gin.Context, abortSignal <-chan struct{}) {
	ld := &GetLedgerDiff{}
	e := ErrorReturn{}

	err := mapstructure.Decode(i, ld)
	if err != nil {
		e.Error = "Internal error"
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	smi := tangle.GetSolidMilestoneIndex()
	requestedIndex := milestone_index.MilestoneIndex(ld.MilestoneIndex)
	if requestedIndex > smi {
		e.Error = fmt.Sprintf("Invalid milestone index supplied, lsmi is %d", smi)
		c.JSON(http.StatusBadRequest, e)
		return
	}

	ldr := &GetLedgerDiffReturn{}

	diff, err := tangle.GetLedgerDiffForMilestone(requestedIndex, abortSignal)
	if err != nil {
		e.Error = "Internal error"
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	ldr.Diff = diff
	ldr.MilestoneIndex = ld.MilestoneIndex

	c.JSON(http.StatusOK, ldr)
}

func getLedgerDiffExt(i interface{}, c *gin.Context, abortSignal <-chan struct{}) {
	ld := &GetLedgerDiffExt{}
	e := ErrorReturn{}

	err := mapstructure.Decode(i, ld)
	if err != nil {
		e.Error = "Internal error"
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	smi := tangle.GetSolidMilestoneIndex()
	requestedIndex := milestone_index.MilestoneIndex(ld.MilestoneIndex)
	if requestedIndex > smi {
		e.Error = fmt.Sprintf("Invalid milestone index supplied, lsmi is %d", smi)
		c.JSON(http.StatusBadRequest, e)
		return
	}

	confirmedTxWithValue, confirmedBundlesWithValue, ledgerChanges, err := getMilestoneStateDiff(requestedIndex)
	if err != nil {
		e.Error = errors.Wrapf(err, "Internal error").Error()
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	ldr := &GetLedgerDiffExtReturn{}

	ldr.ConfirmedTxWithValue = confirmedTxWithValue
	ldr.ConfirmedBundlesWithValue = confirmedBundlesWithValue
	ldr.Diff = ledgerChanges
	ldr.MilestoneIndex = ld.MilestoneIndex

	c.JSON(http.StatusOK, ldr)
}

func getMilestoneStateDiff(milestoneIndex milestone_index.MilestoneIndex) (confirmedTxWithValue []*TxHashWithValue, confirmedBundlesWithValue []*BundleWithValue, totalLedgerChanges map[string]int64, err error) {

	reqMilestone, err := tangle.GetMilestone(milestoneIndex)
	if err != nil {
		return nil, nil, nil, errors.New("failed to retrieve ledger milestone bundle")
	}
	if reqMilestone == nil {
		return nil, nil, nil, errors.New("milestone not found")
	}

	txsToConfirm := make(map[string]struct{})
	txsToTraverse := make(map[string]struct{})
	totalLedgerChanges = make(map[string]int64)

	txsToTraverse[reqMilestone.GetTailHash()] = struct{}{}

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

			tx, _ := tangle.GetTransaction(txHash)
			if tx == nil {
				return nil, nil, nil, fmt.Errorf("getMilestoneStateDiff: Transaction not found: %v", txHash)
			}

			confirmed, at := tx.GetConfirmed()
			if confirmed {
				if at != milestoneIndex {
					// ignore all tx that were confirmed by another milestone
					continue
				}
			} else {
				return nil, nil, nil, fmt.Errorf("getMilestoneStateDiff: Transaction not confirmed yet: %v", txHash)
			}

			// Mark the approvees to be traversed
			txsToTraverse[tx.GetTrunk()] = struct{}{}
			txsToTraverse[tx.GetBranch()] = struct{}{}

			if !tx.IsTail() {
				continue
			}

			bundleBucket, err := tangle.GetBundleBucket(tx.Tx.Bundle)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("getMilestoneStateDiff: BundleBucket not found: %v, Error: %v", tx.Tx.Bundle, err)
			}

			bundle := bundleBucket.GetBundleOfTailTransaction(txHash)
			if bundle == nil {
				return nil, nil, nil, fmt.Errorf("getMilestoneStateDiff: Tx: %v, Bundle not found: %v", txHash, tx.Tx.Bundle)
			}

			if !bundle.IsValid() {
				return nil, nil, nil, fmt.Errorf("getMilestoneStateDiff: Tx: %v, Bundle not valid: %v", txHash, tx.Tx.Bundle)
			}

			if !bundle.IsComplete() {
				return nil, nil, nil, fmt.Errorf("getMilestoneStateDiff: Tx: %v, Bundle not complete: %v", txHash, tx.Tx.Bundle)
			}

			ledgerChanges, isValueSpamBundle := bundle.GetLedgerChanges()
			if !isValueSpamBundle {
				var txsWithValue []*TxWithValue

				txs := bundle.GetTransactions()
				for _, tx := range txs {
					if tx.Tx.Value != 0 {
						confirmedTxWithValue = append(confirmedTxWithValue, &TxHashWithValue{TxHash: tx.GetHash(), TailTxHash: bundle.GetTailHash(), BundleHash: tx.Tx.Bundle, Address: tx.Tx.Address, Value: tx.Tx.Value})
					}
					txsWithValue = append(txsWithValue, &TxWithValue{TxHash: tx.GetHash(), Address: tx.Tx.Address, Index: tx.Tx.CurrentIndex, Value: tx.Tx.Value})
				}
				for address, change := range ledgerChanges {
					totalLedgerChanges[address] += change
				}

				confirmedBundlesWithValue = append(confirmedBundlesWithValue, &BundleWithValue{BundleHash: tx.Tx.Bundle, TailTxHash: bundle.GetTailHash(), Txs: txsWithValue, LastIndex: bundle.GetHead().Tx.CurrentIndex})
			}

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
