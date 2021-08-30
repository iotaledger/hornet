package restapi

import (
	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// the bind address on which the REST API listens on
	CfgRestAPIBindAddress = "restAPI.bindAddress"
	// the allowed HTTP REST routes which can be called from non whitelisted addresses
	CfgRestAPIPermittedRoutes = "restAPI.permittedRoutes"
	// the whitelist of addresses which are allowed to access the REST API
	CfgRestAPIWhitelistedAddresses = "restAPI.whitelistedAddresses"
	// whether to allow the health check route anyways
	CfgRestAPIExcludeHealthCheckFromAuth = "restAPI.excludeHealthCheckFromAuth"
	// whether to use JWT auth for the REST API
	CfgRestAPIJWTAuthEnabled = "restAPI.jwtAuth.enabled"
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
			fs.StringSlice(CfgRestAPIPermittedRoutes,
				[]string{
					"/health",
					"/mqtt",
					"/api/v1/info",
					"/api/v1/tips",
					"/api/v1/messages/:messageID",
					"/api/v1/messages/:messageID/metadata",
					"/api/v1/messages/:messageID/raw",
					"/api/v1/messages/:messageID/children",
					"/api/v1/messages",
					"/api/v1/transactions/:transactionID/included-message",
					"/api/v1/milestones/:milestoneIndex",
					"/api/v1/milestones/:milestoneIndex/utxo-changes",
					"/api/v1/outputs/:outputID",
					"/api/v1/addresses/:address",
					"/api/v1/addresses/:address/outputs",
					"/api/v1/addresses/ed25519/:address",
					"/api/v1/addresses/ed25519/:address/outputs",
					"/api/v1/treasury",
				}, "the allowed HTTP REST routes which can be called from non whitelisted addresses")
			fs.StringSlice(CfgRestAPIWhitelistedAddresses, []string{"127.0.0.1", "::1"}, "the whitelist of addresses which are allowed to access the REST API")
			fs.Bool(CfgRestAPIExcludeHealthCheckFromAuth, false, "whether to allow the health check route anyways")
			fs.Bool(CfgRestAPIJWTAuthEnabled, false, "whether to use JWT auth for the REST API")
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
