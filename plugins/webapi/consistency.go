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

	if !tangle.IsNodeSynced() {
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

		tx := tangle.GetCachedTransaction(t) //+1

		// Check if TX is known
		if !tx.Exists() {
			tx.Release() //-1
			info := fmt.Sprint("Transaction not found: ", t)
			c.JSON(http.StatusOK, CheckConsistencyReturn{State: false, Info: info})
			return
		}

		// Check if provided tx is tail
		if !tx.GetTransaction().IsTail() {
			tx.Release() //-1
			info := fmt.Sprint("Invalid transaction, not a tail: ", t)
			c.JSON(http.StatusOK, CheckConsistencyReturn{State: false, Info: info})
			return
		}

		// Check if TX is solid
		if !tx.GetTransaction().IsSolid() {
			tx.Release() //-1
			info := fmt.Sprint("Tails are not solid (missing a referenced tx): ", t)
			c.JSON(http.StatusOK, CheckConsistencyReturn{State: false, Info: info})
			return
		}

		bundleBucket, err := tangle.GetBundleBucket(tx.GetTransaction().Tx.Bundle)
		if err != nil {
			tx.Release() //-1
			e.Error = fmt.Sprint(err)
			c.JSON(http.StatusInternalServerError, e)
			return
		}

		if bundleBucket == nil {
			tx.Release() //-1
			e.Error = "Internal error"
			c.JSON(http.StatusInternalServerError, e)
			return
		}

		// Check bundle validity
		bundle := bundleBucket.GetBundleOfTailTransaction(tx.GetTransaction().GetHash())
		tx.Release() //-1

		if bundle == nil || !bundle.IsValid() {
			info := fmt.Sprint("tails are not consistent (bundle is invalid): ", t)
			c.JSON(http.StatusOK, CheckConsistencyReturn{State: false, Info: info})
			return
		}

		// skip validating the tx if we already approved it
		if _, alreadyApproved := approved[bundle.GetTailHash()]; alreadyApproved {
			continue
		}

		// Check below max depth
		bundleTail := bundle.GetTail() //+1
		IsBelowMaxDepth := tanglePlugin.IsBelowMaxDepth(bundleTail, lowerAllowedSnapshotIndex)
		bundleTail.Release() //-1
		if IsBelowMaxDepth {
			info := fmt.Sprint("tails are not consistent (below max depth): ", t)
			c.JSON(http.StatusOK, CheckConsistencyReturn{State: false, Info: info})
			return
		}

		// Check consistency
		if !tanglePlugin.CheckConsistencyOfConeAndMutateDiff(bundle.GetTailHash(), approved, diff) {
			info := fmt.Sprint("tails are not consistent (would lead to inconsistent ledger state): ", t)
			c.JSON(http.StatusOK, CheckConsistencyReturn{State: false, Info: info})
			return
		}
	}

	c.JSON(http.StatusOK, CheckConsistencyReturn{State: true})
}
