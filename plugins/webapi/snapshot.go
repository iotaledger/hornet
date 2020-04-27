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
	addEndpoint("createSnapshot", createSnapshot, implementedAPIcalls)
}

func createSnapshot(i interface{}, c *gin.Context, abortSignal <-chan struct{}) {
	e := ErrorReturn{}
	query := &CreateSnapshot{}

	if err := mapstructure.Decode(i, query); err != nil {
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
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
