package webapi

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/iotaledger/iota.go/address"
	"github.com/mitchellh/mapstructure"
	"github.com/gohornet/hornet/packages/model/tangle"
)

func init() {
	addEndpoint("wereAddressesSpentFrom", wereAddressesSpentFrom, implementedAPIcalls)
}

func wereAddressesSpentFrom(i interface{}, c *gin.Context) {
	sp := &WereAddressesSpentFrom{}
	e := ErrorReturn{}

	err := mapstructure.Decode(i, sp)
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

	if len(sp.Addresses) == 0 {
		e.Error = "No addresses provided"
		c.JSON(http.StatusBadRequest, e)
	}

	spr := &WereAddressesSpentFromReturn{}

	for _, addr := range sp.Addresses {
		if err := address.ValidAddress(addr); err != nil {
			e.Error = fmt.Sprintf("Provided address invalid: %s", addr)
			c.JSON(http.StatusBadRequest, e)
			return
		}

		spent, err := tangle.WasAddressSpentFrom(addr[:81])
		if err != nil {
			e.Error = "Spent addresses db invalid"
			c.JSON(http.StatusInternalServerError, e)
			return
		}

		// State
		spr.States = append(spr.States, spent)
	}

	c.JSON(http.StatusOK, spr)
}
