package storage

import (
	"github.com/pkg/errors"

	iotago "github.com/iotaledger/iota.go/v3"
)

var (
	ErrSolidEntryPointsAlreadyInitialized = errors.New("solidEntryPoints already initialized")
	ErrSolidEntryPointsNotInitialized     = errors.New("solidEntryPoints not initialized")
)

// SolidEntryPointConsumer consumes the solid entry point during looping through all solid entry points.
// Returning false from this function indicates to abort the iteration.
type SolidEntryPointConsumer func(solidEntryPoint *SolidEntryPoint) bool

func (s *Storage) ReadLockSolidEntryPoints() {
	s.solidEntryPointsLock.RLock()
}

func (s *Storage) ReadUnlockSolidEntryPoints() {
	s.solidEntryPointsLock.RUnlock()
}

func (s *Storage) WriteLockSolidEntryPoints() {
	s.solidEntryPointsLock.Lock()
}

func (s *Storage) WriteUnlockSolidEntryPoints() {
	s.solidEntryPointsLock.Unlock()
}

func (s *Storage) loadSolidEntryPoints() error {
	s.WriteLockSolidEntryPoints()
	defer s.WriteUnlockSolidEntryPoints()

	if s.solidEntryPoints != nil {
		return ErrSolidEntryPointsAlreadyInitialized
	}

	points, err := s.readSolidEntryPoints()
	if err != nil {
		return err
	}

	if points == nil {
		s.solidEntryPoints = NewSolidEntryPoints()

		return nil
	}

	s.solidEntryPoints = points

	return nil
}

func (s *Storage) SolidEntryPointsContain(blockID iotago.BlockID) (bool, error) {
	s.ReadLockSolidEntryPoints()
	defer s.ReadUnlockSolidEntryPoints()

	if s.solidEntryPoints == nil {
		// this can only happen at startup of the node, no need to return an unused error all the time
		panic(ErrSolidEntryPointsNotInitialized)
	}

	return s.solidEntryPoints.Contains(blockID), nil
}

// SolidEntryPointsIndex returns the index of a solid entry point and whether the block is a solid entry point or not.
func (s *Storage) SolidEntryPointsIndex(blockID iotago.BlockID) (iotago.MilestoneIndex, bool, error) {
	s.ReadLockSolidEntryPoints()
	defer s.ReadUnlockSolidEntryPoints()

	if s.solidEntryPoints == nil {
		// this can only happen at startup of the node, no need to return an unused error all the time
		panic(ErrSolidEntryPointsNotInitialized)
	}

	index, contains := s.solidEntryPoints.Index(blockID)

	return index, contains, nil
}

// SolidEntryPointsAddWithoutLocking adds a block to the solid entry points.
// WriteLockSolidEntryPoints must be held while entering this function.
func (s *Storage) SolidEntryPointsAddWithoutLocking(blockID iotago.BlockID, milestoneIndex iotago.MilestoneIndex) {
	if s.solidEntryPoints == nil {
		// this can only happen at startup of the node, no need to return an unused error all the time
		panic(ErrSolidEntryPointsNotInitialized)
	}
	s.solidEntryPoints.Add(blockID, milestoneIndex)
}

// ResetSolidEntryPointsWithoutLocking resets the solid entry points.
// WriteLockSolidEntryPoints must be held while entering this function.
func (s *Storage) ResetSolidEntryPointsWithoutLocking() {
	if s.solidEntryPoints == nil {
		// this can only happen at startup of the node, no need to return an unused error all the time
		panic(ErrSolidEntryPointsNotInitialized)
	}
	s.solidEntryPoints.Clear()
}

// StoreSolidEntryPointsWithoutLocking stores the solid entry points in the persistence layer.
// WriteLockSolidEntryPoints must be held while entering this function.
func (s *Storage) StoreSolidEntryPointsWithoutLocking() error {
	if s.solidEntryPoints == nil {
		// this can only happen at startup of the node, no need to return an unused error all the time
		panic(ErrSolidEntryPointsNotInitialized)
	}

	return s.storeSolidEntryPoints(s.solidEntryPoints)
}

// ForEachSolidEntryPointWithoutLocking loops over all solid entry points in the persistence layer.
// WriteLockSolidEntryPoints must be held while entering this function.
func (s *Storage) ForEachSolidEntryPointWithoutLocking(consumer SolidEntryPointConsumer) {
	if s.solidEntryPoints == nil {
		// this can only happen at startup of the node, no need to return an unused error all the time
		panic(ErrSolidEntryPointsNotInitialized)
	}

	seps := s.solidEntryPoints.Sorted()
	for i := 0; i < len(seps); i++ {
		sep := seps[i]

		// Call consumer with the cached object and check if we should abort the iteration.
		if !consumer(sep) {
			return
		}
	}
}
