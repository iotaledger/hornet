package urts

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hornet/pkg/common"
	"github.com/iotaledger/hornet/pkg/metrics"
	"github.com/iotaledger/hornet/pkg/model/storage"
	"github.com/iotaledger/hornet/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/pkg/node"
	"github.com/iotaledger/hornet/pkg/shutdown"
	"github.com/iotaledger/hornet/pkg/tangle"
	"github.com/iotaledger/hornet/pkg/tipselect"
	"github.com/iotaledger/hornet/pkg/whiteflag"
)

func init() {
	Plugin = &node.Plugin{
		Status: node.StatusEnabled,
		Pluggable: node.Pluggable{
			Name:           "URTS",
			DepsFunc:       func(cDeps dependencies) { deps = cDeps },
			Params:         params,
			InitConfigPars: initConfigPars,
			Provide:        provide,
			Configure:      configure,
			Run:            run,
		},
	}
}

var (
	Plugin *node.Plugin
	deps   dependencies

	// closures
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

func initConfigPars(c *dig.Container) {

	type cfgDeps struct {
		dig.In
		NodeConfig *configuration.Configuration `name:"nodeConfig"`
	}

	type cfgResult struct {
		dig.Out
		MaxDeltaMsgYoungestConeRootIndexToCMI int `name:"maxDeltaMsgYoungestConeRootIndexToCMI"`
		MaxDeltaMsgOldestConeRootIndexToCMI   int `name:"maxDeltaMsgOldestConeRootIndexToCMI"`
		BelowMaxDepth                         int `name:"belowMaxDepth"`
	}

	if err := c.Provide(func(deps cfgDeps) cfgResult {
		return cfgResult{
			MaxDeltaMsgYoungestConeRootIndexToCMI: deps.NodeConfig.Int(CfgTipSelMaxDeltaMsgYoungestConeRootIndexToCMI),
			MaxDeltaMsgOldestConeRootIndexToCMI:   deps.NodeConfig.Int(CfgTipSelMaxDeltaMsgOldestConeRootIndexToCMI),
			BelowMaxDepth:                         deps.NodeConfig.Int(CfgTipSelBelowMaxDepth),
		}
	}); err != nil {
		Plugin.LogPanic(err)
	}
}

func provide(c *dig.Container) {

	type tipselDeps struct {
		dig.In
		Storage                               *storage.Storage
		SyncManager                           *syncmanager.SyncManager
		ServerMetrics                         *metrics.ServerMetrics
		NodeConfig                            *configuration.Configuration `name:"nodeConfig"`
		MaxDeltaMsgYoungestConeRootIndexToCMI int                          `name:"maxDeltaMsgYoungestConeRootIndexToCMI"`
		MaxDeltaMsgOldestConeRootIndexToCMI   int                          `name:"maxDeltaMsgOldestConeRootIndexToCMI"`
		BelowMaxDepth                         int                          `name:"belowMaxDepth"`
	}

	if err := c.Provide(func(deps tipselDeps) *tipselect.TipSelector {
		return tipselect.New(
			Plugin.Daemon().ContextStopped(),
			deps.Storage,
			deps.SyncManager,
			deps.ServerMetrics,

			deps.MaxDeltaMsgYoungestConeRootIndexToCMI,
			deps.MaxDeltaMsgOldestConeRootIndexToCMI,
			deps.BelowMaxDepth,

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
