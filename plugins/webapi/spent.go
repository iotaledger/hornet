package webapi

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"

	"github.com/iotaledger/iota.go/address"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/plugins/permaspent"
)

func init() {
	addEndpoint("wereAddressesSpentFrom", wereAddressesSpentFrom, implementedAPIcalls)
}

func wereAddressesSpentFrom(i interface{}, c *gin.Context, abortSignal <-chan struct{}) {
	sp := &WereAddressesSpentFrom{}
	e := ErrorReturn{}

	err := mapstructure.Decode(i, sp)
	if err != nil {
		e.Error = "Internal error"
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	if len(sp.Addresses) == 0 {
		e.Error = "No addresses provided"
		c.JSON(http.StatusBadRequest, e)
	}

	spr := &WereAddressesSpentFromReturn{}

	addrs := make(trinary.Hashes, len(sp.Addresses))
	for i, addr := range sp.Addresses {
		if err := address.ValidAddress(addr); err != nil {
			e.Error = fmt.Sprintf("Provided address invalid: %s", addr)
			c.JSON(http.StatusBadRequest, e)
			return
		}
		addrs[i] = addr[:81]

	}
	// State
	states, err := permaspent.WereAddressesSpentFrom(addrs...)
	if err != nil {
		e.Error = fmt.Sprintf("unable to query spent state: %s", err.Error())
		c.JSON(http.StatusInternalServerError, e)
		return
	}
	spr.States = states

	c.JSON(http.StatusOK, spr)
}
