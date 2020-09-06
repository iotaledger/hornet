package config

const (
	// the bind address on which the HTTP API listens on
	CfgWebAPIBindAddress = "httpAPI.bindAddress"
	// the allowed HTTP API calls which can be called from non whitelisted addresses
	CfgWebAPIPermitRemoteAccess = "httpAPI.permitRemoteAccess"
	// the allowed HTTP REST routes which can be called from non whitelisted addresses
	CfgWebAPIPermittedRoutes = "httpAPI.permittedRoutes"
	// the whitelist of addresses which are allowed to access the HTTP API
	CfgWebAPIWhitelistedAddresses = "httpAPI.whitelistedAddresses"
	// whether to allow the health check route anyways
	CfgWebAPIExcludeHealthCheckFromAuth = "httpAPI.excludeHealthCheckFromAuth"
	// whether to use HTTP basic auth for the HTTP API
	CfgWebAPIBasicAuthEnabled = "httpAPI.basicAuth.enabled"
	// the username of the HTTP basic auth
	CfgWebAPIBasicAuthUsername = "httpAPI.basicAuth.username"
	// the HTTP basic auth password+salt as a sha256 hash
	CfgWebAPIBasicAuthPasswordHash = "httpapi.basicauth.passwordhash" // must be lower cased
	// the HTTP basic auth salt used for hashing the password
	CfgWebAPIBasicAuthPasswordSalt = "httpapi.basicauth.passwordsalt" // must be lower cased
	// the maximum number of characters that the body of an API call may contain
	CfgWebAPILimitsMaxBodyLengthBytes = "httpAPI.limits.bodyLengthBytes"
	// the maximum number of transactions that may be returned by the findTransactions endpoint
	CfgWebAPILimitsMaxFindTransactions = "httpAPI.limits.findTransactions"
	// the maximum number of trytes that may be returned by the getTrytes endpoint
	CfgWebAPILimitsMaxGetTrytes = "httpAPI.limits.getTrytes"
	// the maximum number of parameters in an API call
	CfgWebAPILimitsMaxRequestsList = "httpAPI.limits.requestsList"
)

func init() {
	configFlagSet.String(CfgWebAPIBindAddress, "0.0.0.0:14265", "the bind address on which the HTTP API listens on")
	configFlagSet.StringSlice(CfgWebAPIPermitRemoteAccess,
		[]string{
			"getNodeInfo",
			"getBalances",
			"getTransactionsToApprove",
			"getInclusionStates",
			"getNodeAPIConfiguration",
			"wereAddressesSpentFrom",
			"broadcastTransactions",
			"findTransactions",
			"storeTransactions",
			"getTrytes",
		}, "the allowed HTTP API calls which can be called from non whitelisted addresses")
	configFlagSet.StringSlice(CfgWebAPIPermittedRoutes,
		[]string{
			"healthz",
		}, "the allowed HTTP REST routes which can be called from non whitelisted addresses")
	configFlagSet.StringSlice(CfgWebAPIWhitelistedAddresses, []string{}, "the whitelist of addresses which are allowed to access the HTTP API")
	configFlagSet.Bool(CfgWebAPIExcludeHealthCheckFromAuth, false, "whether to allow the health check route anyways")
	configFlagSet.Bool(CfgWebAPIBasicAuthEnabled, false, "whether to use HTTP basic auth for the HTTP API")
	configFlagSet.String(CfgWebAPIBasicAuthUsername, "", "the username of the HTTP basic auth")
	configFlagSet.String(CfgWebAPIBasicAuthPasswordHash, "", "the HTTP basic auth password+salt as a sha256 hash")
	configFlagSet.String(CfgWebAPIBasicAuthPasswordSalt, "", "the HTTP basic auth salt used for hashing the password")
	configFlagSet.Int(CfgWebAPILimitsMaxBodyLengthBytes, 1000000, "the maximum number of characters that the body of an API call may contain")
	configFlagSet.Int(CfgWebAPILimitsMaxFindTransactions, 1000, "the maximum number of transactions that may be returned by the findTransactions endpoint")
	configFlagSet.Int(CfgWebAPILimitsMaxGetTrytes, 1000, "the maximum number of trytes that may be returned by the getTrytes endpoint")
	configFlagSet.Int(CfgWebAPILimitsMaxRequestsList, 1000, "the maximum number of parameters in an API call")
}
