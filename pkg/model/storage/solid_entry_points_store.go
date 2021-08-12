package storage

import (
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
)

var (
	ErrSolidEntryPointsAlreadyInitialized = errors.New("solidEntryPoints already initialized")
	ErrSolidEntryPointsNotInitialized     = errors.New("solidEntryPoints not initialized")
)

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

func (s *Storage) SolidEntryPointsContain(messageID hornet.MessageID) bool {
	s.ReadLockSolidEntryPoints()
	defer s.ReadUnlockSolidEntryPoints()

	if s.solidEntryPoints == nil {
		// this can only happen at startup of the node, no need to return an unused error all the time
		panic(ErrSolidEntryPointsNotInitialized)
	}
	return s.solidEntryPoints.Contains(messageID)
}

// SolidEntryPointsIndex returns the index of a solid entry point and whether the message is a solid entry point or not.
func (s *Storage) SolidEntryPointsIndex(messageID hornet.MessageID) (milestone.Index, bool) {
	s.ReadLockSolidEntryPoints()
	defer s.ReadUnlockSolidEntryPoints()

	if s.solidEntryPoints == nil {
		// this can only happen at startup of the node, no need to return an unused error all the time
		panic(ErrSolidEntryPointsNotInitialized)
	}
	return s.solidEntryPoints.Index(messageID)
}

// SolidEntryPointsAddWithoutLocking adds a message to the solid entry points.
// WriteLockSolidEntryPoints must be held while entering this function.
func (s *Storage) SolidEntryPointsAddWithoutLocking(messageID hornet.MessageID, milestoneIndex milestone.Index) {
	if s.solidEntryPoints == nil {
		// this can only happen at startup of the node, no need to return an unused error all the time
		panic(ErrSolidEntryPointsNotInitialized)
	}
	s.solidEntryPoints.Add(messageID, milestoneIndex)
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
