package webapi

import (
	"fmt"
	"math"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"

	"github.com/iotaledger/iota.go/guards"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/tangle"
	tanglePlugin "github.com/gohornet/hornet/plugins/tangle"
)

func init() {
	addEndpoint("checkConsistency", checkConsistency, implementedAPIcalls)
}

func checkConsistency(i interface{}, c *gin.Context, _ <-chan struct{}) {
	e := ErrorReturn{}
	query := &CheckConsistency{}

	if err := mapstructure.Decode(i, query); err != nil {
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	if len(query.Tails) == 0 {
		e.Error = "No tails provided"
		c.JSON(http.StatusBadRequest, e)
		return
	}

	for _, t := range query.Tails {
		if !guards.IsTransactionHash(t) {
			e.Error = fmt.Sprintf("Invalid reference hash supplied: %s", t)
			c.JSON(http.StatusBadRequest, e)
			return
		}
	}

	tangle.ReadLockLedger()
	defer tangle.ReadUnlockLedger()

	if !tangle.IsNodeSynced() {
		e.Error = ErrNodeNotSync.Error()
		c.JSON(http.StatusBadRequest, e)
		return
	}

	// compute the range in which we allow approvers to reference transactions in
	lowerAllowedSnapshotIndex := int(math.Max(float64(int(tangle.GetSolidMilestoneIndex())-maxDepth), float64(0)))

	diff := map[string]int64{}
	approved := map[string]struct{}{}
	solidEntryPoints := tangle.GetSolidEntryPointsHashes()
	for _, selectEntryPoint := range solidEntryPoints {
		approved[string(selectEntryPoint)] = struct{}{}
	}

	// it is safe to cache the below max depth flag of transactions as long as the same milestone is solid.
	tanglePlugin.BelowDepthMemoizationCache.ResetIfNewerMilestone(tangle.GetSolidMilestoneIndex())

	for _, t := range query.Tails {

		cachedTx := tangle.GetCachedTransactionOrNil(hornet.Hash(trinary.MustTrytesToBytes(t)[:49])) // tx +1

		// Check if TX is known
		if cachedTx == nil {
			info := fmt.Sprint("Transaction not found: ", t)
			c.JSON(http.StatusOK, CheckConsistencyReturn{State: false, Info: info})
			return
		}

		// Check if provided tx is tail
		if !cachedTx.GetTransaction().IsTail() {
			cachedTx.Release(true) // tx -1
			info := fmt.Sprint("Invalid transaction, not a tail: ", t)
			c.JSON(http.StatusOK, CheckConsistencyReturn{State: false, Info: info})
			return
		}

		// Check if TX is solid
		if !cachedTx.GetMetadata().IsSolid() {
			cachedTx.Release(true) // tx -1
			info := fmt.Sprint("Tails are not solid (missing a referenced tx): ", t)
			c.JSON(http.StatusOK, CheckConsistencyReturn{State: false, Info: info})
			return
		}

		cachedBndl := tangle.GetCachedBundleOrNil(cachedTx.GetTransaction().GetTxHash()) // bundle +1
		cachedTx.Release(true)                                                           // tx -1

		if cachedBndl == nil {
			info := fmt.Sprint("tails are not consistent (bundle not found): ", t)
			c.JSON(http.StatusOK, CheckConsistencyReturn{State: false, Info: info})
			return
		}

		if !cachedBndl.GetBundle().IsValid() {
			info := fmt.Sprint("tails are not consistent (bundle is invalid): ", t)
			c.JSON(http.StatusOK, CheckConsistencyReturn{State: false, Info: info})
			cachedBndl.Release(true) // bundle -1
			return
		}

		// skip validating the tx if we already approved it
		if _, alreadyApproved := approved[string(cachedBndl.GetBundle().GetTailHash())]; alreadyApproved {
			cachedBndl.Release(true) // bundle -1
			continue
		}

		// Check below max depth
		if tanglePlugin.IsBelowMaxDepth(cachedBndl.GetBundle().GetTail(), lowerAllowedSnapshotIndex, true) { // tx pass +1
			info := fmt.Sprint("tails are not consistent (below max depth): ", t)
			c.JSON(http.StatusOK, CheckConsistencyReturn{State: false, Info: info})
			cachedBndl.Release(true) // bundle -1
			return
		}

		// Check consistency
		if !tanglePlugin.CheckConsistencyOfConeAndMutateDiff(cachedBndl.GetBundle().GetTailHash(), approved, diff, true) {
			info := fmt.Sprint("tails are not consistent (would lead to inconsistent ledger state): ", t)
			c.JSON(http.StatusOK, CheckConsistencyReturn{State: false, Info: info})
			cachedBndl.Release(true) // bundle -1
			return
		}
		cachedBndl.Release(true) // bundle -1
	}

	c.JSON(http.StatusOK, CheckConsistencyReturn{State: true})
}
