package warpsync

import (
	"context"
	"time"

	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/core/app"
	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hornet/v2/pkg/daemon"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/v2/pkg/protocol/gossip"
	"github.com/iotaledger/hornet/v2/pkg/tangle"
	iotago "github.com/iotaledger/iota.go/v3"
)

func init() {
	Plugin = &app.Plugin{
		Component: &app.Component{
			Name:      "WarpSync",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Configure: configure,
			Run:       run,
		},
		IsEnabled: func() bool {
			return ParamsWarpSync.Enabled
		},
	}
}

var (
	Plugin *app.Plugin
	deps   dependencies

	warpSync                   *gossip.WarpSync
	warpSyncMilestoneRequester *gossip.WarpSyncMilestoneRequester

	onGossipServiceProtocolStarted    *events.Closure
	onGossipServiceProtocolTerminated *events.Closure
	onReferencedBlocksCountUpdated    *events.Closure
	onMilestoneSolidificationFailed   *events.Closure
	onWarpSyncCheckpointUpdated       *events.Closure
	onWarpSyncTargetUpdated           *events.Closure
	onWarpSyncStart                   *events.Closure
	onWarpSyncDone                    *events.Closure
)

type dependencies struct {
	dig.In
	Storage       *storage.Storage
	SyncManager   *syncmanager.SyncManager
	Tangle        *tangle.Tangle
	RequestQueue  gossip.RequestQueue
	GossipService *gossip.Service
	Requester     *gossip.Requester
}

func configure() error {
	warpSync = gossip.NewWarpSync(ParamsWarpSync.AdvancementRange)
	warpSyncMilestoneRequester = gossip.NewWarpSyncMilestoneRequester(Plugin.Daemon().ContextStopped(), deps.Storage, deps.SyncManager, deps.Requester, true)
	configureEvents()

	return nil
}

func run() error {
	if err := Plugin.Daemon().BackgroundWorker("WarpSync[PeerEvents]", func(ctx context.Context) {
		attachEvents()
		<-ctx.Done()
		detachEvents()
	}, daemon.PriorityWarpSync); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
	}

	return nil
}

func configureEvents() {

	onHeartbeatUpdated := events.NewClosure(func(hb *gossip.Heartbeat) {
		warpSync.UpdateCurrentConfirmedMilestone(deps.SyncManager.ConfirmedMilestoneIndex())
		warpSync.UpdateTargetMilestone(hb.SolidMilestoneIndex)
	})

	onGossipServiceProtocolStarted = events.NewClosure(func(p *gossip.Protocol) {
		p.Events.HeartbeatUpdated.Hook(onHeartbeatUpdated)
	})

	onGossipServiceProtocolTerminated = events.NewClosure(func(p *gossip.Protocol) {
		p.Events.HeartbeatUpdated.Detach(onHeartbeatUpdated)
	})

	onReferencedBlocksCountUpdated = events.NewClosure(func(msIndex iotago.MilestoneIndex, referencedBlocksCount int) {
		warpSync.AddReferencedBlocksCount(referencedBlocksCount)
		warpSync.UpdateCurrentConfirmedMilestone(msIndex)
	})

	onMilestoneSolidificationFailed = events.NewClosure(func(msIndex iotago.MilestoneIndex) {
		if warpSync.CurrentCheckpoint != 0 && warpSync.CurrentCheckpoint < msIndex {
			// rerequest since milestone requests could have been lost
			_, msIndexStart, msIndexEnd := warpSyncMilestoneRequester.RequestMilestoneRange(warpSync.AdvancementRange)
			Plugin.LogInfof("Requesting missing milestones %d - %d", msIndexStart, msIndexEnd)
		}
	})

	onWarpSyncCheckpointUpdated = events.NewClosure(func(nextCheckpoint iotago.MilestoneIndex, oldCheckpoint iotago.MilestoneIndex, advRange syncmanager.MilestoneIndexDelta, target iotago.MilestoneIndex) {
		Plugin.LogInfof("Checkpoint updated to milestone %d (target %d)", nextCheckpoint, target)
		// prevent any requests in the queue above our next checkpoint
		deps.RequestQueue.Filter(func(r *gossip.Request) bool {
			return r.MilestoneIndex <= nextCheckpoint
		})
		_, _, _ = warpSyncMilestoneRequester.RequestMilestoneRange(advRange, oldCheckpoint)
	})

	onWarpSyncTargetUpdated = events.NewClosure(func(checkpoint iotago.MilestoneIndex, newTarget iotago.MilestoneIndex) {
		Plugin.LogInfof("Target updated to milestone %d (checkpoint %d)", newTarget, checkpoint)
	})

	onWarpSyncStart = events.NewClosure(func(targetMsIndex iotago.MilestoneIndex, nextCheckpoint iotago.MilestoneIndex, advRange syncmanager.MilestoneIndexDelta) {
		Plugin.LogInfof("Synchronizing to milestone %d", targetMsIndex)
		deps.RequestQueue.Filter(func(r *gossip.Request) bool {
			return r.MilestoneIndex <= nextCheckpoint
		})

		msRequested, _, _ := warpSyncMilestoneRequester.RequestMilestoneRange(advRange)
		// if the amount of requested milestones doesn't correspond to the range,
		// it means we already had the milestones in the database, which suggests
		// that we should manually kick start the milestone solidifier.
		if msRequested != advRange {
			Plugin.LogInfo("Manually starting solidifier, as some milestones are already in the database")
			deps.Tangle.TriggerSolidifier()
		}
	})

	onWarpSyncDone = events.NewClosure(func(deltaSynced int, referencedBlocksTotal int, took time.Duration) {
		// we need to cleanup all memoized things in the requester, so we have a clean state at next run and free the memory.
		// we can only reset the "traversed" blocks here, because otherwise it may happen that the requester always
		// walks the whole cone if there are already paths between newer milestones in the database.
		warpSyncMilestoneRequester.Cleanup()

		Plugin.LogInfof("Synchronized %d milestones in %v (%0.2f BPS)", deltaSynced, took.Truncate(time.Millisecond), float64(referencedBlocksTotal)/took.Seconds())
		deps.RequestQueue.Filter(nil)
	})
}

func attachEvents() {
	deps.GossipService.Events.ProtocolStarted.Hook(onGossipServiceProtocolStarted)
	deps.GossipService.Events.ProtocolTerminated.Hook(onGossipServiceProtocolTerminated)
	deps.Tangle.Events.ReferencedBlocksCountUpdated.Hook(onReferencedBlocksCountUpdated)
	deps.Tangle.Events.MilestoneSolidificationFailed.Hook(onMilestoneSolidificationFailed)
	warpSync.Events.CheckpointUpdated.Hook(onWarpSyncCheckpointUpdated)
	warpSync.Events.TargetUpdated.Hook(onWarpSyncTargetUpdated)
	warpSync.Events.Start.Hook(onWarpSyncStart)
	warpSync.Events.Done.Hook(onWarpSyncDone)
}

func detachEvents() {
	deps.GossipService.Events.ProtocolStarted.Detach(onGossipServiceProtocolStarted)
	deps.GossipService.Events.ProtocolTerminated.Detach(onGossipServiceProtocolTerminated)
	deps.Tangle.Events.ReferencedBlocksCountUpdated.Detach(onReferencedBlocksCountUpdated)
	deps.Tangle.Events.MilestoneSolidificationFailed.Detach(onMilestoneSolidificationFailed)
	warpSync.Events.CheckpointUpdated.Detach(onWarpSyncCheckpointUpdated)
	warpSync.Events.TargetUpdated.Detach(onWarpSyncTargetUpdated)
	warpSync.Events.Start.Detach(onWarpSyncStart)
	warpSync.Events.Done.Detach(onWarpSyncDone)
}
