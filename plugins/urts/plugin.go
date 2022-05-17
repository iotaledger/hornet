package urts

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/daemon"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/syncmanager"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hive.go/app/core/shutdown"
	"github.com/iotaledger/hive.go/events"
)

func init() {
	Plugin = &app.Plugin{
		Status: app.StatusEnabled,
		Component: &app.Component{
			Name:      "URTS",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Provide:   provide,
			Configure: configure,
			Run:       run,
		},
	}
}

var (
	Plugin *app.Plugin
	deps   dependencies

	// closures
	onMessageSolid                   *events.Closure
	onConfirmedMilestoneIndexChanged *events.Closure
)

type dependencies struct {
	dig.In
	TipSelector     *tipselect.TipSelector
	SyncManager     *syncmanager.SyncManager
	Tangle          *tangle.Tangle
	ShutdownHandler *shutdown.ShutdownHandler
}

func provide(c *dig.Container) error {

	type tipselDeps struct {
		dig.In
		TipScoreCalculator *tangle.TipScoreCalculator
		SyncManager        *syncmanager.SyncManager
		ServerMetrics      *metrics.ServerMetrics
	}

	if err := c.Provide(func(deps tipselDeps) *tipselect.TipSelector {
		return tipselect.New(
			Plugin.Daemon().ContextStopped(),
			deps.TipScoreCalculator,
			deps.SyncManager,
			deps.ServerMetrics,

			ParamsTipsel.NonLazy.RetentionRulesTipsLimit,
			ParamsTipsel.NonLazy.MaxReferencedTipAge,
			ParamsTipsel.NonLazy.MaxChildren,
			ParamsTipsel.NonLazy.SpammerTipsThreshold,

			ParamsTipsel.SemiLazy.RetentionRulesTipsLimit,
			ParamsTipsel.SemiLazy.MaxReferencedTipAge,
			ParamsTipsel.SemiLazy.MaxChildren,
			ParamsTipsel.SemiLazy.SpammerTipsThreshold,
		)
	}); err != nil {
		Plugin.LogPanic(err)
	}
	return nil
}

func configure() error {
	configureEvents()
	return nil
}

func run() error {

	if err := Plugin.Daemon().BackgroundWorker("Tipselection[Events]", func(ctx context.Context) {
		attachEvents()
		<-ctx.Done()
		detachEvents()
	}, daemon.PriorityTipselection); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
	}

	if err := Plugin.Daemon().BackgroundWorker("Tipselection[Cleanup]", func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
				ts := time.Now()
				removedTipCount := deps.TipSelector.CleanUpReferencedTips()
				Plugin.LogDebugf("CleanUpReferencedTips finished, removed: %d, took: %v", removedTipCount, time.Since(ts).Truncate(time.Millisecond))
			}
		}
	}, daemon.PriorityTipselection); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
	}
	return nil
}

func configureEvents() {
	onMessageSolid = events.NewClosure(func(cachedBlockMeta *storage.CachedMetadata) {
		cachedBlockMeta.ConsumeMetadata(func(metadata *storage.BlockMetadata) { // meta -1
			// do not add tips during syncing, because it is not needed at all
			if !deps.SyncManager.IsNodeAlmostSynced() {
				return
			}

			deps.TipSelector.AddTip(metadata)
		})
	})

	onConfirmedMilestoneIndexChanged = events.NewClosure(func(_ milestone.Index) {
		// do not update tip scores during syncing, because it is not needed at all
		if !deps.SyncManager.IsNodeAlmostSynced() {
			return
		}

		ts := time.Now()
		removedTipCount, err := deps.TipSelector.UpdateScores()
		if err != nil && err != common.ErrOperationAborted {
			deps.ShutdownHandler.SelfShutdown(fmt.Sprintf("urts tipselection plugin hit a critical error while updating scores: %s", err), true)
		}
		Plugin.LogDebugf("UpdateScores finished, removed: %d, took: %v", removedTipCount, time.Since(ts).Truncate(time.Millisecond))
	})
}

func attachEvents() {
	deps.Tangle.Events.BlockSolid.Attach(onMessageSolid)
	deps.Tangle.Events.ConfirmedMilestoneIndexChanged.Attach(onConfirmedMilestoneIndexChanged)
}

func detachEvents() {
	deps.Tangle.Events.BlockSolid.Detach(onMessageSolid)
	deps.Tangle.Events.ConfirmedMilestoneIndexChanged.Detach(onConfirmedMilestoneIndexChanged)
}
