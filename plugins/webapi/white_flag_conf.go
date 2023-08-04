package webapi

import (
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/hornet/pkg/model/tangle"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"
)

func (s *WebAPIServer) rpcGetWhiteFlagConfirmation(c echo.Context) (interface{}, error) {
	request := &GetMigration{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid request, error: %s", err)
	}

	result := &GetWhiteFlagConfirmationResponse{}

	cachedMsBundle := tangle.GetMilestoneOrNil(request.MilestoneIndex)
	if cachedMsBundle == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "milestone not found for wf-confirmation at %d", request.MilestoneIndex)
	}
	defer cachedMsBundle.Release()

	msBundleTrytes, err := cachedBundleTxsToTrytes(cachedMsBundle.GetBundle().GetTransactions())
	if err != nil {
		return nil, errors.WithMessage(echo.ErrNotFound, err.Error())
	}
	result.MilestoneBundle = msBundleTrytes

	conf, err := tangle.GetWhiteFlagConfirmation(request.MilestoneIndex)
	if err != nil {
		return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
	}

	if conf == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "no wf-confirmation stored for milestone %d", request.MilestoneIndex)
	}

	wfConfIncludedBundles := make([][]trinary.Trytes, 0)

	// go through the included bundles and convert them to trytes
	for _, includedTailTx := range conf.Mutations.TailsIncluded {
		cachedBundle := tangle.GetCachedBundleOrNil(includedTailTx)
		if cachedBundle == nil {
			return nil, errors.WithMessagef(echo.ErrNotFound, "bundle not found for included tail transaction %s wf-confirmation at %d", includedTailTx.Trytes(), request.MilestoneIndex)
		}

		bundleTrytes, err := cachedBundleTxsToTrytes(cachedBundle.GetBundle().GetTransactions())
		if err != nil {
			return nil, errors.WithMessage(echo.ErrNotFound, err.Error())
		}

		wfConfIncludedBundles = append(wfConfIncludedBundles, bundleTrytes)
		cachedBundle.Release()
	}

	result.IncludedBundles = wfConfIncludedBundles

	return result, nil
}

func cachedBundleTxsToTrytes(cachedBundleTxs tangle.CachedTransactions) ([]trinary.Trytes, error) {
	defer cachedBundleTxs.Release()
	bundleTrytes := make([]trinary.Trytes, len(cachedBundleTxs))
	for _, cachedBundleTx := range cachedBundleTxs {
		tx := cachedBundleTx.GetTransaction().Tx
		txTrytes, err := transaction.TransactionToTrytes(tx)
		if err != nil {
			return nil, err
		}
		bundleTrytes[tx.CurrentIndex] = txTrytes
	}
	return bundleTrytes, nil
}
