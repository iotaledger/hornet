package snapshot

import (
	"fmt"
	"sort"
	"sync"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hornet/pkg/protocol"
	iotago "github.com/iotaledger/iota.go/v3"
)

var (
	supportedVersions = protocol.Versions{2} // make sure to add the versions sorted asc
)

func ProtocolParametersUpdateAddedCaller(handler interface{}, params ...interface{}) {
	handler.(func(*iotago.ProtocolParamsMilestoneOpt))(params[0].(*iotago.ProtocolParamsMilestoneOpt))
}

type ProtocolManagerEvents struct {
	ProtocolParametersUpdateAdded *events.Event
}

// Manager handles the knowledge about current, pending and supported protocol versions and parameters.
type ProtocolManager struct {
	currentLock sync.RWMutex
	current     *iotago.ProtocolParameters

	pendingLock sync.RWMutex
	pending     []*iotago.ProtocolParamsMilestoneOpt

	Events *ProtocolManagerEvents
}

// NewManager creates a new Manager.
func NewSnapshotProtocolManager() *ProtocolManager {
	return &ProtocolManager{
		current: nil,
		pending: nil,
		Events: &ProtocolManagerEvents{
			ProtocolParametersUpdateAdded: events.NewEvent(ProtocolParametersUpdateAddedCaller),
		},
	}
}

// SupportedVersions returns a slice of supported protocol versions.
func (m *ProtocolManager) SupportedVersions() protocol.Versions {
	return supportedVersions
}

// Current returns the current protocol parameters under which the node is operating.
func (m *ProtocolManager) Current() *iotago.ProtocolParameters {
	m.currentLock.RLock()
	defer m.currentLock.RUnlock()
	return m.current
}

// HandleConfirmedMilestone examines the newly confirmed milestone for protocol parameter changes.
func (m *ProtocolManager) AddProtocolParametersUpdate(msProtoParas *iotago.ProtocolParamsMilestoneOpt) {
	m.pendingLock.Lock()
	defer m.pendingLock.Unlock()

	m.pending = append(m.pending, msProtoParas)
	sort.Slice(m.pending, func(i, j int) bool {
		return m.pending[i].TargetMilestoneIndex < m.pending[j].TargetMilestoneIndex
	})
	m.Events.ProtocolParametersUpdateAdded.Trigger(msProtoParas)
}

// checks whether the current protocol parameters need to be updated.
func (m *ProtocolManager) SetCurrentMilestoneIndex(index iotago.MilestoneIndex) error {

	if !m.currentShouldChange(index) {
		return nil
	}

	if err := m.updateCurrent(); err != nil {
		return err
	}

	return nil
}

// checks whether the current protocol parameters need to be updated.
func (m *ProtocolManager) currentShouldChange(index iotago.MilestoneIndex) bool {
	m.pendingLock.RLock()
	defer m.pendingLock.RUnlock()

	if len(m.pending) == 0 {
		return false
	}

	return !(m.pending[0].TargetMilestoneIndex > index)
}

func (m *ProtocolManager) updateCurrent() error {
	m.currentLock.Lock()
	defer m.currentLock.Unlock()
	m.pendingLock.Lock()
	defer m.pendingLock.Unlock()

	nextMsProtoParamOpt := m.pending[0]

	if !m.SupportedVersions().Supports(nextMsProtoParamOpt.ProtocolVersion) {
		return fmt.Errorf("protocol version %d is not supported", nextMsProtoParamOpt.ProtocolVersion)
	}

	nextParams := nextMsProtoParamOpt.Params

	// TODO: needs to be adapted for when protocol parameters struct changes
	nextProtoParams := &iotago.ProtocolParameters{}
	if _, err := nextProtoParams.Deserialize(nextParams, serializer.DeSeriModePerformValidation, nil); err != nil {
		return fmt.Errorf("unable to deserialize new protocol parameters: %w", err)
	}

	m.current = nextProtoParams

	// TODO: we should not remove pending ones in the snapshot protocol manager
	m.pending = m.pending[1:]

	return nil
}
