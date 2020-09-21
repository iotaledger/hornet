package urts

import (
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/pkg/whiteflag"
	tangleplugin "github.com/gohornet/hornet/plugins/tangle"
)

var (
	PLUGIN = node.NewPlugin("URTS", node.Enabled, configure, run)
	log    *logger.Logger

	TipSelector *tipselect.TipSelector

	// Closures
	onMessageSolid       *events.Closure
	onMilestoneConfirmed *events.Closure
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	TipSelector = tipselect.New(
		config.NodeConfig.GetInt(config.CfgTipSelMaxDeltaMsgYoungestConeRootIndexToLSMI),
		config.NodeConfig.GetInt(config.CfgTipSelMaxDeltaMsgOldestConeRootIndexToLSMI),
		config.NodeConfig.GetInt(config.CfgTipSelBelowMaxDepth),

		config.NodeConfig.GetInt(config.CfgTipSelNonLazy+config.CfgTipSelRetentionRulesTipsLimit),
		time.Duration(time.Second*time.Duration(config.NodeConfig.GetInt(config.CfgTipSelNonLazy+config.CfgTipSelMaxReferencedTipAgeSeconds))),
		config.NodeConfig.GetUint32(config.CfgTipSelNonLazy+config.CfgTipSelMaxChildren),
		config.NodeConfig.GetInt(config.CfgTipSelNonLazy+config.CfgTipSelSpammerTipsThreshold),

		config.NodeConfig.GetInt(config.CfgTipSelSemiLazy+config.CfgTipSelRetentionRulesTipsLimit),
		time.Duration(time.Second*time.Duration(config.NodeConfig.GetInt(config.CfgTipSelSemiLazy+config.CfgTipSelMaxReferencedTipAgeSeconds))),
		config.NodeConfig.GetUint32(config.CfgTipSelSemiLazy+config.CfgTipSelMaxChildren),
		config.NodeConfig.GetInt(config.CfgTipSelSemiLazy+config.CfgTipSelSpammerTipsThreshold),
	)

	configureEvents()
}

func run(_ *node.Plugin) {
	daemon.BackgroundWorker("Tipselection[Events]", func(shutdownSignal <-chan struct{}) {
		attachEvents()
		<-shutdownSignal
		detachEvents()
	}, shutdown.PriorityTipselection)

	daemon.BackgroundWorker("Tipselection[Cleanup]", func(shutdownSignal <-chan struct{}) {
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
		cachedMsgMeta.ConsumeMetadata(func(metadata *hornet.MessageMetadata) { // metadata -1
			// do not add tips during syncing, because it is not needed at all
			if !tangle.IsNodeSyncedWithThreshold() {
				return
			}

			TipSelector.AddTip(metadata)
		})
	})

	onMilestoneConfirmed = events.NewClosure(func(confirmation *whiteflag.Confirmation) {
		// do not propagate during syncing, because it is not needed at all
		if !tangle.IsNodeSyncedWithThreshold() {
			return
		}

		// propagate new cone root indexes to the future cone for URTS
		ts := time.Now()
		dag.UpdateConeRootIndexes(confirmation.Mutations.MessagesReferenced, confirmation.MilestoneIndex)
		log.Debugf("UpdateConeRootIndexes finished, took: %v", time.Since(ts).Truncate(time.Millisecond))

		ts = time.Now()
		removedTipCount := TipSelector.UpdateScores()
		log.Debugf("UpdateScores finished, removed: %d, took: %v", removedTipCount, time.Since(ts).Truncate(time.Millisecond))
	})
}

func attachEvents() {
	tangleplugin.Events.MessageSolid.Attach(onMessageSolid)
	tangleplugin.Events.MilestoneConfirmed.Attach(onMilestoneConfirmed)
}

func detachEvents() {
	tangleplugin.Events.MessageSolid.Detach(onMessageSolid)
	tangleplugin.Events.MilestoneConfirmed.Detach(onMilestoneConfirmed)
}
