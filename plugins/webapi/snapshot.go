package webapi

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/plugins/snapshot"
)

func init() {
	addEndpoint("createSnapshotFile", createSnapshotFile, implementedAPIcalls)
}

func createSnapshotFile(i interface{}, c *gin.Context, abortSignal <-chan struct{}) {
	e := ErrorReturn{}
	query := &CreateSnapshotFile{}

	if err := mapstructure.Decode(i, query); err != nil {
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	if err := snapshot.CreateLocalSnapshot(milestone.Index(query.TargetIndex), query.FilePath, false, abortSignal); err != nil {
		e.Error = err.Error()
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	c.JSON(http.StatusOK, CreateSnapshotFileReturn{})
}
