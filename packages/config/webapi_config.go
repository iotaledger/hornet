package config

const (
	// the bind address on which the HTTP API listens on
	CfgWebAPIBindAddress = "httpAPI.bindAddress"
	// the allowed HTTP API calls which can be called from non whitelisted addresses
	CfgWebAPIPermitRemoteAccess = "httpAPI.permitRemoteAccess"
	// the whitelist of addresses which are allowed to access the HTTP API
	CfgWebAPIWhitelistedAddresses = "httpAPI.whitelistedAddresses"
	// whether to use HTTP basic auth for the HTTP API
	CfgWebAPIBasicAuthEnabled = "httpAPI.basicAuth.enabled"
	// the username of the HTTP basic auth
	CfgWebAPIBasicAuthUsername = "httpAPI.basicAuth.username"
	// the password of the HTTP basic auth
	CfgWebAPIBasicAuthPassword = "httpapi.basicauth.password" // must be lower cased
	// the maximum number of trytes that may be returned by the getTrytes endpoint
	CfgWebAPILimitsMaxBodyLengthBytes = "httpAPI.limits.bodyLengthBytes"
	// the maximum number of parameters in an API call
	CfgWebAPILimitsMaxFindTransactions = "httpAPI.limits.findTransactions"
	// the maximum number of transactions that may be returned by the findTransactions endpoint
	CfgWebAPILimitsMaxGetTrytes = "httpAPI.limits.getTrytes"
	// the maximum number of characters that the body of an API call may contain
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
	NodeConfig.SetDefault(CfgWebAPIBasicAuthEnabled, "")
	NodeConfig.SetDefault(CfgWebAPIBasicAuthUsername, "")
	NodeConfig.SetDefault(CfgWebAPIBasicAuthPassword, "")
	NodeConfig.SetDefault(CfgWebAPILimitsMaxGetTrytes, 10000)
	NodeConfig.SetDefault(CfgWebAPILimitsMaxRequestsList, 1000)
	NodeConfig.SetDefault(CfgWebAPILimitsMaxFindTransactions, 100000)
	NodeConfig.SetDefault(CfgWebAPILimitsMaxBodyLengthBytes, 1000000)
}
