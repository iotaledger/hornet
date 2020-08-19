package webapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func init() {
	addEndpoint("checkConsistency", checkConsistency, implementedAPIcalls)
}

func checkConsistency(i interface{}, c *gin.Context, _ <-chan struct{}) {
	c.JSON(http.StatusOK, CheckConsistencyReturn{State: true})
}
