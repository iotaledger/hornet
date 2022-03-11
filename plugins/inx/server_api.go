package inx

import (
	"bytes"
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gohornet/hornet/pkg/inx"
	"github.com/gohornet/hornet/plugins/restapi"
)

func (s *INXServer) RegisterAPIRoute(_ context.Context, req *inx.APIRouteRequest) (*inx.NoParams, error) {
	if Plugin.Node.IsSkipped(restapi.Plugin) {
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
	deps.RestPluginManager.AddPluginProxy(req.GetRoute(), req.GetHost(), req.GetPort())
	Plugin.LogInfof("Registered proxy %s => %s:%d", req.GetRoute(), req.GetHost(), req.GetPort())

	if req.GetMetricsPort() != 0 && deps.ExternalMetricsProxy != nil {
		deps.ExternalMetricsProxy.AddReverseProxy(req.GetRoute(), req.GetHost(), req.GetMetricsPort())
		Plugin.LogInfof("Registered external metrics %s => %s:%d", req.GetRoute(), req.GetHost(), req.GetMetricsPort())
	}

	return &inx.NoParams{}, nil
}

func (s *INXServer) UnregisterAPIRoute(_ context.Context, req *inx.APIRouteRequest) (*inx.NoParams, error) {
	if Plugin.Node.IsSkipped(restapi.Plugin) {
		return nil, status.Error(codes.Unavailable, "RestAPI plugin is not enabled")
	}

	if len(req.GetRoute()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "route can not be empty")
	}
	deps.RestPluginManager.RemovePlugin(req.GetRoute())
	Plugin.LogInfof("Removed proxy %s", req.GetRoute())

	if req.GetMetricsPort() != 0 && deps.ExternalMetricsProxy != nil {
		deps.ExternalMetricsProxy.RemoveReverseProxy(req.GetRoute())
		Plugin.LogInfof("Removed external metrics %s", req.GetRoute())
	}

	return &inx.NoParams{}, nil
}
