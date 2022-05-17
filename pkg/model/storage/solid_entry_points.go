package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/syncutils"
)

type SolidEntryPoint struct {
	BlockID hornet.BlockID
	Index   milestone.Index
}

// LexicalOrderedSolidEntryPoints are solid entry points
// ordered in lexical order by their BlockID.
type LexicalOrderedSolidEntryPoints []*SolidEntryPoint

func (l LexicalOrderedSolidEntryPoints) Len() int {
	return len(l)
}

func (l LexicalOrderedSolidEntryPoints) Less(i, j int) bool {
	return bytes.Compare(l[i].BlockID, l[j].BlockID) < 0
}

func (l LexicalOrderedSolidEntryPoints) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

type SolidEntryPoints struct {
	entryPointsMap   map[string]milestone.Index
	entryPointsSlice hornet.BlockIDs

	// Status
	statusMutex syncutils.RWMutex
	modified    bool
}

func NewSolidEntryPoints() *SolidEntryPoints {
	return &SolidEntryPoints{
		entryPointsMap: make(map[string]milestone.Index),
	}
}

func (s *SolidEntryPoints) copy() []*SolidEntryPoint {
	solidEntryPointsCount := len(s.entryPointsMap)
	result := make([]*SolidEntryPoint, solidEntryPointsCount)

	i := 0
	for hash, msIndex := range s.entryPointsMap {
		blockID := hornet.BlockIDFromMapKey(hash)
		result[i] = &SolidEntryPoint{
			BlockID: blockID,
			Index:   msIndex,
		}
		i++
	}

	return result
}

func (s *SolidEntryPoints) Contains(blockID hornet.BlockID) bool {
	_, exists := s.entryPointsMap[blockID.ToMapKey()]
	return exists
}

func (s *SolidEntryPoints) Index(blockID hornet.BlockID) (milestone.Index, bool) {
	index, exists := s.entryPointsMap[blockID.ToMapKey()]
	return index, exists
}

func (s *SolidEntryPoints) Add(blockID hornet.BlockID, milestoneIndex milestone.Index) {
	blockIDMapKey := blockID.ToMapKey()
	if _, exists := s.entryPointsMap[blockIDMapKey]; !exists {
		s.entryPointsMap[blockIDMapKey] = milestoneIndex
		s.entryPointsSlice = append(s.entryPointsSlice, blockID)
		s.SetModified(true)
	}
}

func (s *SolidEntryPoints) Clear() {
	s.entryPointsMap = make(map[string]milestone.Index)
	s.entryPointsSlice = make(hornet.BlockIDs, 0)
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

// sort the solid entry points lexicographically by their BlockID
func (s *SolidEntryPoints) Sorted() []*SolidEntryPoint {

	var sortedSolidEntryPoints LexicalOrderedSolidEntryPoints = s.copy()
	sort.Sort(sortedSolidEntryPoints)
	return sortedSolidEntryPoints
}

func SolidEntryPointsFromBytes(solidEntryPointsBytes []byte) (*SolidEntryPoints, error) {
	s := NewSolidEntryPoints()

	bytesReader := bytes.NewReader(solidEntryPointsBytes)

	var err error

	solidEntryPointsCount := len(solidEntryPointsBytes) / (32 + 4)
	for i := 0; i < solidEntryPointsCount; i++ {
		blockIDBuf := make([]byte, 32)
		var msIndex uint32

		err = binary.Read(bytesReader, binary.LittleEndian, blockIDBuf)
		if err != nil {
			return nil, fmt.Errorf("solidEntryPoints: %s", err)
		}

		err = binary.Read(bytesReader, binary.LittleEndian, &msIndex)
		if err != nil {
			return nil, fmt.Errorf("solidEntryPoints: %s", err)
		}
		s.Add(hornet.BlockIDFromSlice(blockIDBuf), milestone.Index(msIndex))
	}

	return s, nil
}

func (s *SolidEntryPoints) Bytes() []byte {

	buf := bytes.NewBuffer(make([]byte, 0, len(s.entryPointsMap)*(32+4)))

	for _, sep := range s.Sorted() {
		err := binary.Write(buf, binary.LittleEndian, sep.BlockID)
		if err != nil {
			return nil
		}

		err = binary.Write(buf, binary.LittleEndian, sep.Index)
		if err != nil {
			return nil
		}
	}

	return buf.Bytes()
}

func (s *SolidEntryPoints) SHA256Sum() ([]byte, error) {

	sepHash := sha256.New()

	// compute the sha256 of the solid entry points byte representation
	if err := binary.Write(sepHash, binary.LittleEndian, s.Bytes()); err != nil {
		return nil, fmt.Errorf("unable to serialize solid entry points: %w", err)
	}

	// calculate sha256 hash
	return sepHash.Sum(nil), nil
}
