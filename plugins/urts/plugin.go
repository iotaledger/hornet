package urts

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/syncmanager"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/pkg/whiteflag"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
)

func init() {
	Plugin = &node.Plugin{
		Status: node.StatusEnabled,
		Pluggable: node.Pluggable{
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
	Plugin *node.Plugin
	deps   dependencies

	// Closures
	onMessageSolid       *events.Closure
	onMilestoneConfirmed *events.Closure
)

type dependencies struct {
	dig.In
	TipSelector     *tipselect.TipSelector
	SyncManager     *syncmanager.SyncManager
	Tangle          *tangle.Tangle
	ShutdownHandler *shutdown.ShutdownHandler
}

func provide(c *dig.Container) {

	type tipselDeps struct {
		dig.In
		TipScoreCalculator *tangle.TipScoreCalculator
		SyncManager        *syncmanager.SyncManager
		ServerMetrics      *metrics.ServerMetrics
		NodeConfig         *configuration.Configuration `name:"nodeConfig"`
	}

	if err := c.Provide(func(deps tipselDeps) *tipselect.TipSelector {
		return tipselect.New(
			Plugin.Daemon().ContextStopped(),
			deps.TipScoreCalculator,
			deps.SyncManager,
			deps.ServerMetrics,

			deps.NodeConfig.Int(CfgTipSelNonLazy+CfgTipSelRetentionRulesTipsLimit),
			deps.NodeConfig.Duration(CfgTipSelNonLazy+CfgTipSelMaxReferencedTipAge),
			uint32(deps.NodeConfig.Int64(CfgTipSelNonLazy+CfgTipSelMaxChildren)),
			deps.NodeConfig.Int(CfgTipSelNonLazy+CfgTipSelSpammerTipsThreshold),

			deps.NodeConfig.Int(CfgTipSelSemiLazy+CfgTipSelRetentionRulesTipsLimit),
			deps.NodeConfig.Duration(CfgTipSelSemiLazy+CfgTipSelMaxReferencedTipAge),
			uint32(deps.NodeConfig.Int64(CfgTipSelSemiLazy+CfgTipSelMaxChildren)),
			deps.NodeConfig.Int(CfgTipSelSemiLazy+CfgTipSelSpammerTipsThreshold),
		)
	}); err != nil {
		Plugin.LogPanic(err)
	}
}

func configure() {
	configureEvents()
}

func run() {

	if err := Plugin.Daemon().BackgroundWorker("Tipselection[Events]", func(ctx context.Context) {
		attachEvents()
		<-ctx.Done()
		detachEvents()
	}, shutdown.PriorityTipselection); err != nil {
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
	}, shutdown.PriorityTipselection); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
	}
}

func configureEvents() {
	onMessageSolid = events.NewClosure(func(cachedMsgMeta *storage.CachedMetadata) {
		cachedMsgMeta.ConsumeMetadata(func(metadata *storage.MessageMetadata) { // meta -1
			// do not add tips during syncing, because it is not needed at all
			if !deps.SyncManager.IsNodeAlmostSynced() {
				return
			}

			deps.TipSelector.AddTip(metadata)
		})
	})

	onMilestoneConfirmed = events.NewClosure(func(_ *whiteflag.Confirmation) {
		// do not update tip scores during syncing, because it is not needed at all
		if !deps.SyncManager.IsNodeAlmostSynced() {
			return
		}

		ts := time.Now()
		removedTipCount, err := deps.TipSelector.UpdateScores()
		if err != nil && err != common.ErrOperationAborted {
			deps.ShutdownHandler.SelfShutdown(fmt.Sprintf("urts tipselection plugin hit a critical error while updating scores: %s", err))
		}
		Plugin.LogDebugf("UpdateScores finished, removed: %d, took: %v", removedTipCount, time.Since(ts).Truncate(time.Millisecond))
	})
}

func attachEvents() {
	deps.Tangle.Events.MessageSolid.Attach(onMessageSolid)
	deps.Tangle.Events.MilestoneConfirmed.Attach(onMilestoneConfirmed)
}

func detachEvents() {
	deps.Tangle.Events.MessageSolid.Detach(onMessageSolid)
	deps.Tangle.Events.MilestoneConfirmed.Detach(onMilestoneConfirmed)
}
