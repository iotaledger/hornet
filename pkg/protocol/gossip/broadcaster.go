package gossip

import (
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/syncmanager"
	"github.com/gohornet/hornet/pkg/p2p"
)

// Broadcaster provides functions to broadcast data to gossip streams.
type Broadcaster struct {
	// used to access the node storage.
	storage *storage.Storage
	// used to determine the sync status of the node.
	syncManager *syncmanager.SyncManager
	// used to access the p2p peeringManager.
	peeringManager *p2p.Manager
	// used to access gossip service.
	service *Service
	// the queue for pending broadcasts.
	queue chan *Broadcast
}

// NewBroadcaster creates a new Broadcaster.
func NewBroadcaster(
	dbStorage *storage.Storage,
	syncManager *syncmanager.SyncManager,
	peeringManager *p2p.Manager,
	service *Service,
	broadcastQueueSize int) *Broadcaster {

	return &Broadcaster{
		storage:        dbStorage,
		syncManager:    syncManager,
		peeringManager: peeringManager,
		service:        service,
		queue:          make(chan *Broadcast, broadcastQueueSize),
	}
}

// RunBroadcastQueueDrainer runs the broadcast queue drainer.
func (b *Broadcaster) RunBroadcastQueueDrainer(shutdownSignal <-chan struct{}) {
exit:
	for {
		select {
		case <-shutdownSignal:
			break exit
		case broadcast := <-b.queue:
			b.service.ForEach(func(proto *Protocol) bool {
				if _, excluded := broadcast.ExcludePeers[proto.PeerID]; excluded {
					return true
				}

				proto.SendMessage(broadcast.MsgData)
				return true
			})
		}
	}
}

// Broadcast broadcasts the given Broadcast.
func (b *Broadcaster) Broadcast(broadcast *Broadcast) {
	b.queue <- broadcast
}

// BroadcastHeartbeat broadcasts a heartbeat message to every peer.
func (b *Broadcaster) BroadcastHeartbeat(filter func(proto *Protocol) bool) {
	snapshotInfo := b.storage.SnapshotInfo()
	if snapshotInfo == nil {
		return
	}

	confirmedMilestoneIndex := b.syncManager.ConfirmedMilestoneIndex() // bee differentiates between solid and confirmed milestone, for hornet it is the same.
	connectedCount := b.peeringManager.ConnectedCount(p2p.PeerRelationKnown)
	syncedCount := b.service.SynchronizedCount(confirmedMilestoneIndex)
	// TODO: overflow not handled for synced/connected

	heartbeatMsg, err := NewHeartbeatMsg(confirmedMilestoneIndex, snapshotInfo.PruningIndex, b.syncManager.LatestMilestoneIndex(), byte(connectedCount), byte(syncedCount))
	if err != nil {
		return
	}

	b.service.ForEach(func(proto *Protocol) bool {
		if filter != nil && !filter(proto) {
			return true
		}
		proto.Enqueue(heartbeatMsg)
		return true
	})
}

// BroadcastMilestoneRequests broadcasts up to N requests for milestones nearest to the current confirmed milestone index
// to every connected peer. Returns the number of milestones requested.
func (b *Broadcaster) BroadcastMilestoneRequests(rangeToRequest int, onExistingMilestoneInRange func(milestone *storage.CachedMilestone), from ...milestone.Index) int {
	var requested int

	// make sure we only request what we don't have
	startingPoint := b.syncManager.ConfirmedMilestoneIndex()
	if len(from) > 0 {
		startingPoint = from[0]
	}

	var msIndexes []milestone.Index
	for i := 1; i <= rangeToRequest; i++ {
		toReq := startingPoint + milestone.Index(i)

		cachedMs := b.storage.CachedMilestoneOrNil(toReq) // milestone +1
		if cachedMs == nil {
			// only request if we do not have the milestone
			requested++
			msIndexes = append(msIndexes, toReq)
			continue
		}

		// milestone already exists
		if onExistingMilestoneInRange != nil {
			onExistingMilestoneInRange(cachedMs.Retain())
		}

		cachedMs.Release(true) // milestone -1
	}

	if len(msIndexes) == 0 {
		return requested
	}

	// send each ms request to a random peer who supports the message
	for _, msIndex := range msIndexes {
		b.service.ForEach(func(proto *Protocol) bool {
			if !proto.HasDataForMilestone(msIndex) {
				return true
			}
			proto.SendMilestoneRequest(msIndex)
			return false
		})
	}
	return requested
}
