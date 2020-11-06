package restapi

import (
	"github.com/gohornet/hornet/core/cli"
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
	// whether to use HTTP basic auth for the REST API
	CfgRestAPIBasicAuthEnabled = "restAPI.basicAuth.enabled"
	// the username of the HTTP basic auth
	CfgRestAPIBasicAuthUsername = "restAPI.basicAuth.username"
	// the HTTP basic auth password+salt as a sha256 hash
	CfgRestAPIBasicAuthPasswordHash = "restAPI.basicAuth.passwordHash"
	// the HTTP basic auth salt used for hashing the password
	CfgRestAPIBasicAuthPasswordSalt = "restAPI.basicAuth.passwordSalt"
	// the maximum number of characters that the body of an API call may contain
	CfgRestAPILimitsMaxBodyLength = "restAPI.limits.bodyLength"
	// the maximum number of results that may be returned by an endpoint
	CfgRestAPILimitsMaxResults = "restAPI.limits.maxResults"
)

func init() {
	cli.ConfigFlagSet.String(CfgRestAPIBindAddress, "0.0.0.0:14265", "the bind address on which the REST API listens on")
	cli.ConfigFlagSet.StringSlice(CfgRestAPIPermittedRoutes,
		[]string{
			"/health",
			"/api/v1/info",
			"/api/v1/tips",
			"/api/v1/messages/:messageID",
			"/api/v1/messages/:messageID/metadata",
			"/api/v1/messages/:messageID/raw",
			"/api/v1/messages/:messageID/children",
			"/api/v1/messages",
			"/api/v1/milestones/:milestoneIndex",
			"/api/v1/outputs/:outputID",
			"/api/v1/addresses/:address",
			"/api/v1/addresses/:address/outputs",
		}, "the allowed HTTP REST routes which can be called from non whitelisted addresses")
	cli.ConfigFlagSet.StringSlice(CfgRestAPIWhitelistedAddresses, []string{"127.0.0.1", "::1"}, "the whitelist of addresses which are allowed to access the REST API")
	cli.ConfigFlagSet.Bool(CfgRestAPIExcludeHealthCheckFromAuth, false, "whether to allow the health check route anyways")
	cli.ConfigFlagSet.Bool(CfgRestAPIBasicAuthEnabled, false, "whether to use HTTP basic auth for the REST API")
	cli.ConfigFlagSet.String(CfgRestAPIBasicAuthUsername, "", "the username of the HTTP basic auth")
	cli.ConfigFlagSet.String(CfgRestAPIBasicAuthPasswordHash, "", "the HTTP basic auth password+salt as a sha256 hash")
	cli.ConfigFlagSet.String(CfgRestAPIBasicAuthPasswordSalt, "", "the HTTP basic auth salt used for hashing the password")
	cli.ConfigFlagSet.String(CfgRestAPILimitsMaxBodyLength, "1M", "the maximum number of characters that the body of an API call may contain")
	cli.ConfigFlagSet.Int(CfgRestAPILimitsMaxResults, 1000, "the maximum number of results that may be returned by an endpoint")
}
