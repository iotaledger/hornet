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
		originCommand := fmt.Sprint(request["command"])
		cmd := strings.ToLower(originCommand)

		implementation, apiCallExists := implementedAPIcalls[cmd]

		// Check if command is permited. If it's not permited and the request does not come from localhost, deny it.
		_, permited := permitedEndpoints[cmd]
		if apiCallExists && !permited && c.Request.RemoteAddr[:9] != "127.0.0.1" {
			e := ErrorReturn{
				Error: fmt.Sprintf("Command [%v] is protected", originCommand),
			}
			c.JSON(http.StatusForbidden, e)
			return
		}

		if !apiCallExists {
			e := ErrorReturn{
				Error: fmt.Sprintf("Command [%v] is unknown", originCommand),
			}
			c.JSON(http.StatusBadRequest, e)
			return
		}

		implementation(&request, c, serverShutdownSignal)
	})
}
