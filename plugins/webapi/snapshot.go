package webapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/plugins/snapshot"
)

func init() {
	addEndpoint("getSnapshot", getSnapshot, implementedAPIcalls)
	addEndpoint("createSnapshot", createSnapshot, implementedAPIcalls)
}

func getSnapshot(i interface{}, c *gin.Context, abortSignal <-chan struct{}) {
	e := ErrorReturn{}

	balances, index, err := tangle.GetAllLedgerBalances(abortSignal)
	if err != nil {
		e.Error = "Internal error"
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	c.JSON(http.StatusOK, GetSnapshotReturn{Balances: balances, MilestoneIndex: uint64(index)})
}

func createSnapshot(i interface{}, c *gin.Context, abortSignal <-chan struct{}) {
	e := ErrorReturn{}
	query := &CreateSnapshot{}

	err := mapstructure.Decode(i, query)
	if err != nil {
		e.Error = "Internal error"
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	err = snapshot.CreateLocalSnapshot(milestone.Index(query.TargetIndex), query.FilePath, abortSignal)
	if err != nil {
		e.Error = err.Error()
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	c.JSON(http.StatusOK, CreateSnapshotReturn{})
}
