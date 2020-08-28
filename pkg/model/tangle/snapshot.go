package tangle

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
)

const (
	SnapshotMetadataSpentAddressesEnabled = 0
)

var (
	snapshot                             *SnapshotInfo
	mutex                                syncutils.RWMutex
	latestSeenMilestoneIndexFromSnapshot = milestone.Index(0)

	ErrParseSnapshotInfoFailed = errors.New("Parsing of snapshot info failed")
)

type SnapshotInfo struct {
	CoordinatorAddress hornet.Hash
	Hash               hornet.Hash
	SnapshotIndex      milestone.Index
	EntryPointIndex    milestone.Index
	PruningIndex       milestone.Index
	Timestamp          int64
	Metadata           bitmask.BitMask
}

func loadSnapshotInfo() {
	info, err := readSnapshotInfo()
	if err != nil {
		panic(err)
	}
	snapshot = info
	if info != nil {
		println(fmt.Sprintf(`SnapshotInfo:
	CooAddr: %v
	SnapshotIndex: %d (%v)
	EntryPointIndex: %d
	PruningIndex: %d
	Timestamp: %v
	SpentAddressesEnabled: %v`, info.CoordinatorAddress.Trytes(), info.SnapshotIndex, info.Hash.Trytes(), info.EntryPointIndex, info.PruningIndex, time.Unix(info.Timestamp, 0).Truncate(time.Second), info.IsSpentAddressesEnabled()))
	}
}

func SnapshotInfoFromBytes(bytes []byte) (*SnapshotInfo, error) {

	if len(bytes) != 119 {
		return nil, errors.Wrapf(ErrParseSnapshotInfoFailed, "Invalid length %d != 119", len(bytes))
	}

	cooAddr := hornet.Hash(bytes[:49])
	hash := hornet.Hash(bytes[49:98])
	snapshotIndex := milestone.Index(binary.LittleEndian.Uint32(bytes[98:102]))
	entryPointIndex := milestone.Index(binary.LittleEndian.Uint32(bytes[102:106]))
	pruningIndex := milestone.Index(binary.LittleEndian.Uint32(bytes[106:110]))
	timestamp := int64(binary.LittleEndian.Uint64(bytes[110:118]))
	metadata := bitmask.BitMask(bytes[118])

	return &SnapshotInfo{
		CoordinatorAddress: cooAddr,
		Hash:               hash,
		SnapshotIndex:      snapshotIndex,
		EntryPointIndex:    entryPointIndex,
		PruningIndex:       pruningIndex,
		Timestamp:          timestamp,
		Metadata:           metadata,
	}, nil
}

func (i *SnapshotInfo) IsSpentAddressesEnabled() bool {
	return i.Metadata.HasBit(SnapshotMetadataSpentAddressesEnabled)
}

func (i *SnapshotInfo) SetSpentAddressesEnabled(enabled bool) {
	if enabled != i.Metadata.HasBit(SnapshotMetadataSpentAddressesEnabled) {
		i.Metadata = i.Metadata.ModifyBit(SnapshotMetadataSpentAddressesEnabled, enabled)
	}
}

func (i *SnapshotInfo) GetBytes() []byte {
	var bytes []byte

	bytes = append(bytes, i.CoordinatorAddress[:49]...)
	bytes = append(bytes, i.Hash[:49]...)

	snapshotIndexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(snapshotIndexBytes, uint32(i.SnapshotIndex))
	bytes = append(bytes, snapshotIndexBytes...)

	entryPointIndexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(entryPointIndexBytes, uint32(i.EntryPointIndex))
	bytes = append(bytes, entryPointIndexBytes...)

	pruningIndexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(pruningIndexBytes, uint32(i.PruningIndex))
	bytes = append(bytes, pruningIndexBytes...)

	timestampBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestampBytes, uint64(i.Timestamp))
	bytes = append(bytes, timestampBytes...)

	bytes = append(bytes, byte(i.Metadata))

	return bytes
}

func SetSnapshotMilestone(coordinatorAddress hornet.Hash, milestoneHash hornet.Hash, snapshotIndex milestone.Index, entryPointIndex milestone.Index, pruningIndex milestone.Index, timestamp int64, spentAddressesEnabled bool) {

	println(fmt.Sprintf(`SnapshotInfo:
	CooAddr: %v
	SnapshotIndex: %d (%v)
	EntryPointIndex: %d
	PruningIndex: %d
	Timestamp: %v
	SpentAddressesEnabled: %v`, coordinatorAddress.Trytes(), snapshotIndex, milestoneHash.Trytes(), entryPointIndex, pruningIndex, time.Unix(timestamp, 0).Truncate(time.Second), spentAddressesEnabled))

	sn := &SnapshotInfo{
		CoordinatorAddress: coordinatorAddress,
		Hash:               milestoneHash,
		SnapshotIndex:      snapshotIndex,
		EntryPointIndex:    entryPointIndex,
		PruningIndex:       pruningIndex,
		Timestamp:          timestamp,
		Metadata:           bitmask.BitMask(0),
	}
	sn.SetSpentAddressesEnabled(spentAddressesEnabled)

	SetSnapshotInfo(sn)
}

func SetSnapshotInfo(sn *SnapshotInfo) {
	mutex.Lock()
	defer mutex.Unlock()

	err := storeSnapshotInfo(sn)
	if err != nil {
		panic(err)
	}
	snapshot = sn
}

func GetSnapshotInfo() *SnapshotInfo {
	mutex.RLock()
	defer mutex.RUnlock()

	return snapshot
}

func SetLatestSeenMilestoneIndexFromSnapshot(milestoneIndex milestone.Index) {
	if latestSeenMilestoneIndexFromSnapshot < milestoneIndex {
		latestSeenMilestoneIndexFromSnapshot = milestoneIndex
	}
}

func GetLatestSeenMilestoneIndexFromSnapshot() milestone.Index {
	return latestSeenMilestoneIndexFromSnapshot
}
