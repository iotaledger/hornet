package shutdown

// Please add the dependencies if you add your own priority here.
// Otherwise investigating deadlocks at shutdown is much more complicated.

const (
	PriorityCloseDatabase       = iota // no dependencies
	PriorityFlushToDatabase            // depends on PriorityCloseDatabase
	PriorityTangleCache                // depends on PriorityFlushToDatabase
	PriorityTipselection               // depends on PriorityFlushToDatabase, triggered by PriorityReceiveTxWorker, PriorityMilestoneSolidifier
	PriorityMilestoneSolidifier        // depends on PriorityFlushToDatabase, triggered by PriorityReceiveTxWorker, PriorityMilestoneProcessor, PriorityMilestoneSolidifier, PriorityCoordinator, PriorityRestAPI, PriorityWarpSync
	PriorityMilestoneProcessor         // depends on PriorityFlushToDatabase, PriorityMilestoneSolidifier, triggered by PriorityReceiveTxWorker, PriorityMilestoneSolidifier (searchMissingMilestone)
	PrioritySolidifierGossip           // depends on PriorityFlushToDatabase, triggered by PriorityReceiveTxWorker
	PriorityReceiveTxWorker            // triggered by PriorityMessageProcessor
	PriorityMessageProcessor
	PriorityPeerGossipProtocolWrite
	PriorityPeerGossipProtocolRead
	PriorityGossipService
	PriorityRequestsProcessor // depends on PriorityGossipService
	PriorityBroadcastQueue    // depends on PriorityGossipService
	PriorityKademliaDHT
	PriorityPeerDiscovery
	PriorityP2PManager
	PriorityHeartbeats // depends on PriorityGossipService
	PriorityWarpSync
	PrioritySnapshots
	PriorityMetricsUpdater
	PriorityDashboard
	PriorityPoWHandler
	PriorityRestAPI // depends on PriorityPoWHandler
	PriorityMetricsPublishers
	PrioritySpammer // depends on PriorityPoWHandler
	PriorityStatusReport
	PriorityMigrator
	PriorityCoordinator // depends on PriorityPoWHandler
	PriorityUpdateCheck
	PriorityPrometheus
)
