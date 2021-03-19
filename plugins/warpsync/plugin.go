package warpsync

import (
	"time"

	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/gohornet/hornet/pkg/whiteflag"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
)

func init() {
	Plugin = &node.Plugin{
		Status: node.Enabled,
		Pluggable: node.Pluggable{
			Name:      "WarpSync",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Configure: configure,
			Run:       run,
		},
	}
}

var (
	Plugin *node.Plugin
	log    *logger.Logger
	deps   dependencies

	warpSync                   *gossip.WarpSync
	warpSyncMilestoneRequester *gossip.WarpSyncMilestoneRequester

	onGossipProtocolStreamCreated   *events.Closure
	onMilestoneConfirmed            *events.Closure
	onMilestoneSolidificationFailed *events.Closure
	onCheckpointUpdated             *events.Closure
	onTargetUpdated                 *events.Closure
	onStart                         *events.Closure
	onDone                          *events.Closure
)

type dependencies struct {
	dig.In
	Storage      *storage.Storage
	Tangle       *tangle.Tangle
	RequestQueue gossip.RequestQueue
	Service      *gossip.Service
	Broadcaster  *gossip.Broadcaster
	Requester    *gossip.Requester
	NodeConfig   *configuration.Configuration `name:"nodeConfig"`
}

func configure() {
	log = logger.NewLogger(Plugin.Name)
	warpSync = gossip.NewWarpSync(deps.NodeConfig.Int(CfgWarpSyncAdvancementRange))
	warpSyncMilestoneRequester = gossip.NewWarpSyncMilestoneRequester(deps.Storage, deps.Requester, true)
	configureEvents()
}

func run() {
	Plugin.Daemon().BackgroundWorker("WarpSync[PeerEvents]", func(shutdownSignal <-chan struct{}) {
		attachEvents()
		<-shutdownSignal
		detachEvents()
	}, shutdown.PriorityWarpSync)
}

func configureEvents() {

	onGossipProtocolStreamCreated = events.NewClosure(func(p *gossip.Protocol) {
		p.Events.HeartbeatUpdated.Attach(events.NewClosure(func(hb *gossip.Heartbeat) {
			warpSync.UpdateCurrent(deps.Storage.GetConfirmedMilestoneIndex())
			warpSync.UpdateTarget(hb.SolidMilestoneIndex)
		}))
	})

	onMilestoneConfirmed = events.NewClosure(func(confirmation *whiteflag.Confirmation) {
		warpSync.AddReferencedMessagesCount(len(confirmation.Mutations.MessagesReferenced))
		warpSync.UpdateCurrent(confirmation.MilestoneIndex)
	})

	onMilestoneSolidificationFailed = events.NewClosure(func(msIndex milestone.Index) {
		if warpSync.CurrentCheckpoint < msIndex {
			// rerequest since milestone requests could have been lost
			log.Infof("Requesting missing milestones %d - %d", msIndex, msIndex+milestone.Index(warpSync.AdvancementRange))
			deps.Broadcaster.BroadcastMilestoneRequests(warpSync.AdvancementRange, nil)
		}
	})

	onCheckpointUpdated = events.NewClosure(func(nextCheckpoint milestone.Index, oldCheckpoint milestone.Index, advRange int32, target milestone.Index) {
		log.Infof("Checkpoint updated to milestone %d (target %d)", nextCheckpoint, target)
		// prevent any requests in the queue above our next checkpoint
		deps.RequestQueue.Filter(func(r *gossip.Request) bool {
			return r.MilestoneIndex <= nextCheckpoint
		})
		deps.Broadcaster.BroadcastMilestoneRequests(int(advRange), warpSyncMilestoneRequester.RequestMissingMilestoneParents, oldCheckpoint)
	})

	onTargetUpdated = events.NewClosure(func(checkpoint milestone.Index, newTarget milestone.Index) {
		log.Infof("Target updated to milestone %d (checkpoint %d)", newTarget, checkpoint)
	})

	onStart = events.NewClosure(func(targetMsIndex milestone.Index, nextCheckpoint milestone.Index, advRange int32) {
		log.Infof("Synchronizing to milestone %d", targetMsIndex)
		deps.RequestQueue.Filter(func(r *gossip.Request) bool {
			return r.MilestoneIndex <= nextCheckpoint
		})

		msRequested := deps.Broadcaster.BroadcastMilestoneRequests(int(advRange), warpSyncMilestoneRequester.RequestMissingMilestoneParents)
		// if the amount of requested milestones doesn't correspond to the range,
		// it means we already had the milestones in the database, which suggests
		// that we should manually kick start the milestone solidifier.
		if msRequested != int(advRange) {
			log.Info("Manually starting solidifier, as some milestones are already in the database")
			deps.Tangle.TriggerSolidifier()
		}
	})

	onDone = events.NewClosure(func(deltaSynced int, referencedMessagesTotal int, took time.Duration) {
		// we need to cleanup all memoized things in the requester, so we have a clean state at next run and free the memory.
		// we can only reset the "traversed" messages here, because otherwise it may happen that the requester always
		// walks the whole cone if there are already paths between newer milestones in the database.
		warpSyncMilestoneRequester.Cleanup()

		log.Infof("Synchronized %d milestones in %v (%0.2f MPS)", deltaSynced, took.Truncate(time.Millisecond), float64(referencedMessagesTotal)/took.Seconds())
		deps.RequestQueue.Filter(nil)
	})
}

func attachEvents() {
	deps.Service.Events.ProtocolStarted.Attach(onGossipProtocolStreamCreated)
	deps.Tangle.Events.MilestoneConfirmed.Attach(onMilestoneConfirmed)
	deps.Tangle.Events.MilestoneSolidificationFailed.Attach(onMilestoneSolidificationFailed)
	warpSync.Events.CheckpointUpdated.Attach(onCheckpointUpdated)
	warpSync.Events.TargetUpdated.Attach(onTargetUpdated)
	warpSync.Events.Start.Attach(onStart)
	warpSync.Events.Done.Attach(onDone)
}

func detachEvents() {
	deps.Service.Events.ProtocolStarted.Detach(onGossipProtocolStreamCreated)
	deps.Tangle.Events.MilestoneConfirmed.Detach(onMilestoneConfirmed)
	deps.Tangle.Events.MilestoneSolidificationFailed.Detach(onMilestoneSolidificationFailed)
	warpSync.Events.CheckpointUpdated.Detach(onCheckpointUpdated)
	warpSync.Events.TargetUpdated.Detach(onTargetUpdated)
	warpSync.Events.Start.Detach(onStart)
	warpSync.Events.Done.Detach(onDone)
}
