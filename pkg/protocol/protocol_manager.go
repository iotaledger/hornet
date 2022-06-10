package protocol

import (
	"fmt"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hornet/pkg/model/milestone"
	"github.com/iotaledger/hornet/pkg/model/storage"
	"github.com/iotaledger/hornet/pkg/model/syncmanager"
	"github.com/iotaledger/iota.go/v3"
	"sync"
)

var (
	supportedVersions = Versions{2} // make sure to add the versions sorted asc
)

// Versions is a slice of protocol versions.
type Versions []uint32

// Highest returns the highest version.
func (ver Versions) Highest() byte {
	return byte(ver[len(ver)-1])
}

// Lowest returns the lowest version.
func (ver Versions) Lowest() byte {
	return byte(ver[0])
}

// Supports tells whether the given version is supported.
func (ver Versions) Supports(v byte) bool {
	for _, value := range ver {
		if value == uint32(v) {
			return true
		}
	}
	return false
}

// NewManager creates a new Manager.
func NewManager(storage *storage.Storage, initProtoParas *iotago.ProtocolParameters) *Manager {
	return &Manager{
		Events: &Events{
			NextMilestoneUnsupported: events.NewEvent(protoParasMsOptCaller),
			CriticalErrors:           events.NewEvent(events.ErrorCaller),
		},
		storage: storage,
		current: initProtoParas,
		pending: nil,
	}
}

func protoParasMsOptCaller(handler interface{}, params ...interface{}) {
	handler.(func(protoParas *iotago.ProtocolParamsMilestoneOpt))(params[0].(*iotago.ProtocolParamsMilestoneOpt))
}

// Events are events happening around the Manager.
type Events struct {
	// Emits protocol parameters for the unsupported milestone one milestone before.
	NextMilestoneUnsupported *events.Event
	// Emits critical errors.
	CriticalErrors *events.Event
}

// Manager handles the knowledge about current, pending and supported protocol versions and parameters.
type Manager struct {
	// Events holds the events happening within the Manager.
	Events      *Events
	storage     *storage.Storage
	syncManager *syncmanager.SyncManager
	currentRWMu sync.RWMutex
	current     *iotago.ProtocolParameters
	pendingRWMu sync.RWMutex
	pending     []*iotago.ProtocolParamsMilestoneOpt
}

// Init initialises the Manager by loading the last stored or persisting the parameters passed in via constructor.
func (m *Manager) Init() error {
	m.currentRWMu.Lock()
	defer m.currentRWMu.Unlock()

	currentProtoParas, err := m.storage.ProtocolParameters()
	if err != nil {
		return err
	}

	// aka store therefore what the manager was initialised with
	if currentProtoParas == nil {
		if err := m.storage.StoreProtocolParameters(m.current); err != nil {
			return fmt.Errorf("unable to persist init protocol parameters: %w", err)
		}
		return nil
	}

	m.current = currentProtoParas

	return nil
}

// LoadPending examines the database back to below max depth for pending protocol parameter changes.
func (m *Manager) LoadPending(syncManager *syncmanager.SyncManager) {
	m.pendingRWMu.Lock()
	defer m.pendingRWMu.Unlock()

	// examine below max depth milestone to reconstruct pending protocol changes
	confMsIndex := syncManager.ConfirmedMilestoneIndex()
	belowMaxDepthTarget := confMsIndex - milestone.Index(m.current.BelowMaxDepth)
	for i := belowMaxDepthTarget; i < confMsIndex; i-- {
		if protoParasMsOpt := m.readProtocolParasFromMilestone(i); protoParasMsOpt != nil {
			m.pending = append(m.pending, protoParasMsOpt)
		}
	}
}

func (m *Manager) readProtocolParasFromMilestone(index milestone.Index) *iotago.ProtocolParamsMilestoneOpt {
	cachedMs := m.storage.CachedMilestoneByIndexOrNil(index)
	defer cachedMs.Release(true)
	if cachedMs == nil {
		return nil
	}
	return cachedMs.Milestone().Milestone().Opts.MustSet().ProtocolParams()
}

// SupportedVersions returns a slice of supported protocol versions.
func (m *Manager) SupportedVersions() Versions {
	return supportedVersions
}

// Current returns the current protocol parameters under which the node is operating.
func (m *Manager) Current() *iotago.ProtocolParameters {
	m.currentRWMu.RLock()
	defer m.currentRWMu.RUnlock()
	return m.current
}

// Pending returns the currently pending protocol changes.
func (m *Manager) Pending() []*iotago.ProtocolParamsMilestoneOpt {
	m.pendingRWMu.RLock()
	defer m.pendingRWMu.RUnlock()
	cpy := make([]*iotago.ProtocolParamsMilestoneOpt, len(m.pending))
	for i, ele := range m.pending {
		cpy[i] = ele.Clone().(*iotago.ProtocolParamsMilestoneOpt)
	}
	return cpy
}

// NextPendingSupported tells whether the next pending protocol parameters changes are supported.
func (m *Manager) NextPendingSupported() bool {
	m.pendingRWMu.RLock()
	defer m.pendingRWMu.RUnlock()
	if len(m.pending) == 0 {
		return true
	}
	return m.SupportedVersions().Supports(m.pending[0].ProtocolVersion)
}

// HandleConfirmedMilestone examines the newly confirmed milestone for protocol parameter changes.
func (m *Manager) HandleConfirmedMilestone(cachedMilestone *storage.CachedMilestone) {
	defer cachedMilestone.Release(true) // milestone -1
	ms := cachedMilestone.Milestone()

	if msProtoParas := ms.Milestone().Opts.MustSet().ProtocolParams(); msProtoParas != nil {
		m.pendingRWMu.Lock()
		m.pending = append(m.pending, msProtoParas)
		m.pendingRWMu.Unlock()
	}

	if !m.currentShouldChange(ms) {
		return
	}

	if err := m.updateCurrent(); err != nil {
		m.Events.CriticalErrors.Trigger(err)
		return
	}
}

// checks whether the current protocol parameters need to be updated.
func (m *Manager) currentShouldChange(milestone *storage.Milestone) bool {
	m.pendingRWMu.RLock()
	defer m.pendingRWMu.RUnlock()
	if len(m.pending) == 0 {
		return false
	}

	next := m.pending[0]

	switch {
	case next.TargetMilestoneIndex == milestone.Milestone().Index+1:
		if !m.SupportedVersions().Supports(next.ProtocolVersion) {
			m.Events.NextMilestoneUnsupported.Trigger(next)
		}
		return false
	case next.TargetMilestoneIndex > milestone.Milestone().Index:
		return false
	default:
		return true
	}
}

func (m *Manager) updateCurrent() error {
	m.currentRWMu.Lock()
	defer m.currentRWMu.Unlock()
	m.pendingRWMu.Lock()
	defer m.pendingRWMu.Unlock()

	nextMsProtoParamOpt := m.pending[0]
	nextParams := nextMsProtoParamOpt.Params

	// TODO: needs to be adapted for when protocol parameters struct changes
	nextProtoParams := &iotago.ProtocolParameters{}
	if _, err := nextProtoParams.Deserialize(nextParams, serializer.DeSeriModePerformValidation, nil); err != nil {
		return fmt.Errorf("unable to deserialize next protocol parameters: %w", err)
	}

	m.current = nextProtoParams
	m.pending = m.pending[1:]

	if err := m.storage.StoreProtocolParameters(m.current); err != nil {
		return fmt.Errorf("unable to persist new protocol parameters: %w", err)
	}

	return nil
}
