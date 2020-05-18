package tangle

import (
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
)

var (
	solidEntryPoints     *hornet.SolidEntryPoints
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

func GetSolidEntryPointsHashes() []trinary.Hash {
	ReadLockSolidEntryPoints()
	defer ReadUnlockSolidEntryPoints()

	return solidEntryPoints.Hashes()
}

func loadSolidEntryPoints() {
	WriteLockSolidEntryPoints()
	defer WriteUnlockSolidEntryPoints()

	if solidEntryPoints == nil {
		points, err := readSolidEntryPoints()
		if points != nil && err == nil {
			solidEntryPoints = points
		} else {
			solidEntryPoints = hornet.NewSolidEntryPoints()
		}
	} else {
		panic(ErrSolidEntryPointsAlreadyInitialized)
	}
}

func SolidEntryPointsContain(transactionHash trinary.Hash) bool {
	ReadLockSolidEntryPoints()
	defer ReadUnlockSolidEntryPoints()

	if solidEntryPoints != nil {
		return solidEntryPoints.Contains(transactionHash)
	} else {
		panic(ErrSolidEntryPointsNotInitialized)
	}
}

// WriteLockSolidEntryPoints must be held while entering this function
func SolidEntryPointsAdd(transactionHash trinary.Hash, milestoneIndex milestone.Index) {
	if solidEntryPoints != nil {
		solidEntryPoints.Add(transactionHash, milestoneIndex)
	} else {
		panic(ErrSolidEntryPointsNotInitialized)
	}
}

// WriteLockSolidEntryPoints must be held while entering this function
func ResetSolidEntryPoints() {
	if solidEntryPoints != nil {
		solidEntryPoints.Clear()
	} else {
		panic(ErrSolidEntryPointsNotInitialized)
	}
}

// WriteLockSolidEntryPoints must be held while entering this function
func StoreSolidEntryPoints() {
	if solidEntryPoints != nil {
		storeSolidEntryPoints(solidEntryPoints)
	} else {
		panic(ErrSolidEntryPointsNotInitialized)
	}
}
