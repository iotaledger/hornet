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
					"/mqtt",
					"/api/v1/info",
					"/api/v1/tips",
					"/api/v1/messages*",
					"/api/v1/transactions*",
					"/api/v1/milestones*",
					"/api/v1/outputs*",
					"/api/v1/addresses*",
					"/api/v1/treasury",
					"/api/v1/receipts*",
				}, "the HTTP REST routes which can be called without authorization. Wildcards using * are allowed")
			fs.StringSlice(CfgRestAPIProtectedRoutes,
				[]string{
					"/api/v1/*",
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
