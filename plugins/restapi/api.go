package restapi

import (
	"net"

	"github.com/labstack/echo/v4"
)

func networkWhitelisted(c echo.Context) bool {
	remoteHost, _, _ := net.SplitHostPort(c.Request().RemoteAddr)
	remoteAddress := net.ParseIP(remoteHost)
	for _, whitelistedNet := range whitelistedNetworks {
		if whitelistedNet.Contains(remoteAddress) {
			return true
		}
	}
	return false
}

