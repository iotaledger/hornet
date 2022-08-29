package gossip

import (
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/objectstorage"
	"github.com/iotaledger/hive.go/core/syncutils"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
)

// WorkUnitState defines the state which a WorkUnit is in.
type WorkUnitState byte

const (
	Hashing WorkUnitState = 1 << 0
	Invalid WorkUnitState = 1 << 1
	Hashed  WorkUnitState = 1 << 2
)

// newWorkUnit creates a new WorkUnit and initializes values by unmarshaling key.
func newWorkUnit(key []byte, messageProcessor *MessageProcessor) *WorkUnit {
	wu := &WorkUnit{
		receivedBytes:    make([]byte, len(key)),
		receivedFrom:     make([]*Protocol, 0),
		messageProcessor: messageProcessor,
	}
	copy(wu.receivedBytes, key)

	return wu
}

// CachedWorkUnit represents a cached WorkUnit.
type CachedWorkUnit struct {
	objectstorage.CachedObject
}

// WorkUnit retrieves the work unit, that is cached in this container.
func (c *CachedWorkUnit) WorkUnit() *WorkUnit {
	//nolint:forcetypeassert // we will replace that with generics anyway
	return c.Get().(*WorkUnit)
}

// WorkUnit defines the work around processing a received block and its
// associated requests from peers. There is at most one WorkUnit active per same
// block bytes.
type WorkUnit struct {
	objectstorage.StorableObjectFlags
	processingLock syncutils.Mutex

	messageProcessor *MessageProcessor

	// data
	receivedBytes []byte
	block         *storage.Block
	requested     bool

	// status
	stateLock syncutils.RWMutex
	state     WorkUnitState

	// received from
	receivedFromLock syncutils.RWMutex
	receivedFrom     []*Protocol
}

func (wu *WorkUnit) Update(_ objectstorage.StorableObject) {
	// do nothing, since the object is identical (consists of key only)
}

func (wu *WorkUnit) ObjectStorageKey() []byte {
	return wu.receivedBytes
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
func (wu *WorkUnit) addReceivedFrom(p *Protocol) {
	wu.receivedFromLock.Lock()
	defer wu.receivedFromLock.Unlock()
	wu.receivedFrom = append(wu.receivedFrom, p)
}

// punishes, respectively increases the invalid block metric of all peers
// which sent the given underlying block of this WorkUnit.
// it also closes the connection to these peers.
func (wu *WorkUnit) punish(reason error) {
	wu.receivedFromLock.Lock()
	defer wu.receivedFromLock.Unlock()
	for _, p := range wu.receivedFrom {
		wu.messageProcessor.serverMetrics.InvalidBlocks.Inc()

		// drop the connection to the peer
		_ = wu.messageProcessor.peeringManager.DisconnectPeer(p.PeerID, errors.WithMessagef(reason, "peer was punished"))
	}
}

// builds a Broadcast where all peers which are associated with this WorkUnit are excluded from.
func (wu *WorkUnit) broadcast() *Broadcast {
	wu.receivedFromLock.Lock()
	defer wu.receivedFromLock.Unlock()
	exclude := map[peer.ID]struct{}{}
	for _, p := range wu.receivedFrom {
		exclude[p.PeerID] = struct{}{}
	}

	return &Broadcast{
		Data:         wu.receivedBytes,
		ExcludePeers: exclude,
	}
}

// increases the known block metric of all peers
// except the given peer.
func (wu *WorkUnit) increaseKnownTxCount(excludedPeer *Protocol) {
	wu.receivedFromLock.Lock()
	defer wu.receivedFromLock.Unlock()

	for _, p := range wu.receivedFrom {
		if p.PeerID == excludedPeer.PeerID {
			continue
		}
		wu.messageProcessor.serverMetrics.KnownBlocks.Inc()
		p.Metrics.KnownBlocks.Inc()
	}
}
