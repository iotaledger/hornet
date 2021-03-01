package restapi

import (
	"net"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/gohornet/hornet/pkg/restapi"
)

func networkWhitelisted(c echo.Context, whitelistedNetworks []*net.IPNet) bool {
	remoteHost, _, _ := net.SplitHostPort(c.Request().RemoteAddr)
	remoteAddress := net.ParseIP(remoteHost)
	for _, whitelistedNet := range whitelistedNetworks {
		if whitelistedNet.Contains(remoteAddress) {
			return true
		}
	}
	return false
}

func middlewareFilterRoutes(whitelistedNetworks []*net.IPNet, permittedRoutes map[string]struct{}) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !networkWhitelisted(c, whitelistedNetworks) {
				// network is not whitelisted, check if the route is permitted, otherwise deny it.
				if _, permitted := permittedRoutes[strings.ToLower(c.Path())]; !permitted {
					return restapi.ErrForbidden
				}
			}
			return next(c)
		}
	}
}
