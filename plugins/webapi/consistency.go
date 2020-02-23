package webapi

import (
	"fmt"
	"math"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"

	"github.com/iotaledger/iota.go/guards"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/model/tangle"
	tanglePlugin "github.com/gohornet/hornet/plugins/tangle"
)

func init() {
	addEndpoint("checkConsistency", checkConsistency, implementedAPIcalls)
}

func checkConsistency(i interface{}, c *gin.Context, abortSignal <-chan struct{}) {
	checkCon := &CheckConsistency{}
	e := ErrorReturn{}

	err := mapstructure.Decode(i, checkCon)
	if err != nil {
		e.Error = "Internal error"
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	if len(checkCon.Tails) == 0 {
		e.Error = "No tails provided"
		c.JSON(http.StatusBadRequest, e)
		return
	}

	if !tangle.IsNodeSyncedWithThreshold() {
		e.Error = "Node not synced"
		c.JSON(http.StatusBadRequest, e)
		return
	}

	for _, t := range checkCon.Tails {
		if !guards.IsTransactionHash(t) {
			e.Error = fmt.Sprintf("Invalid reference hash supplied: %s", t)
			c.JSON(http.StatusBadRequest, e)
			return
		}
	}

	// compute the range in which we allow approvers to reference transactions in
	lowerAllowedSnapshotIndex := int(math.Max(float64(int(tangle.GetSolidMilestoneIndex())-maxDepth), float64(0)))

	tangle.ReadLockLedger()
	defer tangle.ReadUnlockLedger()

	diff := map[trinary.Hash]int64{}
	approved := map[trinary.Hash]struct{}{}
	solidEntryPoints := tangle.GetSolidEntryPointsHashes()
	for _, selectEntryPoint := range solidEntryPoints {
		approved[selectEntryPoint] = struct{}{}
	}

	// it is safe to cache the below max depth flag of transactions as long as the same milestone is solid.
	tanglePlugin.BelowDepthMemoizationCache.ResetIfNewerMilestone(tangle.GetSolidMilestoneIndex())

	for _, t := range checkCon.Tails {

		cachedTx := tangle.GetCachedTransactionOrNil(t) // tx +1

		// Check if TX is known
		if cachedTx == nil {
			info := fmt.Sprint("Transaction not found: ", t)
			c.JSON(http.StatusOK, CheckConsistencyReturn{State: false, Info: info})
			return
		}

		// Check if provided tx is tail
		if !cachedTx.GetTransaction().IsTail() {
			cachedTx.Release() // tx -1
			info := fmt.Sprint("Invalid transaction, not a tail: ", t)
			c.JSON(http.StatusOK, CheckConsistencyReturn{State: false, Info: info})
			return
		}

		// Check if TX is solid
		if !cachedTx.GetMetadata().IsSolid() {
			cachedTx.Release() // tx -1
			info := fmt.Sprint("Tails are not solid (missing a referenced tx): ", t)
			c.JSON(http.StatusOK, CheckConsistencyReturn{State: false, Info: info})
			return
		}

		cachedBndl := tangle.GetBundleOfTailTransactionOrNil(cachedTx.GetTransaction().GetHash()) // bundle +1
		cachedTx.Release()                                                                        // tx -1

		if cachedBndl == nil {
			info := fmt.Sprint("tails are not consistent (bundle not found): ", t)
			c.JSON(http.StatusOK, CheckConsistencyReturn{State: false, Info: info})
			return
		}

		if !cachedBndl.GetBundle().IsValid() {
			info := fmt.Sprint("tails are not consistent (bundle is invalid): ", t)
			c.JSON(http.StatusOK, CheckConsistencyReturn{State: false, Info: info})
			cachedBndl.Release() // bundle -1
			return
		}

		// skip validating the tx if we already approved it
		if _, alreadyApproved := approved[cachedBndl.GetBundle().GetTailHash()]; alreadyApproved {
			cachedBndl.Release() // bundle -1
			continue
		}

		// Check below max depth
		if tanglePlugin.IsBelowMaxDepth(cachedBndl.GetBundle().GetTail(), lowerAllowedSnapshotIndex) { // tx pass +1
			info := fmt.Sprint("tails are not consistent (below max depth): ", t)
			c.JSON(http.StatusOK, CheckConsistencyReturn{State: false, Info: info})
			cachedBndl.Release() // bundle -1
			return
		}

		// Check consistency
		if !tanglePlugin.CheckConsistencyOfConeAndMutateDiff(cachedBndl.GetBundle().GetTailHash(), approved, diff) {
			info := fmt.Sprint("tails are not consistent (would lead to inconsistent ledger state): ", t)
			c.JSON(http.StatusOK, CheckConsistencyReturn{State: false, Info: info})
			cachedBndl.Release() // bundle -1
			return
		}
		cachedBndl.Release() // bundle -1
	}

	c.JSON(http.StatusOK, CheckConsistencyReturn{State: true})
}
