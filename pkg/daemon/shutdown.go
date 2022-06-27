package daemon

// Please add the dependencies if you add your own priority here.
// Otherwise investigating deadlocks at shutdown is much more complicated.

const (
	PriorityCloseDatabase   = iota // no dependencies
	PriorityFlushToDatabase        // depends on PriorityCloseDatabase
	PriorityDatabaseHealth
	PriorityTipselection        // depends on PriorityFlushToDatabase, triggered by PriorityReceiveTxWorker, PriorityMilestoneSolidifier
	PriorityMilestoneSolidifier // depends on PriorityFlushToDatabase, triggered by PriorityReceiveTxWorker, PriorityMilestoneProcessor, PriorityMilestoneSolidifier, PriorityCoordinator, PriorityRestAPI, PriorityWarpSync
	PriorityMilestoneProcessor  // depends on PriorityFlushToDatabase, PriorityMilestoneSolidifier, triggered by PriorityReceiveTxWorker, PriorityMilestoneSolidifier (searchMissingMilestone)
	PrioritySolidifierGossip    // depends on PriorityFlushToDatabase, triggered by PriorityReceiveTxWorker
	PriorityReceiveTxWorker     // triggered by PriorityMessageProcessor
	PriorityMessageProcessor
	PriorityPeerGossipProtocolWrite
	PriorityPeerGossipProtocolRead
	PriorityGossipService
	PriorityRequestsProcessor // depends on PriorityGossipService
	PriorityBroadcastQueue    // depends on PriorityGossipService
	PriorityP2PManager
	PriorityAutopeering
	PriorityHeartbeats // depends on PriorityGossipService
	PriorityWarpSync
	PrioritySnapshots
	PriorityPruning
	PriorityMetricsUpdater
	PriorityPoWHandler
	PriorityRestAPI // depends on PriorityPoWHandler
	PriorityIndexer
	PriorityStatusReport
	PriorityPrometheus
)
