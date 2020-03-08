package webapi

import (
	"github.com/gohornet/hornet/packages/parameter"
)

func init() {
	// "Set the port on which the API listens"
	parameter.NodeConfig.SetDefault("api.port", 14265)

	// "Set the host to which the API listens"
	parameter.NodeConfig.SetDefault("api.bindAddress", "0.0.0.0")

	// "Allow remote access to certain API commands"
	parameter.NodeConfig.SetDefault(
		"api.permitRemoteAccess",
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

	// "Allow specified addresses and networks to access all API commands"
	parameter.NodeConfig.SetDefault("api.whitelistedAddresses", []string{})

	// "Basic authentication user name"
	parameter.NodeConfig.SetDefault("api.auth.username", "")

	// "Basic authentication password"
	parameter.NodeConfig.SetDefault("api.auth.password", "")

	// "Set a maximum number of trytes that may be returned by the getTrytes endpoint"
	parameter.NodeConfig.SetDefault("api.maxGetTrytes", 10000)

	// "Set a maximum number of parameters in an API call"
	parameter.NodeConfig.SetDefault("api.maxRequestsList", 1000)

	// "Set a maximum number of transactions that may be returned by the findTransactions endpoint"
	parameter.NodeConfig.SetDefault("api.maxFindTransactions", 100000)

	// "Set a maximum number of characters that the body of an API call may contain"
	parameter.NodeConfig.SetDefault("api.maxBodyLength", 1000000)

}
