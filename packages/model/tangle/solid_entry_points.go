package tangle

import (
	"github.com/iotaledger/iota.go/trinary"
	"github.com/gohornet/hornet/packages/model/milestone_index"

	"github.com/pkg/errors"
	"github.com/gohornet/hornet/packages/model/hornet"
)

var (
	solidEntryPoints *hornet.SolidEntryPoints

	ErrSolidEntryPointsAlreadyInitialized = errors.New("solidEntryPoints already initialized")
	ErrSolidEntryPointsNotInitialized     = errors.New("solidEntryPoints not initialized")
)

func GetSolidEntryPoints() *hornet.SolidEntryPoints {
	return solidEntryPoints.Copy()
}

func GetSolidEntryPointsHashes() []trinary.Hash {
	return solidEntryPoints.Hashes()
}

func loadSolidEntryPoints() {
	if solidEntryPoints == nil {
		points, err := readSolidEntryPointsFromDatabase()
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
	if solidEntryPoints != nil {
		return solidEntryPoints.Contains(transactionHash)
	} else {
		panic(ErrSolidEntryPointsNotInitialized)
	}
}

func SolidEntryPointsAdd(transactionHash trinary.Hash, milestoneIndex milestone_index.MilestoneIndex) {
	if solidEntryPoints != nil {
		solidEntryPoints.Add(transactionHash, milestoneIndex)
	} else {
		panic(ErrSolidEntryPointsNotInitialized)
	}
}

func ResetSolidEntryPoints() {
	if solidEntryPoints != nil {
		solidEntryPoints.Clear()
	} else {
		panic(ErrSolidEntryPointsNotInitialized)
	}
}

func StoreSolidEntryPoints() {
	if solidEntryPoints != nil {
		storeSolidEntryPointsInDatabase(solidEntryPoints)
	} else {
		panic(ErrSolidEntryPointsNotInitialized)
	}
}
