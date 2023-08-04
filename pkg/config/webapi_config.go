package config

const (
	// the bind address on which the HTTP API listens on
	CfgWebAPIBindAddress = "httpAPI.bindAddress"
	// the HTTP REST routes which can be called without authorization. Wildcards using * are allowed
	CfgWebAPIPublicRoutes = "httpAPI.publicRoutes"
	// the HTTP REST routes which need to be called with authorization. Wildcards using * are allowed
	CfgWebAPIProtectedRoutes = "httpAPI.protectedRoutes"
	// the HTTP RPC endpoints which can be called without authorization
	CfgWebAPIPublicRPCEndpoints = "httpAPI.publicRPCEndpoints"
	// the private key to sign JWT certificates (better use the file instead)
	CfgWebAPIJWTAuthPrivateKey = "httpAPI.jwtAuth.privateKey"
	// the path to the file containing the private key to sign JWT certificates
	CfgWebAPIJWTAuthPrivateKeyPath = "httpAPI.jwtAuth.privateKeyPath"
	// salt used inside the JWT tokens for the REST API. Change this to a different value to invalidate JWT tokens not matching this new value
	CfgWebAPIJWTAuthSalt = "httpAPI.jwtAuth.salt"
	// the maximum number of results that may be returned by an endpoint
	CfgWebAPILimitsMaxResults = "httpAPI.limits.maxResults"
	// the maximum number of characters that the body of an API call may contain
	CfgWebAPILimitsMaxBodyLengthBytes = "httpAPI.limits.bodyLengthBytes"
	// whether to disable the check whether a to broadcast bundle is a migration bundle
	CfgWebAPIDisableMigrationBundleCheckOnBroadcast = "httpAPI.debug.disableMigrationBundleCheckOnBroadcast"
)

func init() {
	configFlagSet.String(CfgWebAPIBindAddress, "0.0.0.0:14265", "the bind address on which the HTTP API listens on")
	configFlagSet.StringSlice(CfgWebAPIPublicRoutes,
		[]string{
			"/",
			"/healthz",
			"/api/core/v0/info",
			"/api/core/v0/milestones*",
			"/api/core/v0/transactions*",
			"/api/core/v0/addresses*",
			"/api/core/v0/ledger/diff/by-index*",
			"/api/core/v0/ledger/diff-extended/by-index*",
		}, "the HTTP REST routes which can be called without authorization. Wildcards using * are allowed")
	configFlagSet.StringSlice(CfgWebAPIProtectedRoutes,
		[]string{
			"/api/*",
		}, "the HTTP REST routes which need to be called with authorization. Wildcards using * are allowed")
	configFlagSet.StringSlice(CfgWebAPIPublicRPCEndpoints,
		[]string{
			"getNodeInfo",
			"getBalances",
			"checkConsistency",
			"getTipInfo",
			"getTransactionsToApprove",
			"getInclusionStates",
			"getNodeAPIConfiguration",
			"wereAddressesSpentFrom",
			"broadcastTransactions",
			"findTransactions",
			"storeTransactions",
			"getTrytes",
			"getWhiteFlagConfirmation",
		}, "the HTTP RPC endpoints which can be called without authorization")
	configFlagSet.String(CfgWebAPIJWTAuthPrivateKey, "", "the private key to sign JWT certificates (better use the file instead)")
	configFlagSet.String(CfgWebAPIJWTAuthPrivateKeyPath, "jwt/private.key", "the path to the file containing the private key to sign JWT certificates")
	configFlagSet.String(CfgWebAPIJWTAuthSalt, "HORNET", "salt used inside the JWT tokens for the REST API. Change this to a different value to invalidate JWT tokens not matching this new value")
	configFlagSet.Int(CfgWebAPILimitsMaxResults, 1000, "the maximum number of results that may be returned by an endpoint")
	configFlagSet.Int(CfgWebAPILimitsMaxBodyLengthBytes, 1000000, "the maximum number of characters that the body of an API call may contain")
	configFlagSet.Bool(CfgWebAPIDisableMigrationBundleCheckOnBroadcast, false, "whether to disable migration bundle check on broadcast")
}
