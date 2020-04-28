package webapi

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"

	"github.com/gohornet/hornet/plugins/snapshot"
)

func init() {
	addEndpoint("pruneDatabase", pruneDatabase, implementedAPIcalls)
}

func pruneDatabase(i interface{}, c *gin.Context, abortSignal <-chan struct{}) {
	e := ErrorReturn{}
	query := &PruneDatabase{}

	if err := mapstructure.Decode(i, query); err != nil {
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	if (query.Depth != 0 && query.TargetIndex != 0) || (query.Depth == 0 && query.TargetIndex == 0) {
		e.Error = "Either depth or targetIndex has to be specified"
		c.JSON(http.StatusBadRequest, e)
		return
	}

	if query.Depth != 0 {
		if err := snapshot.PruneDatabaseByDepth(query.Depth); err != nil {
			e.Error = err.Error()
			c.JSON(http.StatusInternalServerError, e)
			return
		}
	} else {
		if err := snapshot.PruneDatabaseByTargetIndex(query.TargetIndex); err != nil {
			e.Error = err.Error()
			c.JSON(http.StatusInternalServerError, e)
			return
		}
	}

	c.JSON(http.StatusOK, PruneDatabaseReturn{})
}
