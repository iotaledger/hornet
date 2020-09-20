package hornet

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/pkg/model/milestone"
)

type SolidEntryPoints struct {
	entryPointsMap   map[string]milestone.Index
	entryPointsSlice Hashes

	// Status
	statusMutex syncutils.RWMutex
	modified    bool
}

func NewSolidEntryPoints() *SolidEntryPoints {
	return &SolidEntryPoints{
		entryPointsMap: make(map[string]milestone.Index),
	}
}

func (s *SolidEntryPoints) Hashes() Hashes {
	solidEntryPointCopy := make(Hashes, len(s.entryPointsSlice))
	copy(solidEntryPointCopy, s.entryPointsSlice)
	return solidEntryPointCopy
}

func (s *SolidEntryPoints) Contains(messageID Hash) bool {
	_, exists := s.entryPointsMap[string(messageID)]
	return exists
}

func (s *SolidEntryPoints) Index(messageID Hash) (milestone.Index, bool) {
	index, exists := s.entryPointsMap[string(messageID)]
	return index, exists
}

func (s *SolidEntryPoints) Add(messageID Hash, milestoneIndex milestone.Index) {
	if _, exists := s.entryPointsMap[string(messageID)]; !exists {
		s.entryPointsMap[string(messageID)] = milestoneIndex
		s.entryPointsSlice = append(s.entryPointsSlice, messageID)
		s.SetModified(true)
	}
}

func (s *SolidEntryPoints) Clear() {
	s.entryPointsMap = make(map[string]milestone.Index)
	s.entryPointsSlice = make(Hashes, 0)
	s.SetModified(true)
}

func (s *SolidEntryPoints) IsModified() bool {
	s.statusMutex.RLock()
	defer s.statusMutex.RUnlock()

	return s.modified
}

func (s *SolidEntryPoints) SetModified(modified bool) {
	s.statusMutex.Lock()
	defer s.statusMutex.Unlock()

	s.modified = modified
}

func SolidEntryPointsFromBytes(solidEntryPointsBytes []byte) (*SolidEntryPoints, error) {
	s := NewSolidEntryPoints()

	bytesReader := bytes.NewReader(solidEntryPointsBytes)

	var err error

	solidEntryPointsCount := len(solidEntryPointsBytes) / (32 + 4)
	for i := 0; i < solidEntryPointsCount; i++ {
		messageIDBuf := make([]byte, 32)
		var msIndex uint32

		err = binary.Read(bytesReader, binary.BigEndian, messageIDBuf)
		if err != nil {
			return nil, fmt.Errorf("solidEntryPoints: %s", err)
		}

		err = binary.Read(bytesReader, binary.BigEndian, &msIndex)
		if err != nil {
			return nil, fmt.Errorf("solidEntryPoints: %s", err)
		}

		s.Add(Hash(messageIDBuf), milestone.Index(msIndex))
	}

	return s, nil
}

func (s *SolidEntryPoints) GetBytes() []byte {

	buf := bytes.NewBuffer(make([]byte, 0, len(s.entryPointsMap)*(32+4)))

	for messageID, msIndex := range s.entryPointsMap {
		err := binary.Write(buf, binary.BigEndian, []byte(messageID)[:32])
		if err != nil {
			return nil
		}

		err = binary.Write(buf, binary.BigEndian, uint32(msIndex))
		if err != nil {
			return nil
		}
	}

	return buf.Bytes()
}
