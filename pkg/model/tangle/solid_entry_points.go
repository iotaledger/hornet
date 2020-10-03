package tangle

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/gohornet/hornet/pkg/model/hornet"

	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/pkg/model/milestone"
)

type SolidEntryPoints struct {
	entryPointsMap   map[string]milestone.Index
	entryPointsSlice hornet.MessageIDs

	// Status
	statusMutex syncutils.RWMutex
	modified    bool
}

func NewSolidEntryPoints() *SolidEntryPoints {
	return &SolidEntryPoints{
		entryPointsMap: make(map[string]milestone.Index),
	}
}

func (s *SolidEntryPoints) Hashes() hornet.MessageIDs {
	solidEntryPointCopy := make(hornet.MessageIDs, len(s.entryPointsSlice))
	copy(solidEntryPointCopy, s.entryPointsSlice)
	return solidEntryPointCopy
}

func (s *SolidEntryPoints) Contains(messageID *hornet.MessageID) bool {
	_, exists := s.entryPointsMap[messageID.MapKey()]
	return exists
}

func (s *SolidEntryPoints) Index(messageID *hornet.MessageID) (milestone.Index, bool) {
	index, exists := s.entryPointsMap[messageID.MapKey()]
	return index, exists
}

func (s *SolidEntryPoints) Add(messageID *hornet.MessageID, milestoneIndex milestone.Index) {
	messageIDMapKey := messageID.MapKey()
	if _, exists := s.entryPointsMap[messageIDMapKey]; !exists {
		s.entryPointsMap[messageIDMapKey] = milestoneIndex
		s.entryPointsSlice = append(s.entryPointsSlice, messageID)
		s.SetModified(true)
	}
}

func (s *SolidEntryPoints) Clear() {
	s.entryPointsMap = make(map[string]milestone.Index)
	s.entryPointsSlice = make(hornet.MessageIDs, 0)
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

		err = binary.Read(bytesReader, binary.LittleEndian, messageIDBuf)
		if err != nil {
			return nil, fmt.Errorf("solidEntryPoints: %s", err)
		}

		err = binary.Read(bytesReader, binary.LittleEndian, &msIndex)
		if err != nil {
			return nil, fmt.Errorf("solidEntryPoints: %s", err)
		}

		s.Add(hornet.MessageIDFromBytes(messageIDBuf), milestone.Index(msIndex))
	}

	return s, nil
}

func (s *SolidEntryPoints) GetBytes() []byte {

	buf := bytes.NewBuffer(make([]byte, 0, len(s.entryPointsMap)*(32+4)))

	for messageID, msIndex := range s.entryPointsMap {
		err := binary.Write(buf, binary.LittleEndian, []byte(messageID)[:32])
		if err != nil {
			return nil
		}

		err = binary.Write(buf, binary.LittleEndian, uint32(msIndex))
		if err != nil {
			return nil
		}
	}

	return buf.Bytes()
}
