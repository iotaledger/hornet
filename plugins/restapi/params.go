package restapi

import (
	"github.com/iotaledger/hive.go/core/app"
)

// ParametersRestAPI contains the definition of the parameters used by REST API.
type ParametersRestAPI struct {
	// Enabled defines whether the REST API plugin is enabled.
	Enabled bool `default:"true" usage:"whether the REST API plugin is enabled"`
	// the bind address on which the REST API listens on
	BindAddress string `default:"0.0.0.0:14265" usage:"the bind address on which the REST API listens on"`
	// the HTTP REST routes which can be called without authorization. Wildcards using * are allowed
	PublicRoutes []string `usage:"the HTTP REST routes which can be called without authorization. Wildcards using * are allowed"`
	// the HTTP REST routes which need to be called with authorization. Wildcards using * are allowed
	ProtectedRoutes []string `usage:"the HTTP REST routes which need to be called with authorization. Wildcards using * are allowed"`
	// whether the debug logging for requests should be enabled
	DebugRequestLoggerEnabled bool `default:"false" usage:"whether the debug logging for requests should be enabled"`

	JWTAuth struct {
		// salt used inside the JWT tokens for the REST API. Change this to a different value to invalidate JWT tokens not matching this new value
		Salt string `default:"HORNET" usage:"salt used inside the JWT tokens for the REST API. Change this to a different value to invalidate JWT tokens not matching this new value"`
	} `name:"jwtAuth"`

	PoW struct {
		// whether the node does PoW if blocks are received via API
		Enabled bool `default:"false" usage:"whether the node does PoW if blocks are received via API"`
		// the amount of workers used for calculating PoW when issuing blocks via API
		WorkerCount int `default:"1" usage:"the amount of workers used for calculating PoW when issuing blocks via API"`
	} `name:"pow"`

	Limits struct {
		// the maximum number of characters that the body of an API call may contain
		MaxBodyLength string `default:"1M" usage:"the maximum number of characters that the body of an API call may contain"`
		// the maximum number of results that may be returned by an endpoint
		MaxResults int `default:"1000" usage:"the maximum number of results that may be returned by an endpoint"`
	}
}

var ParamsRestAPI = &ParametersRestAPI{
	PublicRoutes: []string{
		"/health",
		"/api/routes",
		"/api/core/v2/info",
		"/api/core/v2/tips",
		"/api/core/v2/blocks*",
		"/api/core/v2/transactions*",
		"/api/core/v2/milestones*",
		"/api/core/v2/outputs*",
		"/api/core/v2/treasury",
		"/api/core/v2/receipts*",
		"/api/debug/v1/*",
		"/api/indexer/v1/*",
		"/api/mqtt/v1",
		"/api/participation/v1/events*",
		"/api/participation/v1/outputs*",
		"/api/participation/v1/addresses*",
	},
	ProtectedRoutes: []string{
		"/api/*",
	},
}

var params = &app.ComponentParams{
	Params: map[string]any{
		"restAPI": ParamsRestAPI,
	},
	Masked: []string{"restAPI.jwtAuth.salt"},
}
