package webapi

import (
	"github.com/gohornet/hornet/packages/config"
)

func init() {

	// the bind address on which the HTTP API listens on
	config.NodeConfig.SetDefault(config.CfgWebAPIBindAddress, "0.0.0.0:14265")

	// the allowed HTTP API calls which can be called from non whitelisted addresses
	config.NodeConfig.SetDefault(config.CfgWebAPIPermitRemoteAccess,
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

	// the whitelist of addresses which are allowed to access the HTTP API
	config.NodeConfig.SetDefault(config.CfgWebAPIWhitelistedAddresses, []string{})

	// whether to use HTTP basic auth for the HTTP API
	config.NodeConfig.SetDefault(config.CfgWebAPIBasicAuthEnabled, "")

	// the username of the HTTP basic auth
	config.NodeConfig.SetDefault(config.CfgWebAPIBasicAuthUsername, "")

	// the password of the HTTP basic auth
	config.NodeConfig.SetDefault(config.CfgWebAPIBasicAuthPassword, "")

	// the maximum number of trytes that may be returned by the getTrytes endpoint
	config.NodeConfig.SetDefault(config.CfgWebAPILimitsMaxGetTrytes, 10000)

	// the maximum number of parameters in an API call
	config.NodeConfig.SetDefault(config.CfgWebAPILimitsMaxRequestsList, 1000)

	// the maximum number of transactions that may be returned by the findTransactions endpoint
	config.NodeConfig.SetDefault(config.CfgWebAPILimitsMaxFindTransactions, 100000)

	// the maximum number of characters that the body of an API call may contain
	config.NodeConfig.SetDefault(config.CfgWebAPILimitsMaxBodyLengthBytes, 1000000)
}
