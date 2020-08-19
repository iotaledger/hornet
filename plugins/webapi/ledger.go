package webapi

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/hornet"
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

	diffTrytes := make(map[trinary.Trytes]int64)
	for address, balance := range diff {
		diffTrytes[hornet.Hash(address).Trytes()] = balance
	}

	c.JSON(http.StatusOK, GetLedgerDiffReturn{Diff: diffTrytes, MilestoneIndex: query.MilestoneIndex})
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

	ledgerChangesTrytes := make(map[trinary.Trytes]int64)
	for address, balance := range ledgerChanges {
		ledgerChangesTrytes[hornet.Hash(address).Trytes()] = balance
	}

	result := GetLedgerDiffExtReturn{}
	result.ConfirmedTxWithValue = confirmedTxWithValue
	result.ConfirmedBundlesWithValue = confirmedBundlesWithValue
	result.Diff = ledgerChangesTrytes
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

	txsToTraverse[string(cachedReqMs.GetBundle().GetTailHash())] = struct{}{}

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

			if tangle.SolidEntryPointsContain(hornet.Hash(txHash)) {
				// Ignore solid entry points (snapshot milestone included)
				continue
			}

			cachedTxMeta := tangle.GetCachedTxMetadataOrNil(hornet.Hash(txHash)) // meta +1
			if cachedTxMeta == nil {
				return nil, nil, nil, fmt.Errorf("getMilestoneStateDiff: Transaction not found: %v", hornet.Hash(txHash).Trytes())
			}

			confirmed, at := cachedTxMeta.GetMetadata().GetConfirmed()
			if confirmed {
				if at != milestoneIndex {
					// ignore all tx that were confirmed by another milestone
					cachedTxMeta.Release(true) // meta -1
					continue
				}
			} else {
				cachedTxMeta.Release(true) // meta -1
				return nil, nil, nil, fmt.Errorf("getMilestoneStateDiff: Transaction not confirmed yet: %v", hornet.Hash(txHash).Trytes())
			}

			// Mark the approvees to be traversed
			txsToTraverse[string(cachedTxMeta.GetMetadata().GetTrunkHash())] = struct{}{}
			txsToTraverse[string(cachedTxMeta.GetMetadata().GetBranchHash())] = struct{}{}

			if !cachedTxMeta.GetMetadata().IsTail() {
				cachedTxMeta.Release(true) // meta -1
				continue
			}

			cachedBndl := tangle.GetCachedBundleOrNil(hornet.Hash(txHash)) // bundle +1
			if cachedBndl == nil {
				txBundle := cachedTxMeta.GetMetadata().GetBundleHash()
				cachedTxMeta.Release(true) // meta -1
				return nil, nil, nil, fmt.Errorf("getMilestoneStateDiff: Tx: %v, Bundle not found: %v", hornet.Hash(txHash).Trytes(), txBundle.Trytes())
			}

			if !cachedBndl.GetBundle().IsValid() {
				txBundle := cachedTxMeta.GetMetadata().GetBundleHash()
				cachedTxMeta.Release(true) // meta -1
				cachedBndl.Release(true)   // bundle -1
				return nil, nil, nil, fmt.Errorf("getMilestoneStateDiff: Tx: %v, Bundle not valid: %v", hornet.Hash(txHash).Trytes(), txBundle.Trytes())
			}

			if !cachedBndl.GetBundle().IsValueSpam() {
				ledgerChanges := cachedBndl.GetBundle().GetLedgerChanges()

				var txsWithValue []*TxWithValue

				cachedTxs := cachedBndl.GetBundle().GetTransactions() // tx +1
				for _, cachedTx := range cachedTxs {
					// hornetTx is being retained during the loop, so safe to use the pointer here
					hornetTx := cachedTx.GetTransaction()
					if hornetTx.Tx.Value != 0 {
						confirmedTxWithValue = append(confirmedTxWithValue, &TxHashWithValue{TxHash: hornetTx.Tx.Hash, TailTxHash: cachedBndl.GetBundle().GetTailHash().Trytes(), BundleHash: hornetTx.Tx.Bundle, Address: hornetTx.Tx.Address, Value: hornetTx.Tx.Value})
					}
					txsWithValue = append(txsWithValue, &TxWithValue{TxHash: hornetTx.Tx.Hash, Address: hornetTx.Tx.Address, Index: hornetTx.Tx.CurrentIndex, Value: hornetTx.Tx.Value})
				}
				cachedTxs.Release(true) // tx -1
				for address, change := range ledgerChanges {
					totalLedgerChanges[address] += change
				}

				cachedBundleHeadTx := cachedBndl.GetBundle().GetHead() // tx +1
				confirmedBundlesWithValue = append(confirmedBundlesWithValue, &BundleWithValue{BundleHash: cachedTxMeta.GetMetadata().GetBundleHash().Trytes(), TailTxHash: cachedBndl.GetBundle().GetTailHash().Trytes(), Txs: txsWithValue, LastIndex: cachedBundleHeadTx.GetTransaction().Tx.CurrentIndex})
				cachedBundleHeadTx.Release(true) // tx -1
			}
			cachedTxMeta.Release(true) // meta -1
			cachedBndl.Release(true)   // bundle -1

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

	balancesTrytes := make(map[trinary.Trytes]uint64)
	for address, balance := range balances {
		balancesTrytes[hornet.Hash(address).Trytes()] = balance
	}

	c.JSON(http.StatusOK, GetLedgerStateReturn{Balances: balancesTrytes, MilestoneIndex: index})
}
