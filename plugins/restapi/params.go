package restapi

import (
	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// the bind address on which the REST API listens on
	CfgRestAPIBindAddress = "restAPI.bindAddress"
	// the HTTP REST routes which can be called without authorization. Wildcards using * are allowed
	CfgRestAPIPublicRoutes = "restAPI.publicRoutes"
	// the HTTP REST routes which need to be called with authorization. Wildcards using * are allowed
	CfgRestAPIProtectedRoutes = "restAPI.protectedRoutes"
	// salt used inside the JWT tokens for the REST API. Change this to a different value to invalidate JWT tokens not matching this new value
	CfgRestAPIJWTAuthSalt = "restAPI.jwtAuth.salt"
	// whether the node does PoW if messages are received via API
	CfgRestAPIPoWEnabled = "restAPI.powEnabled"
	// the amount of workers used for calculating PoW when issuing messages via API
	CfgRestAPIPoWWorkerCount = "restAPI.powWorkerCount"
	// the maximum number of characters that the body of an API call may contain
	CfgRestAPILimitsMaxBodyLength = "restAPI.limits.bodyLength"
	// the maximum number of results that may be returned by an endpoint
	CfgRestAPILimitsMaxResults = "restAPI.limits.maxResults"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.String(CfgRestAPIBindAddress, "0.0.0.0:14265", "the bind address on which the REST API listens on")
			fs.StringSlice(CfgRestAPIPublicRoutes,
				[]string{
					"/health",
					"/api/v2/info",
					"/api/v2/tips",
					"/api/v2/messages*",
					"/api/v2/transactions*",
					"/api/v2/milestones*",
					"/api/v2/outputs*",
					"/api/v2/addresses*",
					"/api/v2/treasury",
					"/api/v2/receipts*",
					"/api/plugins/participation/v1/events*",
					"/api/plugins/participation/v1/outputs*",
					"/api/plugins/participation/v1/addresses*",
				}, "the HTTP REST routes which can be called without authorization. Wildcards using * are allowed")
			fs.StringSlice(CfgRestAPIProtectedRoutes,
				[]string{
					"/api/v2/*",
					"/api/plugins/*",
				}, "the HTTP REST routes which need to be called with authorization. Wildcards using * are allowed")
			fs.String(CfgRestAPIJWTAuthSalt, "HORNET", "salt used inside the JWT tokens for the REST API. Change this to a different value to invalidate JWT tokens not matching this new value")
			fs.Bool(CfgRestAPIPoWEnabled, false, "whether the node does PoW if messages are received via API")
			fs.Int(CfgRestAPIPoWWorkerCount, 1, "the amount of workers used for calculating PoW when issuing messages via API")
			fs.String(CfgRestAPILimitsMaxBodyLength, "1M", "the maximum number of characters that the body of an API call may contain")
			fs.Int(CfgRestAPILimitsMaxResults, 1000, "the maximum number of results that may be returned by an endpoint")
			return fs
		}(),
	},
	Masked: []string{CfgRestAPIJWTAuthSalt},
}
