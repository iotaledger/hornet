package restapi

import (
	"github.com/gohornet/hornet/pkg/node"
	flag "github.com/spf13/pflag"
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
	// the HTTP basic auth password+salt as a scrypt hash
	CfgRestAPIBasicAuthPasswordHash = "restAPI.basicAuth.passwordHash"
	// the HTTP basic auth salt used for hashing the password
	CfgRestAPIBasicAuthPasswordSalt = "restAPI.basicAuth.passwordSalt"
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
			fs.StringSlice(CfgRestAPIWhitelistedAddresses, []string{"127.0.0.1", "::1"}, "the whitelist of addresses which are allowed to access the REST API")
			fs.Bool(CfgRestAPIExcludeHealthCheckFromAuth, false, "whether to allow the health check route anyways")
			fs.Bool(CfgRestAPIBasicAuthEnabled, false, "whether to use HTTP basic auth for the REST API")
			fs.String(CfgRestAPIBasicAuthUsername, "", "the username of the HTTP basic auth")
			fs.String(CfgRestAPIBasicAuthPasswordHash, "", "the HTTP basic auth password+salt as a scrypt hash")
			fs.String(CfgRestAPIBasicAuthPasswordSalt, "", "the HTTP basic auth salt used for hashing the password")
			fs.String(CfgRestAPILimitsMaxBodyLength, "1M", "the maximum number of characters that the body of an API call may contain")
			fs.Int(CfgRestAPILimitsMaxResults, 1000, "the maximum number of results that may be returned by an endpoint")
			return fs
		}(),
	},
	Masked: []string{CfgRestAPIBasicAuthPasswordHash, CfgRestAPIBasicAuthPasswordSalt},
}
