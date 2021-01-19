package webapi

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/mitchellh/mapstructure"
)

func init() {
	addEndpoint("getWhiteFlagConfirmation", getWhiteFlagConfirmation, implementedAPIcalls)
}

// GetWhiteFlagConfirmationResponse defines the response of a getWhiteFlagConfirmation HTTP API call.
type GetWhiteFlagConfirmationResponse struct {
	// The trytes of the milestone bundle.
	MilestoneBundle []trinary.Trytes `json:"milestoneBundle"`
	// The included bundles of the white-flag confirmation in their DFS order.
	IncludedBundles [][]trinary.Trytes `json:"includedBundles"`
}

func getWhiteFlagConfirmation(i interface{}, c *gin.Context, _ <-chan struct{}) {
	e := ErrorReturn{}
	query := &GetMigration{}
	res := &GetWhiteFlagConfirmationResponse{}

	if err := mapstructure.Decode(i, query); err != nil {
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	cachedMsBundle := tangle.GetMilestoneOrNil(query.MilestoneIndex)
	if cachedMsBundle == nil {
		e.Error = fmt.Sprintf("milestone not found for wf-confirmation at %d", query.MilestoneIndex)
		c.JSON(http.StatusNotFound, e)
		return
	}

	msBundleTrytes, err := cachedBundleTxsToTrytes(cachedMsBundle.GetBundle().GetTransactions())
	if err != nil {
		e.Error = err.Error()
		c.JSON(http.StatusNotFound, e)
		cachedMsBundle.Release()
		return
	}
	res.MilestoneBundle = msBundleTrytes

	conf, err := tangle.GetWhiteFlagConfirmation(query.MilestoneIndex)
	if err != nil {
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	if conf == nil {
		e.Error = fmt.Sprintf("no wf-confirmation stored for milestone %d", query.MilestoneIndex)
		c.JSON(http.StatusNotFound, e)
		return
	}

	wfConfIncludedBundles := make([][]trinary.Trytes, 0)

	// go through the included bundles and convert them to trytes
	for _, includedTailTx := range conf.Mutations.TailsIncluded {
		cachedBundle := tangle.GetCachedBundleOrNil(includedTailTx)
		if cachedBundle == nil {
			e.Error = fmt.Sprintf("bundle not found for included tail transaction %s wf-confirmation at %d", includedTailTx.Trytes(), query.MilestoneIndex)
			c.JSON(http.StatusNotFound, e)
			return
		}

		bundleTrytes, err := cachedBundleTxsToTrytes(cachedBundle.GetBundle().GetTransactions())
		if err != nil {
			e.Error = err.Error()
			c.JSON(http.StatusNotFound, e)
			cachedBundle.Release()
			return
		}

		wfConfIncludedBundles = append(wfConfIncludedBundles, bundleTrytes)
		cachedBundle.Release()
	}

	res.IncludedBundles = wfConfIncludedBundles
	c.JSON(http.StatusOK, res)
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
