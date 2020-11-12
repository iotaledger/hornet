package storage

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/marshalutil"
	iotago "github.com/iotaledger/iota.go"

	"github.com/gohornet/hornet/pkg/model/milestone"
)

var (
	ErrParseSnapshotInfoFailed = errors.New("Parsing of snapshot info failed")
)

type SnapshotInfo struct {
	NetworkID       uint8
	MilestoneID     *iotago.MilestoneID
	SnapshotIndex   milestone.Index
	EntryPointIndex milestone.Index
	PruningIndex    milestone.Index
	Timestamp       time.Time
	Metadata        bitmask.BitMask
}

func (s *Storage) loadSnapshotInfo() {
	info, err := s.readSnapshotInfo()
	if err != nil {
		panic(err)
	}
	s.snapshot = info
	if info != nil {
		println(fmt.Sprintf(`SnapshotInfo:
	NetworkID: %d
	SnapshotIndex: %d (%v)
	EntryPointIndex: %d
	PruningIndex: %d
	Timestamp: %v`, info.NetworkID, info.SnapshotIndex, hex.EncodeToString(info.MilestoneID[:]), info.EntryPointIndex, info.PruningIndex, info.Timestamp.Truncate(time.Second)))
	}
}

func SnapshotInfoFromBytes(bytes []byte) (*SnapshotInfo, error) {

	if len(bytes) != 54 {
		return nil, errors.Wrapf(ErrParseSnapshotInfoFailed, "invalid length %d != %d", len(bytes), 54)
	}

	marshalUtil := marshalutil.New(bytes)

	networkID, err := marshalUtil.ReadByte()
	if err != nil {
		return nil, err
	}

	milestoneID := &iotago.MilestoneID{}
	milestoneIDBytes, err := marshalUtil.ReadBytes(32)
	if err != nil {
		return nil, err
	}
	copy(milestoneID[:], milestoneIDBytes[:32])

	snapshotIndex, err := marshalUtil.ReadUint32()
	if err != nil {
		return nil, err
	}

	entryPointIndex, err := marshalUtil.ReadUint32()
	if err != nil {
		return nil, err
	}

	pruningIndex, err := marshalUtil.ReadUint32()
	if err != nil {
		return nil, err
	}

	timestamp, err := marshalUtil.ReadUint64()
	if err != nil {
		return nil, err
	}

	metadata, err := marshalUtil.ReadByte()
	if err != nil {
		return nil, err
	}

	return &SnapshotInfo{
		NetworkID:       networkID,
		MilestoneID:     milestoneID,
		SnapshotIndex:   milestone.Index(snapshotIndex),
		EntryPointIndex: milestone.Index(entryPointIndex),
		PruningIndex:    milestone.Index(pruningIndex),
		Timestamp:       time.Unix(int64(timestamp), 0),
		Metadata:        bitmask.BitMask(metadata),
	}, nil
}

func (i *SnapshotInfo) GetBytes() []byte {
	marshalUtil := marshalutil.New()

	marshalUtil.WriteByte(i.NetworkID)
	marshalUtil.WriteBytes(i.MilestoneID[:32])
	marshalUtil.WriteUint32(uint32(i.SnapshotIndex))
	marshalUtil.WriteUint32(uint32(i.EntryPointIndex))
	marshalUtil.WriteUint32(uint32(i.PruningIndex))
	marshalUtil.WriteUint64(uint64(i.Timestamp.Unix()))
	marshalUtil.WriteByte(byte(i.Metadata))

	return marshalUtil.Bytes()
}

func (s *Storage) SetSnapshotMilestone(networkID byte, milestoneID *iotago.MilestoneID, snapshotIndex milestone.Index, entryPointIndex milestone.Index, pruningIndex milestone.Index, timestamp time.Time) {

	println(fmt.Sprintf(`SnapshotInfo:
	NetworkID: %d
	SnapshotIndex: %d (%v)
	EntryPointIndex: %d
	PruningIndex: %d
	Timestamp: %v`, networkID, snapshotIndex, hex.EncodeToString(milestoneID[:]), entryPointIndex, pruningIndex, timestamp.Truncate(time.Second)))

	sn := &SnapshotInfo{
		NetworkID:       networkID,
		MilestoneID:     milestoneID,
		SnapshotIndex:   snapshotIndex,
		EntryPointIndex: entryPointIndex,
		PruningIndex:    pruningIndex,
		Timestamp:       timestamp,
		Metadata:        bitmask.BitMask(0),
	}

	s.SetSnapshotInfo(sn)
}

func (s *Storage) SetSnapshotInfo(sn *SnapshotInfo) {
	s.snapshotMutex.Lock()
	defer s.snapshotMutex.Unlock()

	err := s.storeSnapshotInfo(sn)
	if err != nil {
		panic(err)
	}
	s.snapshot = sn
}

func (s *Storage) GetSnapshotInfo() *SnapshotInfo {
	s.snapshotMutex.RLock()
	defer s.snapshotMutex.RUnlock()

	return s.snapshot
}
