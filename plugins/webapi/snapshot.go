package webapi

import (
	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
	"github.com/gohornet/hornet/packages/model/tangle"
	"net/http"
)

func init() {
	addEndpoint("getSnapshot", getSnapshot, implementedAPIcalls)
}

func getSnapshot(i interface{}, c *gin.Context) {
	sn := &GetSnapshot{}
	e := ErrorReturn{}

	err := mapstructure.Decode(i, sn)
	if err != nil {
		e.Error = "Internal error"
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	snr := &GetSnapshotReturn{}

	balances, index, err := tangle.GetAllBalances()
	if err != nil {
		e.Error = "Internal error"
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	snr.Balances = balances
	snr.MilestoneIndex = uint64(index)

	c.JSON(http.StatusOK, snr)
}
