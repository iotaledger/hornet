package webapi

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	"net/http"
)

func init() {
	addEndpoint("getLedgerDiff", getLedgerDiff, implementedAPIcalls)
}

func getLedgerDiff(i interface{}, c *gin.Context) {
	ld := &GetLedgerDiff{}
	e := ErrorReturn{}

	err := mapstructure.Decode(i, ld)
	if err != nil {
		e.Error = "Internal error"
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	smi := tangle.GetSolidMilestoneIndex()
	requestedIndex := milestone_index.MilestoneIndex(ld.MilestoneIndex)
	if requestedIndex > smi {
		e.Error = fmt.Sprintf("Invalid milestone index supplied, lsmi is %d", smi)
		c.JSON(http.StatusBadRequest, e)
		return
	}

	ldr := &GetLedgerDiffReturn{}

	diff, err := tangle.GetLedgerDiffForMilestone(requestedIndex)
	if err != nil {
		e.Error = "Internal error"
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	ldr.Diff = diff
	ldr.MilestoneIndex = ld.MilestoneIndex

	c.JSON(http.StatusOK, ldr)
}
