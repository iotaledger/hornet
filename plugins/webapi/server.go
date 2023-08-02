package webapi

import (
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/iotaledger/hive.go/logger"
)

const (
	APIRouteRest = "/api/core/v0"
)

type WebAPIServer struct {
	logger *logger.Logger

	rpcEndpoints       map[string]rpcEndpoint
	rpcEndpointsPublic map[string]struct{}
	limitsMaxResults   int
	features           []string
}

func NewWebAPIServer(
	e *echo.Echo,
	log *logger.Logger,
	rpcEndpointsPublic []string,
	maxResults int) *WebAPIServer {

	s := &WebAPIServer{
		logger:             log,
		rpcEndpoints:       make(map[string]rpcEndpoint),
		rpcEndpointsPublic: make(map[string]struct{}),
		limitsMaxResults:   maxResults,
		features:           make([]string, 0),
	}

	// Load allowed remote access to specific HTTP API commands
	for _, endpoint := range rpcEndpointsPublic {
		s.rpcEndpointsPublic[strings.ToLower(endpoint)] = struct{}{}
	}

	s.configureRPCEndpoint(e.Group(""))
	s.configureRestRoutes(e.Group(APIRouteRest))

	return s
}

func (s *WebAPIServer) HasRPCEndpoint(endpoint string) bool {
	_, exists := s.rpcEndpoints[strings.ToLower(endpoint)]
	return exists
}

func (s *WebAPIServer) SetFeatures(features []string) {
	s.features = features
}
