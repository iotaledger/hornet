package config

const (
	// the bind address on which the HTTP API listens on
	CfgWebAPIBindAddress = "httpAPI.bindAddress"
	// the allowed HTTP API calls which can be called from non whitelisted addresses
	CfgWebAPIPermitRemoteAccess = "httpAPI.permitRemoteAccess"
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
	NodeConfig.SetDefault(CfgWebAPIBindAddress, "0.0.0.0:14265")
	NodeConfig.SetDefault(CfgWebAPIPermitRemoteAccess,
		[]string{
			"getNodeInfo",
			"getBalances",
			"checkConsistency",
			"getTransactionsToApprove",
			"getInclusionStates",
			"getNodeAPIConfiguration",
			"wereAddressesSpentFrom",
			"broadcastTransactions",
			"findTransactions",
			"storeTransactions",
			"getTrytes",
		})
	NodeConfig.SetDefault(CfgWebAPIWhitelistedAddresses, []string{})
	NodeConfig.SetDefault(CfgWebAPIExcludeHealthCheckFromAuth, false)
	NodeConfig.SetDefault(CfgWebAPIBasicAuthEnabled, false)
	NodeConfig.SetDefault(CfgWebAPIBasicAuthUsername, "")
	NodeConfig.SetDefault(CfgWebAPIBasicAuthPasswordHash, "")
	NodeConfig.SetDefault(CfgWebAPIBasicAuthPasswordSalt, "")
	NodeConfig.SetDefault(CfgWebAPILimitsMaxGetTrytes, 1000)
	NodeConfig.SetDefault(CfgWebAPILimitsMaxRequestsList, 1000)
	NodeConfig.SetDefault(CfgWebAPILimitsMaxFindTransactions, 1000)
	NodeConfig.SetDefault(CfgWebAPILimitsMaxBodyLengthBytes, 1000000)
}
