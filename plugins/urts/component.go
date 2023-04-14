package urts

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/app"
	"github.com/iotaledger/hive.go/app/shutdown"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/hornet/v2/pkg/components"
	"github.com/iotaledger/hornet/v2/pkg/daemon"
	"github.com/iotaledger/hornet/v2/pkg/metrics"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/v2/pkg/tangle"
	"github.com/iotaledger/hornet/v2/pkg/tipselect"
	iotago "github.com/iotaledger/iota.go/v3"
)

func init() {
	Component = &app.Component{
		Name:     "URTS",
		DepsFunc: func(cDeps dependencies) { deps = cDeps },
		Params:   params,
		IsEnabled: func(c *dig.Container) bool {
			// do not enable in "autopeering entry node" mode
			return components.IsAutopeeringEntryNodeDisabled(c) && ParamsTipsel.Enabled
		},
		Provide: provide,
		Run:     run,
	}
}

var (
	Component *app.Component
	deps      dependencies
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
			Component.Daemon().ContextStopped(),
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
		Component.LogPanic(err)
	}

	return nil
}

func run() error {
	if err := Component.Daemon().BackgroundWorker("Tipselection[Events]", func(ctx context.Context) {
		unhook := hookEvents()
		defer unhook()
		<-ctx.Done()
	}, daemon.PriorityTipselection); err != nil {
		Component.LogPanicf("failed to start worker: %s", err)
	}

	if err := Component.Daemon().BackgroundWorker("Tipselection[Cleanup]", func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
				ts := time.Now()
				removedTipCount := deps.TipSelector.CleanUpReferencedTips()
				Component.LogDebugf("CleanUpReferencedTips finished, removed: %d, took: %v", removedTipCount, time.Since(ts).Truncate(time.Millisecond))
			}
		}
	}, daemon.PriorityTipselection); err != nil {
		Component.LogPanicf("failed to start worker: %s", err)
	}

	return nil
}

func hookEvents() (unhook func()) {
	return lo.Batch(
		deps.Tangle.Events.BlockSolid.Hook(func(cachedBlockMeta *storage.CachedMetadata) {
			cachedBlockMeta.ConsumeMetadata(func(metadata *storage.BlockMetadata) { // meta -1
				// do not add tips during syncing, because it is not needed at all
				if !deps.SyncManager.IsNodeAlmostSynced() {
					return
				}

				deps.TipSelector.AddTip(metadata)
			})
		}).Unhook,

		deps.Tangle.Events.ConfirmedMilestoneIndexChanged.Hook(func(_ iotago.MilestoneIndex) {
			// do not update tip scores during syncing, because it is not needed at all
			if !deps.SyncManager.IsNodeAlmostSynced() {
				return
			}

			ts := time.Now()
			removedTipCount, err := deps.TipSelector.UpdateScores()
			if err != nil && err != common.ErrOperationAborted {
				deps.ShutdownHandler.SelfShutdown(fmt.Sprintf("urts tipselection plugin hit a critical error while updating scores: %s", err), true)
			}
			Component.LogDebugf("UpdateScores finished, removed: %d, took: %v", removedTipCount, time.Since(ts).Truncate(time.Millisecond))
		}).Unhook,
	)
}
