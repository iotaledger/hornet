package inx

import (
	"bytes"
	"context"
	"net/http/httptest"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotaledger/hornet/v2/plugins/restapi"
	inx "github.com/iotaledger/inx/go"
)

func (s *Server) RegisterAPIRoute(_ context.Context, req *inx.APIRouteRequest) (*inx.NoParams, error) {
	if Plugin.App().IsPluginSkipped(restapi.Plugin) {
		return nil, status.Error(codes.Unavailable, "RestAPI plugin is not enabled")
	}

	if len(req.GetRoute()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "route can not be empty")
	}
	if len(req.GetHost()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "host can not be empty")
	}
	if req.GetPort() == 0 {
		return nil, status.Error(codes.InvalidArgument, "port can not be zero")
	}
	if err := deps.RestRouteManager.AddProxyRoute(req.GetRoute(), req.GetHost(), req.GetPort()); err != nil {
		Plugin.LogErrorf("Error registering proxy %s", req.GetRoute())

		return nil, status.Errorf(codes.Internal, "error adding route to proxy: %s", err.Error())
	}
	Plugin.LogInfof("Registered proxy %s => %s:%d", req.GetRoute(), req.GetHost(), req.GetPort())

	return &inx.NoParams{}, nil
}

func (s *Server) UnregisterAPIRoute(_ context.Context, req *inx.APIRouteRequest) (*inx.NoParams, error) {
	if Plugin.App().IsPluginSkipped(restapi.Plugin) {
		return nil, status.Error(codes.Unavailable, "RestAPI plugin is not enabled")
	}

	if len(req.GetRoute()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "route can not be empty")
	}
	deps.RestRouteManager.RemoveRoute(req.GetRoute())
	Plugin.LogInfof("Removed proxy %s", req.GetRoute())

	return &inx.NoParams{}, nil
}

func (s *Server) PerformAPIRequest(_ context.Context, req *inx.APIRequest) (*inx.APIResponse, error) {
	if Plugin.App().IsPluginSkipped(restapi.Plugin) {
		return nil, status.Error(codes.Unavailable, "RestAPI plugin is not enabled")
	}

	httpReq := httptest.NewRequest(req.GetMethod(), req.GetPath(), bytes.NewBuffer(req.GetBody()))
	httpReq.Header = req.HTTPHeader()

	rec := httptest.NewRecorder()
	c := deps.Echo.NewContext(httpReq, rec)
	deps.Echo.Router().Find(req.GetMethod(), req.GetPath(), c)
	if err := c.Handler()(c); err != nil {
		return nil, err
	}

	return &inx.APIResponse{
		Code:    uint32(rec.Code),
		Headers: inx.HeadersFromHTTPHeader(rec.Header()),
		Body:    rec.Body.Bytes(),
	}, nil
}
