package inx

import (
	"context"
	"fmt"
	"net/url"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gohornet/hornet/pkg/inx"
	restapiv2 "github.com/gohornet/hornet/plugins/restapi/v2"
)

func proxyMiddleware(host string, port uint32) (echo.MiddlewareFunc, error) {
	apiURL, err := url.Parse(fmt.Sprintf("http://%s:%d", host, port))
	if err != nil {
		return nil, err
	}

	config := middleware.DefaultProxyConfig
	config.Balancer = middleware.NewRoundRobinBalancer([]*middleware.ProxyTarget{
		{
			URL: apiURL,
		},
	})
	config.Rewrite = map[string]string{
		"^/api/plugins/*": "/$1",
	}

	return middleware.ProxyWithConfig(config), nil
}

func (s *INXServer) RegisterAPIRoute(_ context.Context, req *inx.APIRouteRequest) (*inx.NoParams, error) {
	if len(req.GetRoute()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "route can not be empty")
	}
	if len(req.GetHost()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "host can not be empty")
	}
	if req.GetPort() == 0 {
		return nil, status.Error(codes.InvalidArgument, "port can not be zero")
	}
	mw, err := proxyMiddleware(req.GetHost(), req.GetPort())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid host and port combination")
	}
	restapiv2.AddPlugin(req.GetRoute()).Use(mw)
	return &inx.NoParams{}, nil
}

func (s *INXServer) UnregisterAPIRoute(_ context.Context, req *inx.APIRouteRequest) (*inx.NoParams, error) {
	if len(req.GetRoute()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "route can not be empty")
	}
	restapiv2.RemovePlugin(req.GetRoute())
	return &inx.NoParams{}, nil
}
