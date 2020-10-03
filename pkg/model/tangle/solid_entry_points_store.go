package tangle

import (
	"sync"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
)

var (
	solidEntryPoints     *SolidEntryPoints
	solidEntryPointsLock sync.RWMutex

	ErrSolidEntryPointsAlreadyInitialized = errors.New("solidEntryPoints already initialized")
	ErrSolidEntryPointsNotInitialized     = errors.New("solidEntryPoints not initialized")
)

func ReadLockSolidEntryPoints() {
	solidEntryPointsLock.RLock()
}

func ReadUnlockSolidEntryPoints() {
	solidEntryPointsLock.RUnlock()
}

func WriteLockSolidEntryPoints() {
	solidEntryPointsLock.Lock()
}

func WriteUnlockSolidEntryPoints() {
	solidEntryPointsLock.Unlock()
}

func loadSolidEntryPoints() {
	WriteLockSolidEntryPoints()
	defer WriteUnlockSolidEntryPoints()

	if solidEntryPoints != nil {
		panic(ErrSolidEntryPointsAlreadyInitialized)
	}

	points, err := readSolidEntryPoints()
	if points != nil && err == nil {
		solidEntryPoints = points
	} else {
		solidEntryPoints = NewSolidEntryPoints()
	}
}

func SolidEntryPointsContain(messageID *hornet.MessageID) bool {
	ReadLockSolidEntryPoints()
	defer ReadUnlockSolidEntryPoints()

	if solidEntryPoints == nil {
		panic(ErrSolidEntryPointsNotInitialized)
	}
	return solidEntryPoints.Contains(messageID)
}

func SolidEntryPointsIndex(messageID *hornet.MessageID) (milestone.Index, bool) {
	ReadLockSolidEntryPoints()
	defer ReadUnlockSolidEntryPoints()

	if solidEntryPoints == nil {
		panic(ErrSolidEntryPointsNotInitialized)
	}
	return solidEntryPoints.Index(messageID)
}

// WriteLockSolidEntryPoints must be held while entering this function
func SolidEntryPointsAdd(messageID *hornet.MessageID, milestoneIndex milestone.Index) {
	if solidEntryPoints == nil {
		panic(ErrSolidEntryPointsNotInitialized)
	}
	solidEntryPoints.Add(messageID, milestoneIndex)
}

// WriteLockSolidEntryPoints must be held while entering this function
func ResetSolidEntryPoints() {
	if solidEntryPoints == nil {
		panic(ErrSolidEntryPointsNotInitialized)
	}
	solidEntryPoints.Clear()
}

// WriteLockSolidEntryPoints must be held while entering this function
func StoreSolidEntryPoints() {
	if solidEntryPoints == nil {
		panic(ErrSolidEntryPointsNotInitialized)
	}
	storeSolidEntryPoints(solidEntryPoints)
}
