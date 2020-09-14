package webapi

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"

	"github.com/iotaledger/iota.go/address"

	"github.com/gohornet/hornet/pkg/model/hornet"
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

	if err := mapstructure.Decode(i, query); err != nil {
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	if !tangle.WaitForNodeSynced(waitForNodeSyncedTimeout) {
		e.Error = ErrNodeNotSync.Error()
		c.JSON(http.StatusBadRequest, e)
		return
	}

	if len(query.Addresses) == 0 {
		e.Error = "No addresses provided"
		c.JSON(http.StatusBadRequest, e)
		return
	}

	result := WereAddressesSpentFromReturn{}

	for _, addr := range query.Addresses {
		if err := address.ValidAddress(addr); err != nil {
			e.Error = fmt.Sprintf("Provided address invalid: %s", addr)
			c.JSON(http.StatusBadRequest, e)
			return
		}

		// State
		result.States = append(result.States, tangle.WasAddressSpentFrom(hornet.HashFromAddressTrytes(addr)))
	}

	c.JSON(http.StatusOK, result)
}
