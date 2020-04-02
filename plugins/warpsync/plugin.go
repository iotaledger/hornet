package warpsync

import (
	"time"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/peering/peer"
	"github.com/gohornet/hornet/pkg/protocol/rqueue"
	"github.com/gohornet/hornet/pkg/protocol/sting"
	"github.com/gohornet/hornet/pkg/protocol/warpsync"
	"github.com/gohornet/hornet/plugins/gossip"
	peeringplugin "github.com/gohornet/hornet/plugins/peering"
	tangleplugin "github.com/gohornet/hornet/plugins/tangle"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

var (
	PLUGIN   = node.NewPlugin("WarpSync", node.Enabled, configure)
	log      *logger.Logger
	warpSync = warpsync.New(25)
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	peeringplugin.Manager().Events.PeerConnected.Attach(events.NewClosure(func(p *peer.Peer) {

		if !p.Protocol.Supports(sting.FeatureSet) {
			return
		}

		p.Protocol.Events.Received[sting.MessageTypeHeartbeat].Attach(events.NewClosure(func(data []byte) {
			p.LatestHeartbeat = sting.ParseHeartbeat(data)
			warpSync.UpdateCurrent(tangle.GetSolidMilestoneIndex())
			warpSync.UpdateTarget(p.LatestHeartbeat.SolidMilestoneIndex)
		}))
	}))

	tangleplugin.Events.SolidMilestoneChanged.Attach(events.NewClosure(func(cachedMsBundle *tangle.CachedBundle) { // bundle +1
		defer cachedMsBundle.Release() // bundle -1
		index := cachedMsBundle.GetBundle().GetMilestoneIndex()
		warpSync.UpdateCurrent(index)
	}))

	warpSync.Events.CheckpointUpdated.Attach(events.NewClosure(func(nextCheckpoint milestone.Index, oldCheckpoint milestone.Index, advRange int32) {
		log.Infof("Checkpoint updated to milestone %d", nextCheckpoint)
		// prevent any requests in the queue above our next checkpoint
		gossip.RequestQueue().Filter(func(r *rqueue.Request) bool {
			return r.MilestoneIndex <= nextCheckpoint
		})
		gossip.BroadcastMilestoneRequests(int(advRange), gossip.MemoizedRequestMissingMilestoneApprovees(), oldCheckpoint)
	}))

	warpSync.Events.Start.Attach(events.NewClosure(func(targetMsIndex milestone.Index, nextCheckpoint milestone.Index, advRange int32) {
		log.Infof("Synchronizing to milestone %d", targetMsIndex)
		gossip.RequestQueue().Filter(func(r *rqueue.Request) bool {
			return r.MilestoneIndex <= nextCheckpoint
		})
		gossip.BroadcastMilestoneRequests(int(advRange), gossip.MemoizedRequestMissingMilestoneApprovees())
	}))

	warpSync.Events.Done.Attach(events.NewClosure(func(deltaSynced int, took time.Duration) {
		log.Infof("Synchronized %d milestones in %v", deltaSynced, took)
		gossip.RequestQueue().Filter(nil)
	}))
}
