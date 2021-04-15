package urts

import (
	"time"

	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/pkg/whiteflag"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
)

func init() {
	Plugin = &node.Plugin{
		Status: node.Enabled,
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
	log    *logger.Logger
	deps   dependencies

	// Closures
	onMessageSolid       *events.Closure
	onMilestoneConfirmed *events.Closure
)

type dependencies struct {
	dig.In
	TipSelector *tipselect.TipSelector
	Storage     *storage.Storage
	Tangle      *tangle.Tangle
}

func provide(c *dig.Container) {
	type tipseldeps struct {
		dig.In
		Storage       *storage.Storage
		ServerMetrics *metrics.ServerMetrics
		NodeConfig    *configuration.Configuration `name:"nodeConfig"`
		BelowMaxDepth int                          `name:"belowMaxDepth"`
	}

	if err := c.Provide(func(deps tipseldeps) *tipselect.TipSelector {
		return tipselect.New(
			deps.Storage,
			deps.ServerMetrics,

			deps.NodeConfig.Int(CfgTipSelMaxDeltaMsgYoungestConeRootIndexToCMI),
			deps.NodeConfig.Int(CfgTipSelMaxDeltaMsgOldestConeRootIndexToCMI),
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
		panic(err)
	}
}

func configure() {
	log = logger.NewLogger(Plugin.Name)
	configureEvents()
}

func run() {
	Plugin.Daemon().BackgroundWorker("Tipselection[Events]", func(shutdownSignal <-chan struct{}) {
		attachEvents()
		<-shutdownSignal
		detachEvents()
	}, shutdown.PriorityTipselection)

	Plugin.Daemon().BackgroundWorker("Tipselection[Cleanup]", func(shutdownSignal <-chan struct{}) {
		for {
			select {
			case <-shutdownSignal:
				return
			case <-time.After(time.Second):
				ts := time.Now()
				removedTipCount := deps.TipSelector.CleanUpReferencedTips()
				log.Debugf("CleanUpReferencedTips finished, removed: %d, took: %v", removedTipCount, time.Since(ts).Truncate(time.Millisecond))
			}
		}
	}, shutdown.PriorityTipselection)
}

func configureEvents() {
	onMessageSolid = events.NewClosure(func(cachedMsgMeta *storage.CachedMetadata) {
		cachedMsgMeta.ConsumeMetadata(func(metadata *storage.MessageMetadata) { // metadata -1
			// do not add tips during syncing, because it is not needed at all
			if !deps.Storage.IsNodeAlmostSynced() {
				return
			}

			deps.TipSelector.AddTip(metadata)
		})
	})

	onMilestoneConfirmed = events.NewClosure(func(confirmation *whiteflag.Confirmation) {
		// do not update tip scores during syncing, because it is not needed at all
		if !deps.Storage.IsNodeAlmostSynced() {
			return
		}

		ts := time.Now()
		removedTipCount := deps.TipSelector.UpdateScores()
		log.Debugf("UpdateScores finished, removed: %d, took: %v", removedTipCount, time.Since(ts).Truncate(time.Millisecond))
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
