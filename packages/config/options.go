package config

const (
	CfgNetPreferIPv6                            = "network.preferIPv6"
	CfgNetGossipBindAddress                     = "network.gossip.bindAddress"
	CfgNetGossipReconnectAttemptIntervalSeconds = "network.gossip.reconnectAttemptIntervalSeconds"
)

const (
	CfgMilestoneCoordinator              = "milestones.coordinator"
	CfgMilestoneCoordinatorSecurityLevel = "milestones.coordinatorSecurityLevel"
	CfgMilestoneNumberOfKeysInAMilestone = "milestones.numberOfKeysInAMilestone"
	CfgProtocolMWM                       = "protocol.mwm"
)

const (
	CfgNeighborsAcceptAnyNeighborConnection = "acceptAnyNeighborConnection"
	CfgNeighborsMaxNeighbors                = "maxNeighbors"
	CfgNeighbors                            = "neighbors"
)

const (
	CfgNetAutopeeringEntryNodes     = "network.autopeering.entryNodes"
	CfgNetAutopeeringBindAddr       = "network.autopeering.bindAddress"
	CfgNetAutopeeringExternalAddr   = "network.autopeering.externalAddress"
	CfgNetAutopeeringSeed           = "network.autopeering.seed"
	CfgNetAutopeeringRunAsEntryNode = "network.autopeering.runAsEntryNode"
)

const (
	CfgGraphWebRootPath  = "graph.webRootPath"
	CfgGraphWebSocketURI = "graph.webSocket.uri"
	CfgGraphDomain       = "graph.domain"
	CfgGraphBindAddress  = "graph.bindAddress"
	CfgGraphNetworkName  = "graph.networkName"
)

const (
	CfgMonitorTangleMonitorPath = "monitor.tangleMonitorPath"
	CfgMonitorDomain            = "monitor.domain"
	CfgMonitorWebBindAddress    = "monitor.webBindAddress"
	CfgMonitorAPIBindAddress    = "monitor.apiBindAddress"
)

const (
	CfgMQTTConfig = "mqtt.config"
)

const (
	CfgProfilingBindAddress = "profiling.bindAddress"
)

const (
	CfgSnapshotLoadType = "snapshots.loadType"
)

const (
	CfgLocalSnapshotsEnabled            = "snapshots.local.enabled"
	CfgLocalSnapshotsDepth              = "snapshots.local.depth"
	CfgLocalSnapshotsIntervalSynced     = "snapshots.local.intervalSynced"
	CfgLocalSnapshotsIntervalUnsynced   = "snapshots.local.intervalUnsynced"
	CfgLocalSnapshotsPath               = "snapshots.local.path"
	CfgGlobalSnapshotPath               = "snapshots.global.path"
	CfgGlobalSnapshotSpentAddressesPath = "snapshots.global.spentAddressesPath"
	CfgGlobalSnapshotIndex              = "snapshots.global.index"
	CfgPruningEnabled                   = "snapshots.pruning.enabled"
	CfgPruningDelay                     = "snapshots.pruning.delay"
	CfgSpentAddressesEnabled            = "spentAddresses.enabled"
)

const (
	CfgDashboardBindAddress       = "dashboard.bindAddress"
	CfgDashboardDevMode           = "dashboard.dev"
	CfgDashboardTheme             = "dashboard.theme"
	CfgDashboardBasicAuthEnabled  = "dashboard.basicAuth.enabled"
	CfgDashboardBasicAuthUsername = "dashboard.basicAuth.username"
	CfgDashboardBasicAuthPassword = "dashboard.basicauth.password" // must be lower cased
)

const (
	CfgSpammerAddress      = "spammer.address"
	CfgSpammerMessage      = "spammer.message"
	CfgSpammerTag          = "spammer.tag"
	CfgSpammerDepth        = "spammer.depth"
	CfgSpammerTPSRateLimit = "spammer.tpsRateLimit"
	CfgSpammerWorkers      = "spammer.workers"
)

const (
	CfgDatabasePath         = "db.path"
	CfgCompassLoadLSMIAsLMI = "compass.loadLSMIAsLMI"
)

const (
	CfgTipSelMaxDepth                      = "tipsel.maxDepth"
	CfgTipSelBelowMaxDepthTransactionLimit = "tipsel.belowMaxDepthTransactionLimit"
)

const (
	CfgWebAPIBindAddress               = "httpAPI.bindAddress"
	CfgWebAPIPermitRemoteAccess        = "httpAPI.permitRemoteAccess"
	CfgWebAPIWhitelistedAddresses      = "httpAPI.whitelistedAddresses"
	CfgWebAPIBasicAuthEnabled          = "httpAPI.basicAuth.enabled"
	CfgWebAPIBasicAuthUsername         = "httpAPI.basicAuth.username"
	CfgWebAPIBasicAuthPassword         = "httpapi.basicauth.password" // must be lower cased
	CfgWebAPILimitsMaxBodyLengthBytes  = "httpAPI.limits.bodyLengthBytes"
	CfgWebAPILimitsMaxFindTransactions = "httpAPI.limits.findTransactions"
	CfgWebAPILimitsMaxGetTrytes        = "httpAPI.limits.getTrytes"
	CfgWebAPILimitsMaxRequestsList     = "httpAPI.limits.requestsList"
)

const (
	CfgZMQBindAddress = "zmq.bindAddress"
	CfgZMQProtocol    = "zmq.protocol"
)
