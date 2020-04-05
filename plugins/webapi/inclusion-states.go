package webapi

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"

	"github.com/iotaledger/iota.go/guards"

	"github.com/gohornet/hornet/pkg/model/tangle"
)

func init() {
	addEndpoint("getInclusionStates", getInclusionStates, implementedAPIcalls)
}

func getInclusionStates(i interface{}, c *gin.Context, _ <-chan struct{}) {
	query := &GetInclusionStates{}
	e := ErrorReturn{}

	err := mapstructure.Decode(i, query)
	if err != nil {
		e.Error = "Internal error"
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

	tangle.ReadLockLedger()
	defer tangle.ReadUnlockLedger()

	if !tangle.IsNodeSynced() {
		e.Error = "Node not synced"
		c.JSON(http.StatusBadRequest, e)
		return
	}

	inclusionStates := []bool{}

	for _, tx := range query.Transactions {
		// get tx data
		cachedTx := tangle.GetCachedTransactionOrNil(tx) // tx +1

		if cachedTx == nil {
			// if tx is unknown, return false
			inclusionStates = append(inclusionStates, false)
			continue
		}
		// check if tx is set as confirmed
		confirmed, _ := cachedTx.GetMetadata().GetConfirmed()
		cachedTx.Release(true) // tx -1
		inclusionStates = append(inclusionStates, confirmed)
	}

	c.JSON(http.StatusOK, GetInclusionStatesReturn{States: inclusionStates})
}
