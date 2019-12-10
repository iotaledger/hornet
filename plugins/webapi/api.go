package webapi

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func webAPIRoute() {
	api.POST(webAPIBase, func(c *gin.Context) {

		request := make(map[string]interface{})

		err := c.ShouldBindJSON(&request)
		if err != nil {
			fmt.Println(err)
		}

		// Get the command and check if it's implemented
		cmd := strings.ToLower(fmt.Sprint(request["command"]))

		implementation, apiCallExists := implementedAPIcalls[cmd]

		// Check if command is permited. If it's not permited and the request does not come from localhost, deny it.
		_, permited := permitedEndpoints[cmd]
		if apiCallExists && !permited && c.Request.RemoteAddr[:9] != "127.0.0.1" {
			e := ErrorReturn{
				Error: "'command' is protected",
			}
			c.JSON(http.StatusForbidden, e)
			return
		}

		if !apiCallExists {
			e := ErrorReturn{
				Error: "'command' parameter has not been specified",
			}
			c.JSON(http.StatusBadRequest, e)
			return
		}

		implementation(&request, c)
	})
}
