package warpsync

import (
	"time"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/peering/peer"
	"github.com/gohornet/hornet/pkg/protocol/rqueue"
	"github.com/gohornet/hornet/pkg/protocol/sting"
	"github.com/gohornet/hornet/pkg/protocol/warpsync"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/plugins/gossip"
	peeringplugin "github.com/gohornet/hornet/plugins/peering"
	tangleplugin "github.com/gohornet/hornet/plugins/tangle"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

var (
	PLUGIN   = node.NewPlugin("WarpSync", node.Enabled, configure, run)
	log      *logger.Logger
	warpSync = warpsync.New(config.NodeConfig.GetInt(config.CfgWarpSyncAdvancementRange))

	onPeerConnected         *events.Closure
	onSolidMilestoneChanged *events.Closure
	onCheckpointUpdated     *events.Closure
	onTargetUpdated         *events.Closure
	onStart                 *events.Closure
	onDone                  *events.Closure
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	configureEvents()
}

func run(plugin *node.Plugin) {

	daemon.BackgroundWorker("WarpSync[Events]", func(shutdownSignal <-chan struct{}) {
		attachEvents()
		<-shutdownSignal
		detachEvents()
	}, shutdown.PriorityWarpSync)
}

func configureEvents() {

	onPeerConnected = events.NewClosure(func(p *peer.Peer) {
		if !p.Protocol.Supports(sting.FeatureSet) {
			return
		}

		p.Events.HeartbeatUpdated.Attach(events.NewClosure(func(hb *sting.Heartbeat) {
			warpSync.UpdateCurrent(tangle.GetSolidMilestoneIndex())
			warpSync.UpdateTarget(hb.SolidMilestoneIndex)
		}))
	})

	onSolidMilestoneChanged = events.NewClosure(func(cachedMsBundle *tangle.CachedBundle) { // bundle +1
		defer cachedMsBundle.Release() // bundle -1
		index := cachedMsBundle.GetBundle().GetMilestoneIndex()
		warpSync.UpdateCurrent(index)
	})

	onCheckpointUpdated = events.NewClosure(func(nextCheckpoint milestone.Index, oldCheckpoint milestone.Index, advRange int32) {
		log.Infof("Checkpoint updated to milestone %d", nextCheckpoint)
		// prevent any requests in the queue above our next checkpoint
		gossip.RequestQueue().Filter(func(r *rqueue.Request) bool {
			return r.MilestoneIndex <= nextCheckpoint
		})
		requestMissingMilestoneApprovees := gossip.MemoizedRequestMissingMilestoneApprovees()
		gossip.BroadcastMilestoneRequests(int(advRange), requestMissingMilestoneApprovees, oldCheckpoint)
	})

	onTargetUpdated = events.NewClosure(func(newTarget milestone.Index) {
		log.Infof("Target updated to milestone %d", newTarget)
	})

	onStart = events.NewClosure(func(targetMsIndex milestone.Index, nextCheckpoint milestone.Index, advRange int32) {
		log.Infof("Synchronizing to milestone %d", targetMsIndex)
		gossip.RequestQueue().Filter(func(r *rqueue.Request) bool {
			return r.MilestoneIndex <= nextCheckpoint
		})
		requestMissingMilestoneApprovees := gossip.MemoizedRequestMissingMilestoneApprovees()
		msRequested := gossip.BroadcastMilestoneRequests(int(advRange), requestMissingMilestoneApprovees)
		// if the amount of requested milestones doesn't correspond to the range,
		// it means we already had the milestones in the database, which suggests
		// that we should manually kick start the milestone solidifier.
		if msRequested != int(advRange) {
			log.Info("Manually starting solidifier, as some milestones are already in the database")
			tangleplugin.TriggerSolidifier()
		}
	})

	onDone = events.NewClosure(func(deltaSynced int, took time.Duration) {
		log.Infof("Synchronized %d milestones in %v", deltaSynced, took)
		gossip.RequestQueue().Filter(nil)
	})
}

func attachEvents() {
	peeringplugin.Manager().Events.PeerConnected.Attach(onPeerConnected)
	tangleplugin.Events.SolidMilestoneChanged.Attach(onSolidMilestoneChanged)
	warpSync.Events.CheckpointUpdated.Attach(onCheckpointUpdated)
	warpSync.Events.TargetUpdated.Attach(onTargetUpdated)
	warpSync.Events.Start.Attach(onStart)
	warpSync.Events.Done.Attach(onDone)
}

func detachEvents() {
	peeringplugin.Manager().Events.PeerConnected.Detach(onPeerConnected)
	tangleplugin.Events.SolidMilestoneChanged.Detach(onSolidMilestoneChanged)
	warpSync.Events.CheckpointUpdated.Detach(onCheckpointUpdated)
	warpSync.Events.TargetUpdated.Detach(onTargetUpdated)
	warpSync.Events.Start.Detach(onStart)
	warpSync.Events.Done.Detach(onDone)
}
