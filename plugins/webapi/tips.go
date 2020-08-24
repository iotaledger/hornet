package webapi

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/iota.go/guards"
	"github.com/mitchellh/mapstructure"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/plugins/urts"
)

func init() {
	addEndpoint("getTipInfo", getTipInfo, implementedAPIcalls)
	addEndpoint("getTransactionsToApprove", getTransactionsToApprove, implementedAPIcalls)
	addEndpoint("getSpammerTips", getSpammerTips, implementedAPIcalls)
}

func getTipInfo(i interface{}, c *gin.Context, _ <-chan struct{}) {
	e := ErrorReturn{}

	// do not reply if URTS is disabled
	if node.IsSkipped(urts.PLUGIN) {
		e.Error = "tipselection plugin disabled in this node"
		c.JSON(http.StatusServiceUnavailable, e)
		return
	}

	if !tangle.IsNodeSyncedWithThreshold() {
		e.Error = "node is not synced"
		c.JSON(http.StatusBadRequest, e)
		return
	}

	query := &GetTipInfo{}

	if err := mapstructure.Decode(i, query); err != nil {
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	if !guards.IsTransactionHash(query.TailTransaction) {
		e.Error = "invalid tail hash supplied"
		c.JSON(http.StatusBadRequest, e)
		return
	}

	cachedTxMeta := tangle.GetCachedTxMetadataOrNil(hornet.HashFromHashTrytes(query.TailTransaction)) // meta +1
	if cachedTxMeta == nil {
		e.Error = "unknown tail transaction"
		c.JSON(http.StatusBadRequest, e)
		return
	}
	defer cachedTxMeta.Release(true)

	if !cachedTxMeta.GetMetadata().IsTail() {
		e.Error = "transaction is not a tail"
		c.JSON(http.StatusBadRequest, e)
		return
	}

	if !cachedTxMeta.GetMetadata().IsSolid() {
		e.Error = "transaction is not solid"
		c.JSON(http.StatusBadRequest, e)
		return
	}

	conflicting := cachedTxMeta.GetMetadata().IsConflicting()

	// check if tx is set as confirmed. Avoid passing true for conflicting tx to be backwards compatible
	confirmed := cachedTxMeta.GetMetadata().IsConfirmed() && !conflicting

	if confirmed || conflicting {
		c.JSON(http.StatusOK, GetTipInfoReturn{
			Confirmed:      confirmed,
			Conflicting:    conflicting,
			ShouldPromote:  false,
			ShouldReattach: false,
		})
		return
	}

	lsmi := tangle.GetSolidMilestoneIndex()
	ytrsi, ortsi := dag.GetTransactionRootSnapshotIndexes(cachedTxMeta.Retain(), lsmi)

	// if the OTRSI to LSMI delta is over BelowMaxDepth/below-max-depth, then the tip is lazy and should be reattached
	if (lsmi - ortsi) > milestone.Index(config.NodeConfig.GetInt(config.CfgTipSelBelowMaxDepth)) {
		c.JSON(http.StatusOK, GetTipInfoReturn{
			Confirmed:      false,
			Conflicting:    false,
			ShouldPromote:  false,
			ShouldReattach: true,
		})
		return
	}

	// if the LSMI to YTRSI delta is over MaxDeltaTxYoungestRootSnapshotIndexToLSMI, then the tip is lazy and should be promoted
	if (lsmi - ytrsi) > milestone.Index(config.NodeConfig.GetInt(config.CfgTipSelMaxDeltaTxYoungestRootSnapshotIndexToLSMI)) {
		c.JSON(http.StatusOK, GetTipInfoReturn{
			Confirmed:      false,
			Conflicting:    false,
			ShouldPromote:  true,
			ShouldReattach: false,
		})
		return
	}

	// if the OTRSI to LSMI delta is over MaxDeltaTxOldestRootSnapshotIndexToLSMI, the tip is semi-lazy and should be promoted
	if (lsmi - ortsi) > milestone.Index(config.NodeConfig.GetInt(config.CfgTipSelMaxDeltaTxOldestRootSnapshotIndexToLSMI)) {
		c.JSON(http.StatusOK, GetTipInfoReturn{
			Confirmed:      false,
			Conflicting:    false,
			ShouldPromote:  true,
			ShouldReattach: false,
		})
		return
	}

	// tip is non-lazy, no need to promote or reattach
	c.JSON(http.StatusOK, GetTipInfoReturn{
		Confirmed:      false,
		Conflicting:    false,
		ShouldPromote:  false,
		ShouldReattach: false,
	})
}

func getTransactionsToApprove(i interface{}, c *gin.Context, _ <-chan struct{}) {
	e := ErrorReturn{}

	// do not reply if URTS is disabled
	if node.IsSkipped(urts.PLUGIN) {
		e.Error = "tipselection plugin disabled in this node"
		c.JSON(http.StatusServiceUnavailable, e)
		return
	}

	query := &GetTransactionsToApprove{}

	if err := mapstructure.Decode(i, query); err != nil {
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	tips, err := urts.TipSelector.SelectNonLazyTips()
	if err != nil {
		if err == tangle.ErrNodeNotSynced || err == tipselect.ErrNoTipsAvailable {
			e.Error = err.Error()
			c.JSON(http.StatusServiceUnavailable, e)
			return
		}
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	if len(query.Reference) > 0 {
		if !guards.IsTransactionHash(query.Reference) {
			e.Error = "invalid reference hash supplied"
			c.JSON(http.StatusBadRequest, e)
			return
		}
		c.JSON(http.StatusOK, GetTransactionsToApproveReturn{TrunkTransaction: tips[0].Trytes(), BranchTransaction: query.Reference})
		return
	}

	c.JSON(http.StatusOK, GetTransactionsToApproveReturn{TrunkTransaction: tips[0].Trytes(), BranchTransaction: tips[1].Trytes()})
}

func getSpammerTips(i interface{}, c *gin.Context, _ <-chan struct{}) {
	e := ErrorReturn{}

	// do not reply if URTS is disabled
	if node.IsSkipped(urts.PLUGIN) {
		e.Error = "tipselection plugin disabled in this node"
		c.JSON(http.StatusServiceUnavailable, e)
		return
	}

	_, tips, err := urts.TipSelector.SelectSpammerTips()
	if err != nil {
		if err == tangle.ErrNodeNotSynced || err == tipselect.ErrNoTipsAvailable {
			e.Error = err.Error()
			c.JSON(http.StatusServiceUnavailable, e)
			return
		}
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	c.JSON(http.StatusOK, GetTransactionsToApproveReturn{TrunkTransaction: tips[0].Trytes(), BranchTransaction: tips[1].Trytes()})
}
