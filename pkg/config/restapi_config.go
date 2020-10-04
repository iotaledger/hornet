package config

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
	CfgRestAPILimitsMaxBodyLengthBytes = "restAPI.limits.bodyLengthBytes"
	// the maximum number of results that may be returned by an endpoint
	CfgRestAPILimitsMaxResults = "restAPI.limits.maxResults"
)

func init() {
	configFlagSet.String(CfgRestAPIBindAddress, "0.0.0.0:14265", "the bind address on which the REST API listens on")
	configFlagSet.StringSlice(CfgRestAPIPermittedRoutes,
		[]string{
			"healthz",
		}, "the allowed HTTP REST routes which can be called from non whitelisted addresses")
	configFlagSet.StringSlice(CfgRestAPIWhitelistedAddresses, []string{}, "the whitelist of addresses which are allowed to access the REST API")
	configFlagSet.Bool(CfgRestAPIExcludeHealthCheckFromAuth, false, "whether to allow the health check route anyways")
	configFlagSet.Bool(CfgRestAPIBasicAuthEnabled, false, "whether to use HTTP basic auth for the REST API")
	configFlagSet.String(CfgRestAPIBasicAuthUsername, "", "the username of the HTTP basic auth")
	configFlagSet.String(CfgRestAPIBasicAuthPasswordHash, "", "the HTTP basic auth password+salt as a sha256 hash")
	configFlagSet.String(CfgRestAPIBasicAuthPasswordSalt, "", "the HTTP basic auth salt used for hashing the password")
	configFlagSet.Int(CfgRestAPILimitsMaxBodyLengthBytes, 1000000, "the maximum number of characters that the body of an API call may contain")
	configFlagSet.Int(CfgRestAPILimitsMaxResults, 1000, "the maximum number of results that may be returned by an endpoint")
}
