package webapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"

	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/plugins/snapshot"
)

func init() {
	addEndpoint("getSnapshot", getSnapshot, implementedAPIcalls)
	addEndpoint("createSnapshot", createSnapshot, implementedAPIcalls)
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

func createSnapshot(i interface{}, c *gin.Context) {
	sn := &CreateSnapshot{}
	e := ErrorReturn{}

	err := mapstructure.Decode(i, sn)
	if err != nil {
		e.Error = "Internal error"
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	snr := &CreateSnapshotReturn{}

	err = snapshot.CreateLocalSnapshot(milestone_index.MilestoneIndex(sn.TargetIndex), sn.FilePath)
	if err != nil {
		e.Error = err.Error()
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	c.JSON(http.StatusOK, snr)
}
