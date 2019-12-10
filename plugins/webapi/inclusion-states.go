package webapi

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iotaledger/iota.go/guards"
	"github.com/mitchellh/mapstructure"
	"github.com/gohornet/hornet/packages/model/tangle"
)

func init() {
	addEndpoint("getInclusionStates", getInclusionStates, implementedAPIcalls)
}

func getInclusionStates(i interface{}, c *gin.Context) {
	gis := &GetInclusionStates{}
	e := ErrorReturn{}

	err := mapstructure.Decode(i, gis)
	if err != nil {
		e.Error = "Internal error"
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	if !tangle.IsNodeSynced() {
		e.Error = "Node not synced"
		c.JSON(http.StatusBadRequest, e)
		return
	}

	inclusionStates := []bool{}

	for _, tx := range gis.Transactions {
		if !guards.IsTransactionHash(tx) {
			e.Error = fmt.Sprintf("Invalid reference hash supplied: %s", tx)
			c.JSON(http.StatusBadRequest, e)
			return
		}
	}

	for _, tx := range gis.Transactions {
		// get tx data
		t, err := tangle.GetTransaction(tx)
		if err != nil {
			e.Error = "Internal error"
			c.JSON(http.StatusInternalServerError, e)
			return
		}

		if t != nil {
			// check if tx is set as confirmed
			confirmed, _ := t.GetConfirmed()
			inclusionStates = append(inclusionStates, confirmed)
		} else {
			// if tx is unknown, return false
			inclusionStates = append(inclusionStates, false)
		}
	}

	c.JSON(http.StatusOK, GetInclusionStatesReturn{States: inclusionStates})
}
