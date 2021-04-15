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

func (s *Storage) loadSolidEntryPoints() {
	s.WriteLockSolidEntryPoints()
	defer s.WriteUnlockSolidEntryPoints()

	if s.solidEntryPoints != nil {
		panic(ErrSolidEntryPointsAlreadyInitialized)
	}

	points, err := s.readSolidEntryPoints()
	if points != nil && err == nil {
		s.solidEntryPoints = points
	} else {
		s.solidEntryPoints = NewSolidEntryPoints()
	}
}

func (s *Storage) SolidEntryPointsContain(messageID hornet.MessageID) bool {
	s.ReadLockSolidEntryPoints()
	defer s.ReadUnlockSolidEntryPoints()

	if s.solidEntryPoints == nil {
		panic(ErrSolidEntryPointsNotInitialized)
	}
	return s.solidEntryPoints.Contains(messageID)
}

func (s *Storage) SolidEntryPointsIndex(messageID hornet.MessageID) (milestone.Index, bool) {
	s.ReadLockSolidEntryPoints()
	defer s.ReadUnlockSolidEntryPoints()

	if s.solidEntryPoints == nil {
		panic(ErrSolidEntryPointsNotInitialized)
	}
	return s.solidEntryPoints.Index(messageID)
}

// WriteLockSolidEntryPoints must be held while entering this function
func (s *Storage) SolidEntryPointsAdd(messageID hornet.MessageID, milestoneIndex milestone.Index) {
	if s.solidEntryPoints == nil {
		panic(ErrSolidEntryPointsNotInitialized)
	}
	s.solidEntryPoints.Add(messageID, milestoneIndex)
}

// WriteLockSolidEntryPoints must be held while entering this function
func (s *Storage) ResetSolidEntryPoints() {
	if s.solidEntryPoints == nil {
		panic(ErrSolidEntryPointsNotInitialized)
	}
	s.solidEntryPoints.Clear()
}

// WriteLockSolidEntryPoints must be held while entering this function
func (s *Storage) StoreSolidEntryPoints() {
	if s.solidEntryPoints == nil {
		panic(ErrSolidEntryPointsNotInitialized)
	}
	s.storeSolidEntryPoints(s.solidEntryPoints)
}
