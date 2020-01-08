package webapi

import (
	"strings"

	"github.com/gin-gonic/gin"
)

type apiEndpoint func(i interface{}, c *gin.Context, abortSignal <-chan struct{})

func addEndpoint(enpointName string, implementation apiEndpoint, avaiableImplementions map[string]apiEndpoint) {
	ep := strings.ToLower(enpointName)
	avaiableImplementions[ep] = implementation
}
