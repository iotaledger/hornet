package webapi

import (
	"github.com/labstack/echo/v4"
)

func (s *WebAPIServer) rpcCheckConsistency(c echo.Context) (interface{}, error) {
	return &CheckConsistencyResponse{State: true}, nil
}
