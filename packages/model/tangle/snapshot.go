package tangle

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/model/milestone_index"
)

const (
	SNAPSHOT_METADATA_SPENTADDRESSES_ENABLED = 0
)

var (
	snapshot                             *SnapshotInfo
	mutex                                syncutils.RWMutex
	latestSeenMilestoneIndexFromSnapshot = milestone_index.MilestoneIndex(0)

	ErrParseSnapshotInfoFailed = errors.New("Parsing of snapshot info failed")
)

type SnapshotInfo struct {
	Hash          trinary.Hash
	SnapshotIndex milestone_index.MilestoneIndex
	PruningIndex  milestone_index.MilestoneIndex
	Timestamp     int64
	Metadata      bitmask.BitMask
}

func loadSnapshotInfo() {
	info, err := readSnapshotInfoFromDatabase()
	if err != nil {
		panic(err)
	}
	snapshot = info
	if info != nil {
		println(fmt.Sprintf("SnapshotInfo: PruningIndex: %d, SnapshotIndex: %d (%v) Timestamp: %v, SpentAddressesEnabled: %v", info.PruningIndex, info.SnapshotIndex, info.Hash, time.Unix(info.Timestamp, 0).Truncate(time.Second), info.IsSpentAddressesEnabled()))
	}
}

func SnapshotInfoFromBytes(bytes []byte) (*SnapshotInfo, error) {

	if len(bytes) != 66 {
		return nil, errors.Wrapf(ErrParseSnapshotInfoFailed, "Invalid length %d != 66", len(bytes))
	}

	hash := trinary.MustBytesToTrytes(bytes[:49])
	snapshotIndex := milestone_index.MilestoneIndex(binary.LittleEndian.Uint32(bytes[49:53]))
	pruningIndex := milestone_index.MilestoneIndex(binary.LittleEndian.Uint32(bytes[53:57]))
	timestamp := int64(binary.LittleEndian.Uint64(bytes[57:65]))
	metadata := bitmask.BitMask(bytes[65])

	return &SnapshotInfo{
		Hash:          hash,
		SnapshotIndex: snapshotIndex,
		PruningIndex:  pruningIndex,
		Timestamp:     timestamp,
		Metadata:      metadata,
	}, nil
}

func (i *SnapshotInfo) IsSpentAddressesEnabled() bool {
	return i.Metadata.HasFlag(SNAPSHOT_METADATA_SPENTADDRESSES_ENABLED)
}

func (i *SnapshotInfo) SetSpentAddressesEnabled(enabled bool) {
	if enabled != i.Metadata.HasFlag(SNAPSHOT_METADATA_SPENTADDRESSES_ENABLED) {
		i.Metadata = i.Metadata.ModifyFlag(SNAPSHOT_METADATA_SPENTADDRESSES_ENABLED, enabled)
	}
}

func (i *SnapshotInfo) GetBytes() []byte {
	bytes := trinary.MustTrytesToBytes(i.Hash)[:49]

	snapshotIndexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(snapshotIndexBytes, uint32(i.SnapshotIndex))
	bytes = append(bytes, snapshotIndexBytes...)

	pruningIndexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(pruningIndexBytes, uint32(i.PruningIndex))
	bytes = append(bytes, pruningIndexBytes...)

	timestampBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestampBytes, uint64(i.Timestamp))
	bytes = append(bytes, timestampBytes...)

	bytes = append(bytes, byte(i.Metadata))

	return bytes
}

func SetSnapshotMilestone(milestoneHash trinary.Hash, snapshotIndex milestone_index.MilestoneIndex, pruningIndex milestone_index.MilestoneIndex, timestamp int64, spentAddressesEnabled bool) {
	println(fmt.Sprintf("Loaded solid milestone from snapshot %d (%v), pruning index: %d, Timestamp: %v, SpentAddressesEnabled: %v", snapshotIndex, milestoneHash, pruningIndex, time.Unix(timestamp, 0).Truncate(time.Second), spentAddressesEnabled))

	sn := &SnapshotInfo{
		Hash:          milestoneHash,
		SnapshotIndex: snapshotIndex,
		PruningIndex:  pruningIndex,
		Timestamp:     timestamp,
		Metadata:      bitmask.BitMask(0),
	}
	sn.SetSpentAddressesEnabled(spentAddressesEnabled)

	SetSnapshotInfo(sn)
}

func SetSnapshotInfo(sn *SnapshotInfo) {
	mutex.Lock()
	defer mutex.Unlock()

	err := storeSnapshotInfoInDatabase(sn)
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

func SetLatestSeenMilestoneIndexFromSnapshot(milestoneIndex milestone_index.MilestoneIndex) {
	if latestSeenMilestoneIndexFromSnapshot < milestoneIndex {
		latestSeenMilestoneIndexFromSnapshot = milestoneIndex
	}
}

func GetLatestSeenMilestoneIndexFromSnapshot() milestone_index.MilestoneIndex {
	return latestSeenMilestoneIndexFromSnapshot
}
