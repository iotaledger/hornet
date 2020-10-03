package processor

import (
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/peering/peer"
	"github.com/gohornet/hornet/pkg/protocol/bqueue"
	"github.com/gohornet/hornet/plugins/peering"
)

// WorkUnitState defines the state which a WorkUnit is in.
type WorkUnitState byte

const (
	Hashing WorkUnitState = 1 << 0
	Invalid WorkUnitState = 1 << 1
	Hashed  WorkUnitState = 1 << 2
)

// newWorkUnit creates a new WorkUnit and initializes values by unmarshalling key.
func newWorkUnit(key []byte) *WorkUnit {
	wu := &WorkUnit{
		receivedMsgBytes: make([]byte, len(key)),
		receivedFrom:     make([]*peer.Peer, 0),
	}
	copy(wu.receivedMsgBytes, key)
	return wu
}

// defines the factory function for WorkUnits.
func workUnitFactory(key []byte, data []byte) (objectstorage.StorableObject, error) {
	return newWorkUnit(key), nil
}

// CachedWorkUnit represents a cached WorkUnit.
type CachedWorkUnit struct {
	objectstorage.CachedObject
}

// WorkUnit gets the underlying WorkUnit.
func (c *CachedWorkUnit) WorkUnit() *WorkUnit {
	return c.Get().(*WorkUnit)
}

// WorkUnit defines the work around processing a received message and its
// associated requests from peers. There is at most one WorkUnit active per same
// message bytes.
type WorkUnit struct {
	objectstorage.StorableObjectFlags
	processingLock syncutils.Mutex

	// data
	dataLock         syncutils.RWMutex
	receivedMsgBytes []byte
	msg              *tangle.Message

	// status
	stateLock syncutils.RWMutex
	state     WorkUnitState

	// received from
	receivedFromLock syncutils.RWMutex
	receivedFrom     []*peer.Peer
}

func (wu *WorkUnit) Update(_ objectstorage.StorableObject) {
	panic("WorkUnit should never be updated")
}

func (wu *WorkUnit) ObjectStorageKey() []byte {
	return wu.receivedMsgBytes
}

func (wu *WorkUnit) ObjectStorageValue() []byte {
	return nil
}

// UpdateState updates the WorkUnit's state.
func (wu *WorkUnit) UpdateState(state WorkUnitState) {
	wu.stateLock.Lock()
	wu.state = 0
	wu.state |= state
	wu.stateLock.Unlock()
}

// Is tells whether the WorkUnit has the given state.
func (wu *WorkUnit) Is(state WorkUnitState) bool {
	wu.stateLock.Lock()
	defer wu.stateLock.Unlock()
	return wu.state&state > 0
}

// adds a Request for the given peer to this WorkUnit.
// requestedMessageID can be nil to flag that this request just reflects a receive from the given
// peer and has no associated request.
func (wu *WorkUnit) addReceivedFrom(p *peer.Peer, requestedMessageID *hornet.MessageID) {
	wu.receivedFromLock.Lock()
	defer wu.receivedFromLock.Unlock()
	wu.receivedFrom = append(wu.receivedFrom, p)
}

// punishes, respectively increases the invalid message metric of all peers
// which sent the given underlying message of this WorkUnit.
// it also closes the connection to these peers.
func (wu *WorkUnit) punish() {
	wu.receivedFromLock.Lock()
	defer wu.receivedFromLock.Unlock()
	for _, p := range wu.receivedFrom {
		metrics.SharedServerMetrics.InvalidMessages.Inc()

		// drop the connection to the peer
		peering.Manager().Remove(p.ID)
	}
}

// builds a Broadcast where all peers which are associated with this WorkUnit are excluded from.
func (wu *WorkUnit) broadcast() *bqueue.Broadcast {
	wu.receivedFromLock.Lock()
	defer wu.receivedFromLock.Unlock()
	exclude := map[string]struct{}{}
	for _, p := range wu.receivedFrom {
		exclude[p.ID] = struct{}{}
	}
	return &bqueue.Broadcast{
		MsgData:      wu.receivedMsgBytes,
		ExcludePeers: exclude,
	}
}

// increases the known message metric of all peers
// except the given peer
func (wu *WorkUnit) increaseKnownTxCount(excludedPeer *peer.Peer) {
	wu.receivedFromLock.Lock()
	defer wu.receivedFromLock.Unlock()

	for _, p := range wu.receivedFrom {
		if p.ID == excludedPeer.ID {
			continue
		}
		metrics.SharedServerMetrics.KnownMessages.Inc()
		p.Metrics.KnownMessages.Inc()
	}
}
