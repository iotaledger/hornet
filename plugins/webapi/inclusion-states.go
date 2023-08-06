package webapi

import (
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/iota.go/guards"

	"github.com/iotaledger/hornet/pkg/model/hornet"
	"github.com/iotaledger/hornet/pkg/model/milestone"
	"github.com/iotaledger/hornet/pkg/model/tangle"
)

func (s *WebAPIServer) rpcGetInclusionStates(c echo.Context) (interface{}, error) {
	request := &GetInclusionStates{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid request, error: %s", err)
	}

	for _, tx := range request.Transactions {
		if !guards.IsTransactionHash(tx) {
			return nil, errors.WithMessagef(ErrInvalidParameter, "invalid reference hash provided: %s", tx)
		}
	}

	inclusionStates := []bool{}

	for _, tx := range request.Transactions {
		// get tx data
		cachedTxMeta := tangle.GetCachedTxMetadataOrNil(hornet.HashFromHashTrytes(tx)) // meta +1

		if cachedTxMeta == nil {
			// if tx is unknown, return false
			inclusionStates = append(inclusionStates, false)
			continue
		}
		// check if tx is set as confirmed. Avoid passing true for conflicting tx to be backwards compatible
		confirmed := cachedTxMeta.GetMetadata().IsConfirmed() && !cachedTxMeta.GetMetadata().IsConflicting()

		cachedTxMeta.Release(true) // meta -1
		inclusionStates = append(inclusionStates, confirmed)
	}

	return &GetInclusionStatesResponse{
		States: inclusionStates,
	}, nil
}

func (s *WebAPIServer) transactionMetadata(c echo.Context) (interface{}, error) {
	txHash, err := parseTransactionHashParam(c)
	if err != nil {
		return nil, err
	}

	// get tx data
	cachedTxMeta := tangle.GetCachedTxMetadataOrNil(txHash) // meta +1
	if cachedTxMeta == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "transaction not found: %s", txHash.Trytes())
	}
	defer cachedTxMeta.Release(true)

	txMeta := cachedTxMeta.GetMetadata()

	var milestoneIndex milestone.Index
	tangle.ForEachBundle(txMeta.GetBundleHash(), func(bundle *tangle.Bundle) bool {
		if bundle.IsMilestone() {
			milestoneIndex = bundle.GetMilestoneIndex()
			return false
		}

		return true
	})

	var referencedByMilestoneIndex milestone.Index
	var milestoneTimestampReferenced uint64
	confirmed, at := txMeta.GetConfirmed()
	if confirmed {
		referencedByMilestoneIndex = at

		timestamp, err := tangle.GetMilestoneTimestamp(referencedByMilestoneIndex)
		if err == nil {
			milestoneTimestampReferenced = timestamp
		}
	}

	return &transactionMetadataResponse{
		TxHash:                       txHash.Trytes(),
		Solid:                        txMeta.IsSolid(),
		Included:                     confirmed && !txMeta.IsConflicting(), // avoid passing true for conflicting tx to be backwards compatible
		Confirmed:                    confirmed,
		Conflicting:                  txMeta.IsConflicting(),
		ReferencedByMilestoneIndex:   referencedByMilestoneIndex,
		MilestoneTimestampReferenced: milestoneTimestampReferenced,
		MilestoneIndex:               milestoneIndex,
		LedgerIndex:                  tangle.GetSolidMilestoneIndex(),
	}, nil
}
