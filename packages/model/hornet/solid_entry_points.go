package hornet

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/packages/model/milestone_index"
)

type SolidEntryPoints struct {
	entryPointsMap   map[trinary.Hash]milestone_index.MilestoneIndex
	entryPointsSlice []trinary.Hash

	// Status
	statusMutex syncutils.RWMutex
	modified    bool
}

func NewSolidEntryPoints() *SolidEntryPoints {
	return &SolidEntryPoints{
		entryPointsMap: make(map[trinary.Hash]milestone_index.MilestoneIndex),
	}
}

func (s *SolidEntryPoints) Hashes() []trinary.Hash {
	return s.entryPointsSlice
}

func (s *SolidEntryPoints) Contains(transactionHash trinary.Hash) bool {
	_, exists := s.entryPointsMap[transactionHash]
	return exists
}

func (s *SolidEntryPoints) Add(transactionHash trinary.Hash, milestoneIndex milestone_index.MilestoneIndex) {
	s.entryPointsMap[transactionHash] = milestoneIndex
	s.entryPointsSlice = append(s.entryPointsSlice, transactionHash)
	s.SetModified(true)
}

func (s *SolidEntryPoints) Clear() {
	s.entryPointsMap = make(map[trinary.Hash]milestone_index.MilestoneIndex)
	s.entryPointsSlice = make([]trinary.Hash, 0)
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

	hashBuf := make([]byte, 49)
	bytesReader := bytes.NewReader(solidEntryPointsBytes)

	var err error

	solidEntryPointsCount := len(solidEntryPointsBytes) / (49 + 4)
	for i := 0; i < int(solidEntryPointsCount); i++ {
		var val uint32

		err = binary.Read(bytesReader, binary.BigEndian, hashBuf)
		if err != nil {
			return nil, fmt.Errorf("solidEntryPoints: %s", err)
		}

		err = binary.Read(bytesReader, binary.BigEndian, &val)
		if err != nil {
			return nil, fmt.Errorf("solidEntryPoints: %s", err)
		}

		hash := trinary.MustBytesToTrytes(hashBuf, 81)

		s.Add(hash[:81], milestone_index.MilestoneIndex(val))
	}

	return s, nil
}

func (s *SolidEntryPoints) GetBytes() []byte {

	buf := bytes.NewBuffer(make([]byte, 0, len(s.entryPointsMap)*(49+4)))

	for hash, msIndex := range s.entryPointsMap {
		hashBytes, err := trinary.TrytesToBytes(hash)
		if err != nil {
			return nil
		}

		err = binary.Write(buf, binary.BigEndian, hashBytes[:49])
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
