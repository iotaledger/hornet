package tangle

import (
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
)

var (
	ErrSolidEntryPointsAlreadyInitialized = errors.New("solidEntryPoints already initialized")
	ErrSolidEntryPointsNotInitialized     = errors.New("solidEntryPoints not initialized")
)

func (t *Tangle) ReadLockSolidEntryPoints() {
	t.solidEntryPointsLock.RLock()
}

func (t *Tangle) ReadUnlockSolidEntryPoints() {
	t.solidEntryPointsLock.RUnlock()
}

func (t *Tangle) WriteLockSolidEntryPoints() {
	t.solidEntryPointsLock.Lock()
}

func (t *Tangle) WriteUnlockSolidEntryPoints() {
	t.solidEntryPointsLock.Unlock()
}

func (t *Tangle) loadSolidEntryPoints() {
	t.WriteLockSolidEntryPoints()
	defer t.WriteUnlockSolidEntryPoints()

	if t.solidEntryPoints != nil {
		panic(ErrSolidEntryPointsAlreadyInitialized)
	}

	points, err := t.readSolidEntryPoints()
	if points != nil && err == nil {
		t.solidEntryPoints = points
	} else {
		t.solidEntryPoints = NewSolidEntryPoints()
	}
}

func (t *Tangle) SolidEntryPointsContain(messageID *hornet.MessageID) bool {
	t.ReadLockSolidEntryPoints()
	defer t.ReadUnlockSolidEntryPoints()

	if t.solidEntryPoints == nil {
		panic(ErrSolidEntryPointsNotInitialized)
	}
	return t.solidEntryPoints.Contains(messageID)
}

func (t *Tangle) SolidEntryPointsIndex(messageID *hornet.MessageID) (milestone.Index, bool) {
	t.ReadLockSolidEntryPoints()
	defer t.ReadUnlockSolidEntryPoints()

	if t.solidEntryPoints == nil {
		panic(ErrSolidEntryPointsNotInitialized)
	}
	return t.solidEntryPoints.Index(messageID)
}

// WriteLockSolidEntryPoints must be held while entering this function
func (t *Tangle) SolidEntryPointsAdd(messageID *hornet.MessageID, milestoneIndex milestone.Index) {
	if t.solidEntryPoints == nil {
		panic(ErrSolidEntryPointsNotInitialized)
	}
	t.solidEntryPoints.Add(messageID, milestoneIndex)
}

// WriteLockSolidEntryPoints must be held while entering this function
func (t *Tangle) ResetSolidEntryPoints() {
	if t.solidEntryPoints == nil {
		panic(ErrSolidEntryPointsNotInitialized)
	}
	t.solidEntryPoints.Clear()
}

// WriteLockSolidEntryPoints must be held while entering this function
func (t *Tangle) StoreSolidEntryPoints() {
	if t.solidEntryPoints == nil {
		panic(ErrSolidEntryPointsNotInitialized)
	}
	t.storeSolidEntryPoints(t.solidEntryPoints)
}
