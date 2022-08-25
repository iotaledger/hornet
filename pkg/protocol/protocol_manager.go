package protocol

import (
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	iotago "github.com/iotaledger/iota.go/v3"
)

func protoParamsMsOptionCaller(handler interface{}, params ...interface{}) {
	//nolint:forcetypeassert // we will replace that with generic events anyway
	handler.(func(protoParamsMsOption *iotago.ProtocolParamsMilestoneOpt))(params[0].(*iotago.ProtocolParamsMilestoneOpt))
}

// Events are events happening around the Manager.
type Events struct {
	// Emits a protocol parameters milestone option for the unsupported milestone one milestone before.
	NextMilestoneUnsupported *events.Event
	// Emits critical errors.
	CriticalErrors *events.Event
}

// NewManager creates a new Manager.
func NewManager(storage *storage.Storage, ledgerIndex iotago.MilestoneIndex) (*Manager, error) {
	manager := &Manager{
		Events: &Events{
			NextMilestoneUnsupported: events.NewEvent(protoParamsMsOptionCaller),
			CriticalErrors:           events.NewEvent(events.ErrorCaller),
		},
		storage: storage,
		current: nil,
		pending: nil,
	}

	if err := manager.init(ledgerIndex); err != nil {
		return nil, err
	}

	return manager, nil
}

// Manager handles the knowledge about current, pending and supported protocol versions and parameters.
type Manager struct {
	// Events holds the events happening within the Manager.
	Events      *Events
	storage     *storage.Storage
	currentLock sync.RWMutex
	current     *iotago.ProtocolParameters
	pendingLock sync.RWMutex
	pending     []*iotago.ProtocolParamsMilestoneOpt
}

// init initializes the Manager by loading the last stored parameters and pending parameters.
func (m *Manager) init(ledgerIndex iotago.MilestoneIndex) error {
	m.currentLock.Lock()
	defer m.currentLock.Unlock()

	currentProtoParams, err := m.storage.ProtocolParameters(ledgerIndex)
	if err != nil {
		return err
	}

	m.current = currentProtoParams
	m.loadPending(ledgerIndex)

	return nil
}

// loadPending initializes the pending protocol parameter changes from database.
func (m *Manager) loadPending(ledgerIndex iotago.MilestoneIndex) {
	m.pendingLock.Lock()
	defer m.pendingLock.Unlock()

	if err := m.storage.ForEachProtocolParameterMilestoneOption(func(protoParamsMsOption *iotago.ProtocolParamsMilestoneOpt) bool {
		if protoParamsMsOption.TargetMilestoneIndex > ledgerIndex {
			m.pending = append(m.pending, protoParamsMsOption)
		}

		return true
	}); err != nil {
		panic(err)
	}
}

// SupportedVersions returns a slice of supported protocol versions.
func (m *Manager) SupportedVersions() Versions {
	return SupportedVersions
}

// Current returns the current protocol parameters under which the node is operating.
func (m *Manager) Current() *iotago.ProtocolParameters {
	m.currentLock.RLock()
	defer m.currentLock.RUnlock()

	return m.current
}

// Pending returns the currently pending protocol changes.
func (m *Manager) Pending() []*iotago.ProtocolParamsMilestoneOpt {
	m.pendingLock.RLock()
	defer m.pendingLock.RUnlock()

	cpy := make([]*iotago.ProtocolParamsMilestoneOpt, len(m.pending))
	for i, ele := range m.pending {
		//nolint:forcetypeassert // we will replace that with generics anyway
		cpy[i] = ele.Clone().(*iotago.ProtocolParamsMilestoneOpt)
	}

	return cpy
}

// NextPendingSupported tells whether the next pending protocol parameters changes are supported.
func (m *Manager) NextPendingSupported() bool {
	m.pendingLock.RLock()
	defer m.pendingLock.RUnlock()
	if len(m.pending) == 0 {
		return true
	}

	return m.SupportedVersions().Supports(m.pending[0].ProtocolVersion)
}

// HandleConfirmedMilestone examines the newly confirmed milestone payload for protocol parameter changes.
func (m *Manager) HandleConfirmedMilestone(milestonePayload *iotago.Milestone) {

	if protoParamsMsOption := milestonePayload.Opts.MustSet().ProtocolParams(); protoParamsMsOption != nil {
		m.pendingLock.Lock()
		m.pending = append(m.pending, protoParamsMsOption)
		m.pendingLock.Unlock()

		if err := m.storage.StoreProtocolParametersMilestoneOption(protoParamsMsOption); err != nil {
			m.Events.CriticalErrors.Trigger(fmt.Errorf("unable to persist new protocol parameters: %w", err))

			return
		}
	}

	if !m.currentShouldChange(milestonePayload) {
		return
	}

	if err := m.updateCurrent(); err != nil {
		m.Events.CriticalErrors.Trigger(err)

		return
	}
}

// checks whether the current protocol parameters need to be updated.
func (m *Manager) currentShouldChange(milestonePayload *iotago.Milestone) bool {
	m.pendingLock.RLock()
	defer m.pendingLock.RUnlock()

	if len(m.pending) == 0 {
		return false
	}

	next := m.pending[0]

	switch {
	case next.TargetMilestoneIndex == milestonePayload.Index+1:
		if !m.SupportedVersions().Supports(next.ProtocolVersion) {
			m.Events.NextMilestoneUnsupported.Trigger(next)
		}

		return false
	case next.TargetMilestoneIndex > milestonePayload.Index:
		return false
	default:
		return true
	}
}

func (m *Manager) updateCurrent() error {
	m.currentLock.Lock()
	defer m.currentLock.Unlock()
	m.pendingLock.Lock()
	defer m.pendingLock.Unlock()

	nextProtoParamMsOption := m.pending[0]

	// TODO: needs to be adapted for when protocol parameters struct changes
	nextProtoParams := &iotago.ProtocolParameters{}
	if _, err := nextProtoParams.Deserialize(nextProtoParamMsOption.Params, serializer.DeSeriModePerformValidation, nil); err != nil {
		return fmt.Errorf("unable to deserialize new protocol parameters: %w", err)
	}

	m.current = nextProtoParams
	m.pending = m.pending[1:]

	return nil
}
