package webapi

import flag "github.com/spf13/pflag"

func init() {
	flag.Int("api.port", 14265, "Set the port on which the API listens")
	flag.String("api.host", "0.0.0.0", "Set the host to which the API listens")
	flag.StringSlice(
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
		},
		"Allow remote access to certain API commands",
	)
	flag.String("api.auth.username", "", "Basic authentication user name")
	flag.String("api.auth.password", "", "Basic authentication password")
	flag.Int("api.maxGetTrytes", 10000, "Set a maximum number of trytes that may be returned by the getTrytes endpoint")
	flag.Int("api.maxRequestsList", 1000, "Set a maximum number of parameters in an API call")
	flag.Int("api.maxFindTransactions", 100000, "Set a maximum number of transactions that may be returned by the findTransactions endpoint")
	flag.Int("api.maxBodyLength", 1000000, "Set a maximum number of characters that the body of an API call may contain")
}
