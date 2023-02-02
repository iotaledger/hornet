package poi

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/iotaledger/hornet/pkg/restapi"
)

const (
	RouteCreateProof   = "/create/:" + restapi.ParameterMessageID
	RouteValidateProof = "/validate"
)

func setupRoutes(routeGroup *echo.Group) {

	routeGroup.GET(RouteCreateProof, func(c echo.Context) error {
		resp, err := createProof(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.POST(RouteValidateProof, func(c echo.Context) error {
		resp, err := validateProof(c)
		if err != nil {
			return err
		}

		return restapi.JSONResponse(c, http.StatusOK, resp)
	})
}
