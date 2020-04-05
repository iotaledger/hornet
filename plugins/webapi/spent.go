package webapi

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"

	"github.com/iotaledger/iota.go/address"

	"github.com/gohornet/hornet/pkg/model/tangle"
)

func init() {
	addEndpoint("wereAddressesSpentFrom", wereAddressesSpentFrom, implementedAPIcalls)
}

func wereAddressesSpentFrom(i interface{}, c *gin.Context, _ <-chan struct{}) {
	e := ErrorReturn{}
	query := &WereAddressesSpentFrom{}

	if !tangle.GetSnapshotInfo().IsSpentAddressesEnabled() {
		e.Error = "wereAddressesSpentFrom not available in this node"
		c.JSON(http.StatusBadRequest, e)
		return
	}

	err := mapstructure.Decode(i, query)
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

	if len(query.Addresses) == 0 {
		e.Error = "No addresses provided"
		c.JSON(http.StatusBadRequest, e)
	}

	result := WereAddressesSpentFromReturn{}

	for _, addr := range query.Addresses {
		if err := address.ValidAddress(addr); err != nil {
			e.Error = fmt.Sprintf("Provided address invalid: %s", addr)
			c.JSON(http.StatusBadRequest, e)
			return
		}

		// State
		result.States = append(result.States, tangle.WasAddressSpentFrom(addr[:81]))
	}

	c.JSON(http.StatusOK, result)
}
