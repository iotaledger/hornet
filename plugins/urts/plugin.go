package urts

import (
	"time"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"go.uber.org/dig"

	tanglecore "github.com/gohornet/hornet/core/tangle"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/pkg/whiteflag"
)

var (
	Plugin *node.Plugin
	log    *logger.Logger

	// Closures
	onMessageSolid       *events.Closure
	onMilestoneConfirmed *events.Closure

	deps dependencies
)

type dependencies struct {
	dig.In
	TipSelector *tipselect.TipSelector
	Tangle *tangle.Tangle
}

func init() {
	Plugin = node.NewPlugin("URTS", node.Enabled, configure, run)
	Plugin.Events.Init.Attach(events.NewClosure(func(c *dig.Container) {
		type tipseldeps struct {
			dig.In

			Tangle     *tangle.Tangle
			NodeConfig *configuration.Configuration `name:"nodeConfig"`
		}

		if err := c.Provide(func(deps tipseldeps) *tipselect.TipSelector {
			return tipselect.New(
				deps.Tangle,

				deps.NodeConfig.Int(config.CfgTipSelMaxDeltaMsgYoungestConeRootIndexToLSMI),
				deps.NodeConfig.Int(config.CfgTipSelMaxDeltaMsgOldestConeRootIndexToLSMI),
				deps.NodeConfig.Int(config.CfgTipSelBelowMaxDepth),

				deps.NodeConfig.Int(config.CfgTipSelNonLazy+config.CfgTipSelRetentionRulesTipsLimit),
				time.Second*time.Duration(deps.NodeConfig.Int(config.CfgTipSelNonLazy+config.CfgTipSelMaxReferencedTipAgeSeconds)),
				uint32(deps.NodeConfig.Int64(config.CfgTipSelNonLazy+config.CfgTipSelMaxChildren)),
				deps.NodeConfig.Int(config.CfgTipSelNonLazy+config.CfgTipSelSpammerTipsThreshold),

				deps.NodeConfig.Int(config.CfgTipSelSemiLazy+config.CfgTipSelRetentionRulesTipsLimit),
				time.Second*time.Duration(deps.NodeConfig.Int(config.CfgTipSelSemiLazy+config.CfgTipSelMaxReferencedTipAgeSeconds)),
				uint32(deps.NodeConfig.Int64(config.CfgTipSelSemiLazy+config.CfgTipSelMaxChildren)),
				deps.NodeConfig.Int(config.CfgTipSelSemiLazy+config.CfgTipSelSpammerTipsThreshold),
			)
		}); err != nil {
			panic(err)
		}
	}))
}
func configure(c *dig.Container) {
	log = logger.NewLogger(Plugin.Name)

	if err := c.Invoke(func(cDeps dependencies) {
		deps = cDeps
	}); err != nil {
		panic(err)
	}

	configureEvents()
}

func run(_ *dig.Container) {
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
	onMessageSolid = events.NewClosure(func(cachedMsgMeta *tangle.CachedMetadata) {
		cachedMsgMeta.ConsumeMetadata(func(metadata *tangle.MessageMetadata) { // metadata -1
			// do not add tips during syncing, because it is not needed at all
			if !deps.Tangle.IsNodeSyncedWithThreshold() {
				return
			}

			deps.TipSelector.AddTip(metadata)
		})
	})

	onMilestoneConfirmed = events.NewClosure(func(confirmation *whiteflag.Confirmation) {
		// do not propagate during syncing, because it is not needed at all
		if !deps.Tangle.IsNodeSyncedWithThreshold() {
			return
		}

		// propagate new cone root indexes to the future cone for URTS
		ts := time.Now()
		dag.UpdateConeRootIndexes(deps.Tangle, confirmation.Mutations.MessagesReferenced, confirmation.MilestoneIndex)
		log.Debugf("UpdateConeRootIndexes finished, took: %v", time.Since(ts).Truncate(time.Millisecond))

		ts = time.Now()
		removedTipCount := deps.TipSelector.UpdateScores()
		log.Debugf("UpdateScores finished, removed: %d, took: %v", removedTipCount, time.Since(ts).Truncate(time.Millisecond))
	})
}

func attachEvents() {
	tanglecore.Events.MessageSolid.Attach(onMessageSolid)
	tanglecore.Events.MilestoneConfirmed.Attach(onMilestoneConfirmed)
}

func detachEvents() {
	tanglecore.Events.MessageSolid.Detach(onMessageSolid)
	tanglecore.Events.MilestoneConfirmed.Detach(onMilestoneConfirmed)
}
