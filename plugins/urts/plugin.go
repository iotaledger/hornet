package urts

import (
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/tipselect"
	tangleplugin "github.com/gohornet/hornet/plugins/tangle"
)

var (
	PLUGIN = node.NewPlugin("URTS", node.Enabled, configure, run)
	log    *logger.Logger

	TipSelector   *tipselect.TipSelector
	wasSyncBefore = false
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	TipSelector = tipselect.New(
		config.NodeConfig.GetInt(config.CfgTipSelMaxDeltaTxYoungestRootSnapshotIndexToLSMI),
		config.NodeConfig.GetInt(config.CfgTipSelMaxDeltaTxApproveesOldestRootSnapshotIndexToLSMI),
		config.NodeConfig.GetInt(config.CfgTipSelBelowMaxDepth),
		time.Duration(time.Second*time.Duration(config.NodeConfig.GetInt(config.CfgTipSelMaxReferencedTipAgeSeconds))),
		config.NodeConfig.GetUint32(config.CfgTipSelMaxApprovers),
	)

	tangleplugin.Events.BundleSolid.Attach(events.NewClosure(func(cachedBndl *tangle.CachedBundle) {
		cachedBndl.ConsumeBundle(func(bndl *tangle.Bundle) { // tx -1
			if !wasSyncBefore {
				if !tangle.IsNodeSyncedWithThreshold() {
					// do not add tips if the node is not synced
					return
				}
				wasSyncBefore = true
			}

			if bndl.IsInvalidPastCone() || !bndl.IsValid() || !bndl.ValidStrictSemantics() {
				// ignore invalid bundles or semantically invalid bundles or bundles with invalid past cone
				return
			}

			TipSelector.AddTip(bndl.GetTailHash())
		})
	}))
}

func run(_ *node.Plugin) {
	// nothing
}
