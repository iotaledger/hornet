package hornet

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/syncutils"
)

type SolidEntryPoints struct {
	entryPointsMutex syncutils.RWMutex
	entryPoints      map[trinary.Hash]milestone_index.MilestoneIndex

	// Status
	statusMutex syncutils.RWMutex
	modified    bool
}

func NewSolidEntryPoints() *SolidEntryPoints {
	return &SolidEntryPoints{
		entryPoints: make(map[trinary.Hash]milestone_index.MilestoneIndex),
	}
}

func (s *SolidEntryPoints) Hashes() []trinary.Hash {
	// TODO: cache subsequent calls instead of creating a new slice everytime
	s.entryPointsMutex.RLock()
	defer s.entryPointsMutex.RUnlock()
	var hashes []trinary.Hash
	for hash := range s.entryPoints {
		hashes = append(hashes, hash)
	}
	return hashes
}

func (s *SolidEntryPoints) Copy() *SolidEntryPoints {
	s.entryPointsMutex.RLock()
	defer s.entryPointsMutex.RUnlock()
	cpy := NewSolidEntryPoints()
	cpy.modified = s.modified
	for hash, index := range s.entryPoints {
		cpy.entryPoints[hash] = index
	}
	return cpy
}

func (s *SolidEntryPoints) Contains(transactionHash trinary.Hash) bool {
	s.entryPointsMutex.RLock()
	defer s.entryPointsMutex.RUnlock()
	return ContainsKeyTrinaryHashMilestoneIndex(s.entryPoints, transactionHash)
}

func (s *SolidEntryPoints) Add(transactionHash trinary.Hash, milestoneIndex milestone_index.MilestoneIndex) {
	s.entryPointsMutex.Lock()
	defer s.entryPointsMutex.Unlock()
	s.entryPoints[transactionHash] = milestoneIndex
	s.SetModified(true)
}

func (s *SolidEntryPoints) Clear() {
	s.entryPointsMutex.Lock()
	defer s.entryPointsMutex.Unlock()
	s.entryPoints = make(map[trinary.Hash]milestone_index.MilestoneIndex)
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

	buf := bytes.NewBuffer(make([]byte, 0, len(s.entryPoints)*(49+4)))

	for hash, msIndex := range s.entryPoints {
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
