package urts

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/core/app"
	"github.com/iotaledger/hive.go/core/app/pkg/shutdown"
	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/hornet/v2/pkg/daemon"
	"github.com/iotaledger/hornet/v2/pkg/metrics"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/v2/pkg/tangle"
	"github.com/iotaledger/hornet/v2/pkg/tipselect"
	iotago "github.com/iotaledger/iota.go/v3"
)

func init() {
	Plugin = &app.Plugin{
		Component: &app.Component{
			Name:      "URTS",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Provide:   provide,
			Configure: configure,
			Run:       run,
		},
		IsEnabled: func() bool {
			return ParamsTipsel.Enabled
		},
	}
}

var (
	Plugin *app.Plugin
	deps   dependencies

	// closures.
	onBlockSolid                     *events.Closure
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

			ParamsTipsel.SemiLazy.RetentionRulesTipsLimit,
			ParamsTipsel.SemiLazy.MaxReferencedTipAge,
			ParamsTipsel.SemiLazy.MaxChildren,
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
	onBlockSolid = events.NewClosure(func(cachedBlockMeta *storage.CachedMetadata) {
		cachedBlockMeta.ConsumeMetadata(func(metadata *storage.BlockMetadata) { // meta -1
			// do not add tips during syncing, because it is not needed at all
			if !deps.SyncManager.IsNodeAlmostSynced() {
				return
			}

			deps.TipSelector.AddTip(metadata)
		})
	})

	onConfirmedMilestoneIndexChanged = events.NewClosure(func(_ iotago.MilestoneIndex) {
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
	deps.Tangle.Events.BlockSolid.Hook(onBlockSolid)
	deps.Tangle.Events.ConfirmedMilestoneIndexChanged.Hook(onConfirmedMilestoneIndexChanged)
}

func detachEvents() {
	deps.Tangle.Events.BlockSolid.Detach(onBlockSolid)
	deps.Tangle.Events.ConfirmedMilestoneIndexChanged.Detach(onConfirmedMilestoneIndexChanged)
}
