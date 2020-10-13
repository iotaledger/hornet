package tangle

import (
	"crypto/ed25519"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/syncutils"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
)

var (
	snapshot *SnapshotInfo
	mutex    syncutils.RWMutex

	ErrParseSnapshotInfoFailed = errors.New("Parsing of snapshot info failed")
)

type SnapshotInfo struct {
	CoordinatorPublicKey ed25519.PublicKey
	MilestoneMessageID   *hornet.MessageID
	SnapshotIndex        milestone.Index
	EntryPointIndex      milestone.Index
	PruningIndex         milestone.Index
	Timestamp            time.Time
	Metadata             bitmask.BitMask
}

func loadSnapshotInfo() {
	info, err := readSnapshotInfo()
	if err != nil {
		panic(err)
	}
	snapshot = info
	if info != nil {
		println(fmt.Sprintf(`SnapshotInfo:
	CoordinatorPublicKey: %v
	SnapshotIndex: %d (%v)
	EntryPointIndex: %d
	PruningIndex: %d
	Timestamp: %v`, hex.EncodeToString(info.CoordinatorPublicKey), info.SnapshotIndex, info.MilestoneMessageID.Hex(), info.EntryPointIndex, info.PruningIndex, info.Timestamp.Truncate(time.Second)))
	}
}

func SnapshotInfoFromBytes(bytes []byte) (*SnapshotInfo, error) {

	if len(bytes) != ed25519.PublicKeySize+53 {
		return nil, errors.Wrapf(ErrParseSnapshotInfoFailed, "Invalid length %d != %d", len(bytes), ed25519.PublicKeySize+53)
	}

	cooPublicKey := ed25519.PublicKey(bytes[:ed25519.PublicKeySize])
	milestoneMessageID := hornet.MessageIDFromBytes(bytes[ed25519.PublicKeySize : ed25519.PublicKeySize+32])
	snapshotIndex := milestone.Index(binary.LittleEndian.Uint32(bytes[ed25519.PublicKeySize+32 : ed25519.PublicKeySize+36]))
	entryPointIndex := milestone.Index(binary.LittleEndian.Uint32(bytes[ed25519.PublicKeySize+36 : ed25519.PublicKeySize+40]))
	pruningIndex := milestone.Index(binary.LittleEndian.Uint32(bytes[ed25519.PublicKeySize+40 : ed25519.PublicKeySize+44]))
	timestamp := int64(binary.LittleEndian.Uint64(bytes[ed25519.PublicKeySize+44 : ed25519.PublicKeySize+52]))
	metadata := bitmask.BitMask(bytes[ed25519.PublicKeySize+52])

	return &SnapshotInfo{
		CoordinatorPublicKey: cooPublicKey,
		MilestoneMessageID:   milestoneMessageID,
		SnapshotIndex:        snapshotIndex,
		EntryPointIndex:      entryPointIndex,
		PruningIndex:         pruningIndex,
		Timestamp:            time.Unix(timestamp, 0),
		Metadata:             metadata,
	}, nil
}

func (i *SnapshotInfo) GetBytes() []byte {
	var bytes []byte

	bytes = append(bytes, i.CoordinatorPublicKey[:ed25519.PublicKeySize]...)
	bytes = append(bytes, i.MilestoneMessageID[:32]...)

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
	binary.LittleEndian.PutUint64(timestampBytes, uint64(i.Timestamp.Unix()))
	bytes = append(bytes, timestampBytes...)

	bytes = append(bytes, byte(i.Metadata))

	return bytes
}

func SetSnapshotMilestone(coordinatorPublicKey ed25519.PublicKey, milestoneMessageID *hornet.MessageID, snapshotIndex milestone.Index, entryPointIndex milestone.Index, pruningIndex milestone.Index, timestamp time.Time) {

	println(fmt.Sprintf(`SnapshotInfo:
	CoordinatorPublicKey: %v
	SnapshotIndex: %d (%v)
	EntryPointIndex: %d
	PruningIndex: %d
	Timestamp: %v`, hex.EncodeToString(coordinatorPublicKey), snapshotIndex, milestoneMessageID.Hex(), entryPointIndex, pruningIndex, timestamp.Truncate(time.Second)))

	sn := &SnapshotInfo{
		CoordinatorPublicKey: coordinatorPublicKey,
		MilestoneMessageID:   milestoneMessageID,
		SnapshotIndex:        snapshotIndex,
		EntryPointIndex:      entryPointIndex,
		PruningIndex:         pruningIndex,
		Timestamp:            timestamp,
		Metadata:             bitmask.BitMask(0),
	}

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
