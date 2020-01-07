package tangle

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/model/milestone_index"
)

var (
	snapshot                             *SnapshotInfo
	mutex                                syncutils.RWMutex
	latestSeenMilestoneIndexFromSnapshot = milestone_index.MilestoneIndex(0)
)

type SnapshotInfo struct {
	Hash          trinary.Hash
	SnapshotIndex milestone_index.MilestoneIndex
	PruningIndex  milestone_index.MilestoneIndex
	Timestamp     int64
}

func loadSnapshotInfo() {
	info, err := readSnapshotInfoFromDatabase()
	if err != nil {
		panic(err)
	}
	snapshot = info
	if info != nil {
		println(fmt.Sprintf("SnapshotInfo: %d (%v) Timestamp: %v", info.SnapshotIndex, info.Hash, time.Unix(info.Timestamp, 0).Truncate(time.Second)))
	}
}

func SnapshotInfoFromBytes(bytes []byte) (*SnapshotInfo, error) {

	if len(bytes) != 65 {
		return nil, fmt.Errorf("Invalid length %d != 61", len(bytes))
	}

	hash := trinary.MustBytesToTrytes(bytes[:49])
	snapshotIndex := milestone_index.MilestoneIndex(binary.LittleEndian.Uint32(bytes[49:53]))
	pruningIndex := milestone_index.MilestoneIndex(binary.LittleEndian.Uint32(bytes[53:57]))
	timestamp := int64(binary.LittleEndian.Uint64(bytes[57:65]))

	return &SnapshotInfo{
		Hash:          hash,
		SnapshotIndex: snapshotIndex,
		PruningIndex:  pruningIndex,
		Timestamp:     timestamp,
	}, nil
}

func (i *SnapshotInfo) GetBytes() []byte {
	bytes := trinary.MustTrytesToBytes(i.Hash)

	snapshotIndexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(snapshotIndexBytes, uint32(i.SnapshotIndex))
	bytes = append(bytes, snapshotIndexBytes...)

	pruningIndexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(pruningIndexBytes, uint32(i.PruningIndex))
	bytes = append(bytes, pruningIndexBytes...)

	timestampBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestampBytes, uint64(i.Timestamp))
	bytes = append(bytes, timestampBytes...)

	return bytes
}

func SetSnapshotMilestone(milestoneHash trinary.Hash, snapshotIndex milestone_index.MilestoneIndex, pruningIndex milestone_index.MilestoneIndex, timestamp int64) {
	println(fmt.Sprintf("Loaded solid milestone from snapshot %d (%v), pruning index: %d, Timestamp: %v", snapshotIndex, milestoneHash, pruningIndex, time.Unix(timestamp, 0).Truncate(time.Second)))
	sn := &SnapshotInfo{
		Hash:          milestoneHash,
		SnapshotIndex: snapshotIndex,
		PruningIndex:  pruningIndex,
		Timestamp:     timestamp,
	}
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
