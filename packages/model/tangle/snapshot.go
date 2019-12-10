package tangle

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/model/milestone_index"
)

var (
	snapshot                             *SnapshotInfo
	latestSeenMilestoneIndexFromSnapshot = milestone_index.MilestoneIndex(0)
)

type SnapshotInfo struct {
	Hash         trinary.Hash
	LedgerIndex  milestone_index.MilestoneIndex
	PruningIndex milestone_index.MilestoneIndex
	Timestamp    int64
}

func loadSnapshotInfo() {
	info, err := readSnapshotInfoFromDatabase()
	if err != nil {
		panic(err)
	}
	snapshot = info
	if info != nil {
		println(fmt.Sprintf("SnapshotInfo: %d (%v) Timestamp: %v", info.LedgerIndex, info.Hash, time.Unix(info.Timestamp, 0).Truncate(time.Second)))
	}
}

func SnapshotInfoFromBytes(bytes []byte) (*SnapshotInfo, error) {

	if len(bytes) != 65 {
		return nil, fmt.Errorf("Invalid length %d != 61", len(bytes))
	}

	hash := trinary.MustBytesToTrytes(bytes[:49])
	ledgerIndex := milestone_index.MilestoneIndex(binary.LittleEndian.Uint32(bytes[49:53]))
	pruningIndex := milestone_index.MilestoneIndex(binary.LittleEndian.Uint32(bytes[53:57]))
	timestamp := int64(binary.LittleEndian.Uint64(bytes[57:65]))

	return &SnapshotInfo{
		Hash:         hash,
		LedgerIndex:  ledgerIndex,
		PruningIndex: pruningIndex,
		Timestamp:    timestamp,
	}, nil
}

func (i *SnapshotInfo) GetBytes() []byte {
	bytes := trinary.MustTrytesToBytes(i.Hash)

	ledgerIndexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(ledgerIndexBytes, uint32(i.LedgerIndex))
	bytes = append(bytes, ledgerIndexBytes...)

	pruningIndexBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(pruningIndexBytes, uint32(i.PruningIndex))
	bytes = append(bytes, pruningIndexBytes...)

	timestampBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestampBytes, uint64(i.Timestamp))
	bytes = append(bytes, timestampBytes...)

	return bytes
}

func SetSnapshotMilestone(milestoneHash trinary.Hash, ledgerIndex milestone_index.MilestoneIndex, pruningIndex milestone_index.MilestoneIndex, timestamp int64) {
	println(fmt.Sprintf("Loaded solid milestone from snapshot %d (%v), pruning index: %d, Timestamp: %v", ledgerIndex, milestoneHash, pruningIndex, time.Unix(timestamp, 0).Truncate(time.Second)))
	sn := &SnapshotInfo{
		Hash:         milestoneHash,
		LedgerIndex:  ledgerIndex,
		PruningIndex: pruningIndex,
		Timestamp:    timestamp,
	}
	err := storeSnapshotInfoInDatabase(sn)
	if err != nil {
		panic(err)
	}
	snapshot = sn
}

func GetSnapshotInfo() *SnapshotInfo {
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
