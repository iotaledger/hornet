package warpsync

import (
	"context"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hornet/v2/pkg/components"
	"github.com/iotaledger/hornet/v2/pkg/daemon"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/v2/pkg/protocol/gossip"
	"github.com/iotaledger/hornet/v2/pkg/tangle"
	iotago "github.com/iotaledger/iota.go/v3"
)

func init() {
	Component = &app.Component{
		Name:     "WarpSync",
		DepsFunc: func(cDeps dependencies) { deps = cDeps },
		Params:   params,
		IsEnabled: func(c *dig.Container) bool {
			// do not enable in "autopeering entry node" mode
			return components.IsAutopeeringEntryNodeDisabled(c) && ParamsWarpSync.Enabled
		},
		Configure: configure,
		Run:       run,
	}
}

var (
	Component *app.Component
	deps      dependencies

	warpSync                   *gossip.WarpSync
	warpSyncMilestoneRequester *gossip.WarpSyncMilestoneRequester

	heartbeatHooks       *shrinkingmap.ShrinkingMap[peer.ID, func()]
	heartbeatsHooksMutex sync.Mutex
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
	warpSyncMilestoneRequester = gossip.NewWarpSyncMilestoneRequester(Component.Daemon().ContextStopped(), deps.Storage, deps.SyncManager, deps.Requester, true)

	heartbeatHooks = shrinkingmap.New[peer.ID, func()]()

	return nil
}

func run() error {
	if err := Component.Daemon().BackgroundWorker("WarpSync[PeerEvents]", func(ctx context.Context) {
		unhook := hookEvents()
		defer unhook()
		<-ctx.Done()
	}, daemon.PriorityWarpSync); err != nil {
		Component.LogPanicf("failed to start worker: %s", err)
	}

	return nil
}

func hookHeartbeatEvents(p *gossip.Protocol) {
	heartbeatsHooksMutex.Lock()
	defer heartbeatsHooksMutex.Unlock()

	if unhook, exists := heartbeatHooks.Get(p.PeerID); exists {
		unhook()
		heartbeatHooks.Delete(p.PeerID)
	}

	heartbeatHooks.Set(p.PeerID, p.Events.HeartbeatUpdated.Hook(func(hb *gossip.Heartbeat) {
		warpSync.UpdateCurrentConfirmedMilestone(deps.SyncManager.ConfirmedMilestoneIndex())
		warpSync.UpdateTargetMilestone(hb.SolidMilestoneIndex)
	}).Unhook)
}

func unhookHeartbeatEvents(p *gossip.Protocol) {
	heartbeatsHooksMutex.Lock()
	defer heartbeatsHooksMutex.Unlock()

	if unhook, exists := heartbeatHooks.Get(p.PeerID); exists {
		unhook()
		heartbeatHooks.Delete(p.PeerID)
	}
}

func unhookAllHeartbeatEvents() {
	heartbeatsHooksMutex.Lock()
	defer heartbeatsHooksMutex.Unlock()

	heartbeatHooks.ForEach(func(id peer.ID, unhook func()) bool {
		unhook()
		heartbeatHooks.Delete(id)
		return true
	})
}

func hookEvents() (detach func()) {
	return lo.Batch(
		deps.GossipService.Events.ProtocolStarted.Hook(hookHeartbeatEvents).Unhook,
		deps.GossipService.Events.ProtocolTerminated.Hook(unhookHeartbeatEvents).Unhook,
		unhookAllHeartbeatEvents,

		deps.Tangle.Events.ReferencedBlocksCountUpdated.Hook(func(msIndex iotago.MilestoneIndex, referencedBlocksCount int) {
			warpSync.AddReferencedBlocksCount(referencedBlocksCount)
			warpSync.UpdateCurrentConfirmedMilestone(msIndex)
		}).Unhook,

		deps.Tangle.Events.MilestoneSolidificationFailed.Hook(func(msIndex iotago.MilestoneIndex) {
			if warpSync.CurrentCheckpoint != 0 && warpSync.CurrentCheckpoint < msIndex {
				// rerequest since milestone requests could have been lost
				_, msIndexStart, msIndexEnd := warpSyncMilestoneRequester.RequestMilestoneRange(warpSync.AdvancementRange)
				Component.LogInfof("Requesting missing milestones %d - %d", msIndexStart, msIndexEnd)
			}
		}).Unhook,

		warpSync.Events.CheckpointUpdated.Hook(func(nextCheckpoint iotago.MilestoneIndex, oldCheckpoint iotago.MilestoneIndex, advRange syncmanager.MilestoneIndexDelta, target iotago.MilestoneIndex) {
			Component.LogInfof("Checkpoint updated to milestone %d (target %d)", nextCheckpoint, target)
			// prevent any requests in the queue above our next checkpoint
			deps.RequestQueue.Filter(func(r *gossip.Request) bool {
				return r.MilestoneIndex <= nextCheckpoint
			})
			_, _, _ = warpSyncMilestoneRequester.RequestMilestoneRange(advRange, oldCheckpoint)
		}).Unhook,

		warpSync.Events.TargetUpdated.Hook(func(checkpoint iotago.MilestoneIndex, newTarget iotago.MilestoneIndex) {
			Component.LogInfof("Target updated to milestone %d (checkpoint %d)", newTarget, checkpoint)
		}).Unhook,

		warpSync.Events.Start.Hook(func(targetMsIndex iotago.MilestoneIndex, nextCheckpoint iotago.MilestoneIndex, advRange syncmanager.MilestoneIndexDelta) {
			Component.LogInfof("Synchronizing to milestone %d", targetMsIndex)
			deps.RequestQueue.Filter(func(r *gossip.Request) bool {
				return r.MilestoneIndex <= nextCheckpoint
			})

			msRequested, _, _ := warpSyncMilestoneRequester.RequestMilestoneRange(advRange)
			// if the amount of requested milestones doesn't correspond to the range,
			// it means we already had the milestones in the database, which suggests
			// that we should manually kick start the milestone solidifier.
			if msRequested != advRange {
				Component.LogInfo("Manually starting solidifier, as some milestones are already in the database")
				deps.Tangle.TriggerSolidifier()
			}
		}).Unhook,

		warpSync.Events.Done.Hook(func(deltaSynced int, referencedBlocksTotal int, took time.Duration) {
			// we need to cleanup all memoized things in the requester, so we have a clean state at next run and free the memory.
			// we can only reset the "traversed" blocks here, because otherwise it may happen that the requester always
			// walks the whole cone if there are already paths between newer milestones in the database.
			warpSyncMilestoneRequester.Cleanup()

			Component.LogInfof("Synchronized %d milestones in %v (%0.2f BPS)", deltaSynced, took.Truncate(time.Millisecond), float64(referencedBlocksTotal)/took.Seconds())
			deps.RequestQueue.Filter(nil)
		}).Unhook,
	)
}
