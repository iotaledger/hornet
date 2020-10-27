package tangle

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/bitmask"
	"github.com/iotaledger/hive.go/marshalutil"
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
	NetworkID          uint8
	MilestoneMessageID *hornet.MessageID
	SnapshotIndex      milestone.Index
	EntryPointIndex    milestone.Index
	PruningIndex       milestone.Index
	Timestamp          time.Time
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
	NetworkID: %d
	SnapshotIndex: %d (%v)
	EntryPointIndex: %d
	PruningIndex: %d
	Timestamp: %v`, info.NetworkID, info.SnapshotIndex, info.MilestoneMessageID.Hex(), info.EntryPointIndex, info.PruningIndex, info.Timestamp.Truncate(time.Second)))
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

	milestoneMessageID, err := marshalUtil.ReadBytes(32)
	if err != nil {
		return nil, err
	}

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
		NetworkID:          networkID,
		MilestoneMessageID: hornet.MessageIDFromBytes(milestoneMessageID),
		SnapshotIndex:      milestone.Index(snapshotIndex),
		EntryPointIndex:    milestone.Index(entryPointIndex),
		PruningIndex:       milestone.Index(pruningIndex),
		Timestamp:          time.Unix(int64(timestamp), 0),
		Metadata:           bitmask.BitMask(metadata),
	}, nil
}

func (i *SnapshotInfo) GetBytes() []byte {
	marshalUtil := marshalutil.New()

	marshalUtil.WriteByte(i.NetworkID)
	marshalUtil.WriteBytes(i.MilestoneMessageID[:32])
	marshalUtil.WriteUint32(uint32(i.SnapshotIndex))
	marshalUtil.WriteUint32(uint32(i.EntryPointIndex))
	marshalUtil.WriteUint32(uint32(i.PruningIndex))
	marshalUtil.WriteUint64(uint64(i.Timestamp.Unix()))
	marshalUtil.WriteByte(byte(i.Metadata))

	return marshalUtil.Bytes()
}

func SetSnapshotMilestone(networkID byte, milestoneMessageID *hornet.MessageID, snapshotIndex milestone.Index, entryPointIndex milestone.Index, pruningIndex milestone.Index, timestamp time.Time) {

	println(fmt.Sprintf(`SnapshotInfo:
	NetworkID: %d
	SnapshotIndex: %d (%v)
	EntryPointIndex: %d
	PruningIndex: %d
	Timestamp: %v`, networkID, snapshotIndex, milestoneMessageID.Hex(), entryPointIndex, pruningIndex, timestamp.Truncate(time.Second)))

	sn := &SnapshotInfo{
		NetworkID:          networkID,
		MilestoneMessageID: milestoneMessageID,
		SnapshotIndex:      snapshotIndex,
		EntryPointIndex:    entryPointIndex,
		PruningIndex:       pruningIndex,
		Timestamp:          timestamp,
		Metadata:           bitmask.BitMask(0),
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
