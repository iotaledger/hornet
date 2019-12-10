package webapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/iotaledger/iota.go/address"
	"github.com/mitchellh/mapstructure"

	"github.com/gohornet/hornet/packages/model/tangle"
)

func init() {
	addEndpoint("getBalances", getBalances, implementedAPIcalls)
}

func getBalances(i interface{}, c *gin.Context) {
	gb := &GetBalances{}
	e := ErrorReturn{}

	err := mapstructure.Decode(i, gb)
	if err != nil {
		e.Error = "Internal error"
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	if len(gb.Addresses) == 0 {
		e.Error = "No addresses provided"
		c.JSON(http.StatusBadRequest, e)
	}

	if !tangle.IsNodeSynced() {
		e.Error = "Node not synced"
		c.JSON(http.StatusBadRequest, e)
		return
	}

	gbr := &GetBalancesReturn{}

	for _, addr := range gb.Addresses {
		// Check if address is valid
		if err := address.ValidAddress(addr); err != nil {
			e.Error = "Invalid address: " + addr
			c.JSON(http.StatusBadRequest, e)
			return
		}
	}

	tangle.ReadLockLedger()
	defer tangle.ReadUnlockLedger()

	lsm, err := tangle.GetMilestone(tangle.GetSolidMilestoneIndex())
	if err != nil {
		e.Error = "Ledger state invalid - Milestone db error"
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	for _, addr := range gb.Addresses {

		balance, _, err := tangle.GetBalanceForAddressWithoutLocking(addr[:81])
		if err != nil {
			e.Error = "Ledger state invalid"
			c.JSON(http.StatusInternalServerError, e)
			return
		}

		// Address balance
		gbr.Balances = append(gbr.Balances, strconv.FormatUint(balance, 10))
	}

	// The index of the milestone that confirmed the most recent balance
	gbr.MilestoneIndex = uint32(lsm.GetMilestoneIndex())
	gbr.References = []string{lsm.GetMilestoneHash()}
	c.JSON(http.StatusOK, gbr)
}
