package urts

import (
	"time"

	"github.com/gohornet/hornet/pkg/node"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"

	"github.com/gohornet/hornet/core/database"
	tanglecore "github.com/gohornet/hornet/core/tangle"
	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/pkg/whiteflag"
)

var (
	Plugin *node.Plugin
	log    *logger.Logger

	TipSelector *tipselect.TipSelector

	// Closures
	onMessageSolid       *events.Closure
	onMilestoneConfirmed *events.Closure
)

func init() {
	Plugin = node.NewPlugin("URTS", node.Enabled, configure, run)
}
func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	TipSelector = tipselect.New(
		database.Tangle(),

		config.NodeConfig.Int(config.CfgTipSelMaxDeltaMsgYoungestConeRootIndexToLSMI),
		config.NodeConfig.Int(config.CfgTipSelMaxDeltaMsgOldestConeRootIndexToLSMI),
		config.NodeConfig.Int(config.CfgTipSelBelowMaxDepth),

		config.NodeConfig.Int(config.CfgTipSelNonLazy+config.CfgTipSelRetentionRulesTipsLimit),
		time.Duration(time.Second*time.Duration(config.NodeConfig.Int(config.CfgTipSelNonLazy+config.CfgTipSelMaxReferencedTipAgeSeconds))),
		uint32(config.NodeConfig.Int64(config.CfgTipSelNonLazy+config.CfgTipSelMaxChildren)),
		config.NodeConfig.Int(config.CfgTipSelNonLazy+config.CfgTipSelSpammerTipsThreshold),

		config.NodeConfig.Int(config.CfgTipSelSemiLazy+config.CfgTipSelRetentionRulesTipsLimit),
		time.Duration(time.Second*time.Duration(config.NodeConfig.Int(config.CfgTipSelSemiLazy+config.CfgTipSelMaxReferencedTipAgeSeconds))),
		uint32(config.NodeConfig.Int64(config.CfgTipSelSemiLazy+config.CfgTipSelMaxChildren)),
		config.NodeConfig.Int(config.CfgTipSelSemiLazy+config.CfgTipSelSpammerTipsThreshold),
	)

	configureEvents()
}

func run(_ *node.Plugin) {
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
				removedTipCount := TipSelector.CleanUpReferencedTips()
				log.Debugf("CleanUpReferencedTips finished, removed: %d, took: %v", removedTipCount, time.Since(ts).Truncate(time.Millisecond))
			}
		}
	}, shutdown.PriorityTipselection)
}

func configureEvents() {
	onMessageSolid = events.NewClosure(func(cachedMsgMeta *tangle.CachedMetadata) {
		cachedMsgMeta.ConsumeMetadata(func(metadata *tangle.MessageMetadata) { // metadata -1
			// do not add tips during syncing, because it is not needed at all
			if !database.Tangle().IsNodeSyncedWithThreshold() {
				return
			}

			TipSelector.AddTip(metadata)
		})
	})

	onMilestoneConfirmed = events.NewClosure(func(confirmation *whiteflag.Confirmation) {
		// do not propagate during syncing, because it is not needed at all
		if !database.Tangle().IsNodeSyncedWithThreshold() {
			return
		}

		// propagate new cone root indexes to the future cone for URTS
		ts := time.Now()
		dag.UpdateConeRootIndexes(database.Tangle(), confirmation.Mutations.MessagesReferenced, confirmation.MilestoneIndex)
		log.Debugf("UpdateConeRootIndexes finished, took: %v", time.Since(ts).Truncate(time.Millisecond))

		ts = time.Now()
		removedTipCount := TipSelector.UpdateScores()
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
