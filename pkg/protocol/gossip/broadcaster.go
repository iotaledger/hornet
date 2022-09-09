package gossip

import (
	"context"

	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/v2/pkg/p2p"
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
func (b *Broadcaster) RunBroadcastQueueDrainer(ctx context.Context) {
exit:
	for {
		select {
		case <-ctx.Done():
			break exit
		case broadcast := <-b.queue:
			b.service.ForEach(func(proto *Protocol) bool {
				if _, excluded := broadcast.ExcludePeers[proto.PeerID]; excluded {
					return true
				}

				proto.SendBlock(broadcast.Data)

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

	syncState := b.syncManager.SyncState()
	connectedCount := b.peeringManager.ConnectedCount()

	// TODO: overflow not handled for synced/connected
	syncedCount := b.service.SynchronizedCount(syncState.ConfirmedMilestoneIndex)

	heartbeatMsg, err := newHeartbeatMessage(syncState.ConfirmedMilestoneIndex, snapshotInfo.PruningIndex(), syncState.LatestMilestoneIndex, byte(connectedCount), byte(syncedCount))
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
