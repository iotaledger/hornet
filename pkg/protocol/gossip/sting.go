package gossip

import (
	"bytes"
	"encoding/binary"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/protocol/message"
	"github.com/iotaledger/hive.go/protocol/tlv"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

var (
	// ErrInvalidSourceLength is returned when an invalid source byte slice for extraction of certain data is passed.
	ErrInvalidSourceLength = errors.New("invalid source byte slice")
)

const (
	MessageTypeMilestoneRequest message.Type = 1
	MessageTypeBlock            message.Type = 2
	MessageTypeBlockRequest     message.Type = 3
	MessageTypeHeartbeat        message.Type = 4
)

const (
	// requestedBlockIDMsgBytesLength the amount of bytes used for the requested block ID.
	requestedBlockIDMsgBytesLength = iotago.BlockIDLength

	// requestedMilestoneIndexMsgBytesLength the amount of bytes used for the requested milestone index.
	requestedMilestoneIndexMsgBytesLength = 4

	// heartbeatMilestoneIndexBytesLength the amount of bytes used for a milestone index within a heartbeat packet.
	heartbeatMilestoneIndexBytesLength = 4

	// latestMilestoneRequestIndex the index to use to request the latest milestone via a milestone request message.
	latestMilestoneRequestIndex = 0
)

var (
	// blockMessageDefinition defines a block message's format.
	blockMessageDefinition = &message.Definition{
		ID:             MessageTypeBlock,
		MaxBytesLength: iotago.BlockBinSerializedMaxSize,
		VariableLength: true,
	}

	// blockRequestMessageDefinition the requested block ID gossipping packet.
	// Contains only an ID of a requested block payload.
	blockRequestMessageDefinition = &message.Definition{
		ID:             MessageTypeBlockRequest,
		MaxBytesLength: requestedBlockIDMsgBytesLength,
		VariableLength: false,
	}

	// heartbeatMessageDefinition the heartbeat packet containing the current solid, pruned and latest milestone index,
	// number of connected peers and number of synced peers.
	heartbeatMessageDefinition = &message.Definition{
		ID:             MessageTypeHeartbeat,
		MaxBytesLength: heartbeatMilestoneIndexBytesLength*3 + 2,
		VariableLength: false,
	}

	// milestoneRequestMessageDefinition the requested milestone index packet.
	milestoneRequestMessageDefinition = &message.Definition{
		ID:             MessageTypeMilestoneRequest,
		MaxBytesLength: requestedMilestoneIndexMsgBytesLength,
		VariableLength: false,
	}
)

// newBlockMessage creates a new block message.
func newBlockMessage(blockData []byte) ([]byte, error) {
	blockBytesLength := uint16(len(blockData))
	buf := bytes.NewBuffer(make([]byte, 0, tlv.HeaderMessageDefinition.MaxBytesLength+blockBytesLength))

	if err := tlv.WriteHeader(buf, MessageTypeBlock, blockBytesLength); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, blockData); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// newBlockRequestMessage creates a block request message.
func newBlockRequestMessage(requestedBlockID iotago.BlockID) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, tlv.HeaderMessageDefinition.MaxBytesLength+blockRequestMessageDefinition.MaxBytesLength))
	if err := tlv.WriteHeader(buf, MessageTypeBlockRequest, blockRequestMessageDefinition.MaxBytesLength); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, requestedBlockID[:requestedBlockIDMsgBytesLength]); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// newHeartbeatMessage creates a new heartbeat message.
func newHeartbeatMessage(solidMilestoneIndex milestone.Index, prunedMilestoneIndex milestone.Index, latestMilestoneIndex milestone.Index, connectedPeers uint8, syncedPeers uint8) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, tlv.HeaderMessageDefinition.MaxBytesLength+heartbeatMessageDefinition.MaxBytesLength))
	if err := tlv.WriteHeader(buf, MessageTypeHeartbeat, heartbeatMessageDefinition.MaxBytesLength); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, solidMilestoneIndex); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, prunedMilestoneIndex); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, latestMilestoneIndex); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, connectedPeers); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, syncedPeers); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// newMilestoneRequestMessage creates a new milestone request message.
func newMilestoneRequestMessage(requestedMilestoneIndex milestone.Index) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, tlv.HeaderMessageDefinition.MaxBytesLength+milestoneRequestMessageDefinition.MaxBytesLength))
	if err := tlv.WriteHeader(buf, MessageTypeMilestoneRequest, milestoneRequestMessageDefinition.MaxBytesLength); err != nil {
		return nil, err
	}

	if err := binary.Write(buf, binary.LittleEndian, requestedMilestoneIndex); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// extractRequestedMilestoneIndex extracts the requested milestone index from the given source.
func extractRequestedMilestoneIndex(source []byte) (milestone.Index, error) {
	if len(source) != serializer.UInt32ByteSize {
		return 0, ErrInvalidSourceLength
	}

	msIndex := binary.LittleEndian.Uint32(source)
	return milestone.Index(msIndex), nil
}

// Heartbeat contains information about a nodes current solid and pruned milestone index
// and its connected and synced neighbors count.
type Heartbeat struct {
	SolidMilestoneIndex  milestone.Index `json:"solidMilestoneIndex"`
	PrunedMilestoneIndex milestone.Index `json:"prunedMilestoneIndex"`
	LatestMilestoneIndex milestone.Index `json:"latestMilestoneIndex"`
	ConnectedNeighbors   int             `json:"connectedNeighbors"`
	SyncedNeighbors      int             `json:"syncedNeighbors"`
}

// ParseHeartbeat parses the given message into a heartbeat.
func ParseHeartbeat(data []byte) *Heartbeat {
	return &Heartbeat{
		SolidMilestoneIndex:  milestone.Index(binary.LittleEndian.Uint32(data[:4])),
		PrunedMilestoneIndex: milestone.Index(binary.LittleEndian.Uint32(data[4:8])),
		LatestMilestoneIndex: milestone.Index(binary.LittleEndian.Uint32(data[8:12])),
		ConnectedNeighbors:   int(data[12]),
		SyncedNeighbors:      int(data[13]),
	}
}

func heartbeatCaller(handler interface{}, params ...interface{}) {
	handler.(func(heartbeat *Heartbeat))(params[0].(*Heartbeat))
}
