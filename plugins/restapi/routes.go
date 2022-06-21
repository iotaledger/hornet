package restapi

import (
	"net/http"

	"github.com/labstack/echo/v4"
	
	"github.com/iotaledger/hornet/pkg/restapi"
)

const (
	nodeAPIHealthRoute = "/health"

	nodeAPIRoutesRoute = "/api/routes"
)

type RoutesResponse struct {
	Routes []string `json:"routes"`
}

func setupRoutes() {

	errorHandler := restapi.ErrorHandler()

	deps.Echo.HTTPErrorHandler = func(err error, c echo.Context) {
		Plugin.LogDebugf("HTTP request failed: %s", err)
		deps.RestAPIMetrics.HTTPRequestErrorCounter.Inc()

		errorHandler(err, c)
	}

	deps.Echo.GET(nodeAPIHealthRoute, func(c echo.Context) error {
		// node mode
		if deps.Tangle != nil && !deps.Tangle.IsNodeHealthy() {
			return c.NoContent(http.StatusServiceUnavailable)
		}
		return c.NoContent(http.StatusOK)
	})

	deps.Echo.GET(nodeAPIRoutesRoute, func(c echo.Context) error {

		resp := &RoutesResponse{
			Routes: deps.RestRouteManager.Routes(),
		}
		return restapi.JSONResponse(c, http.StatusOK, resp)
	})
}
