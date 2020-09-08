package webapi

import (
	"strings"

	"github.com/gin-gonic/gin"
)

type apiEndpoint func(i interface{}, c *gin.Context, abortSignal <-chan struct{})

func addEndpoint(endpointName string, implementation apiEndpoint, availableImplementions map[string]apiEndpoint) {
	ep := strings.ToLower(endpointName)
	availableImplementions[ep] = implementation
}
