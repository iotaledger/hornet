package webapi

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"

	"github.com/iotaledger/iota.go/guards"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

func init() {
	addEndpoint("getInclusionStates", getInclusionStates, implementedAPIcalls)
}

func getInclusionStates(i interface{}, c *gin.Context, _ <-chan struct{}) {
	e := ErrorReturn{}
	query := &GetInclusionStates{}

	if err := mapstructure.Decode(i, query); err != nil {
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	for _, tx := range query.Transactions {
		if !guards.IsTransactionHash(tx) {
			e.Error = fmt.Sprintf("Invalid reference hash supplied: %s", tx)
			c.JSON(http.StatusBadRequest, e)
			return
		}
	}

	if !tangle.WaitForNodeSynced(waitForNodeSyncedTimeout) {
		e.Error = ErrNodeNotSync.Error()
		c.JSON(http.StatusBadRequest, e)
		return
	}

	tangle.ReadLockLedger()
	defer tangle.ReadUnlockLedger()

	inclusionStates := []bool{}

	for _, tx := range query.Transactions {
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

	c.JSON(http.StatusOK, GetInclusionStatesReturn{States: inclusionStates})
}
