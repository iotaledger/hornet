package webapi

import (
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/iota.go/guards"

	"github.com/iotaledger/hornet/pkg/config"
	"github.com/iotaledger/hornet/pkg/dag"
	"github.com/iotaledger/hornet/pkg/model/hornet"
	"github.com/iotaledger/hornet/pkg/model/milestone"
	"github.com/iotaledger/hornet/pkg/model/tangle"
	"github.com/iotaledger/hornet/pkg/tipselect"
	"github.com/iotaledger/hornet/plugins/urts"
)

func (s *WebAPIServer) rpcGetTipInfo(c echo.Context) (interface{}, error) {
	// do not reply if URTS is disabled
	if node.IsSkipped(urts.PLUGIN) {
		return nil, errors.WithMessage(echo.ErrServiceUnavailable, "tipselection plugin disabled in this node")
	}

	if !tangle.IsNodeSyncedWithThreshold() {
		return nil, errors.WithMessage(echo.ErrServiceUnavailable, "node is not synced")
	}

	request := &GetTipInfo{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid request, error: %s", err)
	}

	if !guards.IsTransactionHash(request.TailTransaction) {
		return nil, errors.WithMessage(echo.ErrBadRequest, "invalid tail hash supplied")
	}

	cachedTxMeta := tangle.GetCachedTxMetadataOrNil(hornet.HashFromHashTrytes(request.TailTransaction)) // meta +1
	if cachedTxMeta == nil {
		return nil, errors.WithMessage(echo.ErrBadRequest, "unknown tail transaction")
	}
	defer cachedTxMeta.Release(true)

	if !cachedTxMeta.GetMetadata().IsTail() {
		return nil, errors.WithMessage(echo.ErrBadRequest, "transaction is not a tail")
	}

	if !cachedTxMeta.GetMetadata().IsSolid() {
		return nil, errors.WithMessage(echo.ErrBadRequest, "transaction is not solid")
	}

	conflicting := cachedTxMeta.GetMetadata().IsConflicting()

	// check if tx is set as confirmed. Avoid passing true for conflicting tx to be backwards compatible
	confirmed := cachedTxMeta.GetMetadata().IsConfirmed() && !conflicting

	if confirmed || conflicting {
		return &GetTipInfoResponse{
			Confirmed:      confirmed,
			Conflicting:    conflicting,
			ShouldPromote:  false,
			ShouldReattach: false,
		}, nil
	}

	lsmi := tangle.GetSolidMilestoneIndex()
	ytrsi, ortsi := dag.GetTransactionRootSnapshotIndexes(cachedTxMeta.Retain(), lsmi)

	// if the OTRSI to LSMI delta is over BelowMaxDepth/below-max-depth, then the tip is lazy and should be reattached
	if (lsmi - ortsi) > milestone.Index(config.NodeConfig.GetInt(config.CfgTipSelBelowMaxDepth)) {
		return &GetTipInfoResponse{
			Confirmed:      false,
			Conflicting:    false,
			ShouldPromote:  false,
			ShouldReattach: true,
		}, nil
	}

	// if the LSMI to YTRSI delta is over MaxDeltaTxYoungestRootSnapshotIndexToLSMI, then the tip is lazy and should be promoted
	if (lsmi - ytrsi) > milestone.Index(config.NodeConfig.GetInt(config.CfgTipSelMaxDeltaTxYoungestRootSnapshotIndexToLSMI)) {
		return &GetTipInfoResponse{
			Confirmed:      false,
			Conflicting:    false,
			ShouldPromote:  true,
			ShouldReattach: false,
		}, nil
	}

	// if the OTRSI to LSMI delta is over MaxDeltaTxOldestRootSnapshotIndexToLSMI, the tip is semi-lazy and should be promoted
	if (lsmi - ortsi) > milestone.Index(config.NodeConfig.GetInt(config.CfgTipSelMaxDeltaTxOldestRootSnapshotIndexToLSMI)) {
		return &GetTipInfoResponse{
			Confirmed:      false,
			Conflicting:    false,
			ShouldPromote:  true,
			ShouldReattach: false,
		}, nil
	}

	// tip is non-lazy, no need to promote or reattach
	return &GetTipInfoResponse{
		Confirmed:      false,
		Conflicting:    false,
		ShouldPromote:  false,
		ShouldReattach: false,
	}, nil
}

func (s *WebAPIServer) rpcGetTransactionsToApprove(c echo.Context) (interface{}, error) {
	// do not reply if URTS is disabled
	if node.IsSkipped(urts.PLUGIN) {
		return nil, errors.WithMessage(echo.ErrServiceUnavailable, "tipselection plugin disabled in this node")
	}

	request := &GetTransactionsToApprove{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid request, error: %s", err)
	}

	tips, err := urts.TipSelector.SelectNonLazyTips()
	if err != nil {
		if err == tangle.ErrNodeNotSynced || err == tipselect.ErrNoTipsAvailable {
			return nil, errors.WithMessage(echo.ErrServiceUnavailable, err.Error())
		}

		return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
	}

	if len(request.Reference) > 0 {
		if !guards.IsTransactionHash(request.Reference) {
			return nil, errors.WithMessage(echo.ErrBadRequest, "invalid reference hash supplied")
		}
		return &GetTransactionsToApproveResponse{TrunkTransaction: tips[0].Trytes(), BranchTransaction: request.Reference}, nil
	}

	return &GetTransactionsToApproveResponse{TrunkTransaction: tips[0].Trytes(), BranchTransaction: tips[1].Trytes()}, nil
}

func (s *WebAPIServer) rpcGetSpammerTips(c echo.Context) (interface{}, error) {
	// do not reply if URTS is disabled
	if node.IsSkipped(urts.PLUGIN) {
		return nil, errors.WithMessage(echo.ErrServiceUnavailable, "tipselection plugin disabled in this node")
	}

	_, tips, err := urts.TipSelector.SelectSpammerTips()
	if err != nil {
		if err == tangle.ErrNodeNotSynced || err == tipselect.ErrNoTipsAvailable {
			return nil, errors.WithMessage(echo.ErrServiceUnavailable, err.Error())
		}

		return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
	}

	return &GetTransactionsToApproveResponse{TrunkTransaction: tips[0].Trytes(), BranchTransaction: tips[1].Trytes()}, nil
}
