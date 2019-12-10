package gossip

import (
	"bytes"
	"encoding/binary"
	"errors"
	"time"
)

type HeaderState int32

const (
	// The amount of bytes used for the coo address sent in a handshake packet.
	BYTE_ENCODED_COO_ADDRESS_BYTES_LENGTH = 49

	HEADER_INIT   HeaderState = 0
	HEADER_FAILED HeaderState = 1
	HEADER_OK     HeaderState = 2
)

var (
	ErrVersionNotSupported = errors.New("Version not supported")
)

// Defines information exchanged up on a new connection with a Neighbor.
type Handshake struct {
	// The state of the handshaking.
	State HeaderState

	ServerSocketPort      uint16
	SentTimestamp         uint64
	ByteEncodedCooAddress []byte
	MWM                   byte
	SupportedVersions     []byte
}

// CheckNeighborSupportedVersion returns the highest supported protocol version by the neighbor
func (hs Handshake) CheckNeighborSupportedVersion() (version int, err error) {
	highestSupportedVersion := 0
	highestSupportedVersionByNeighbor := 0

	for i, ownSupportedVersion := range SUPPORTED_PROTOCOL_VERSIONS {
		// max check up to advertised versions by the neighbor
		if i >= len(hs.SupportedVersions) {
			break
		}

		// get versions matched by both
		supported := (hs.SupportedVersions[i] & ownSupportedVersion)

		// none supported
		if supported == 0 {
			continue
		}

		// iterate through all bits and find highest (more to the left is higher)
		highest := 0
		var j uint
		for j = 0; j < 8; j++ {
			if ((supported >> j) & 1) == 1 {
				highest = int(j + 1)
			}
		}
		highestSupportedVersion = highest + (i * 8)
	}

	// if the highest version is still 0, it means that we don't support
	// any protocol version the neighbor supports
	if highestSupportedVersion == 0 {
		// grab last byte denoting the highest versions.
		// a node will only hold version bytes if at least one version in that
		// byte is supported, therefore it's safe to assume, that the last byte contains
		// the highest supported version of a given node.

		lastVersionsByte := hs.SupportedVersions[len(hs.SupportedVersions)-1]

		// iterate through all bits and find highest (more to the left is higher)
		highest := 0
		var j uint
		for j = 0; j < 8; j++ {
			if ((lastVersionsByte >> j) & 0x01) == 0x01 {
				highest = int(j + 1)
			}
		}

		highestSupportedVersionByNeighbor = highest + ((len(hs.SupportedVersions) - 1) * 8)

		return highestSupportedVersionByNeighbor, ErrVersionNotSupported
	}

	return highestSupportedVersion, nil
}

// CreateHandshakePacket creates a new handshake packet.
// 	ownSourcePort the node's own server socket port number
//  return byte slice containing the handshake packet
func CreateHandshakePacket(ownSourcePort uint16, ownByteEncodedCooAddress []byte, ownUsedMWM byte) ([]byte, error) {

	maxLength := ProtocolHandshakeMsg.MaxLength
	payloadLengthBytes := (maxLength - (maxLength - 60) + uint16(len(SUPPORTED_PROTOCOL_VERSIONS)))
	buf := bytes.NewBuffer(make([]byte, 0, ProtocolHeaderMsg.MaxLength+payloadLengthBytes))

	err := AddProtocolHeader(buf, PROTOCOL_MSG_TYPE_HANDSHAKE, payloadLengthBytes)
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, binary.BigEndian, ownSourcePort)
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, binary.BigEndian, time.Now().UnixNano()/int64(time.Millisecond))
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, binary.BigEndian, ownByteEncodedCooAddress)
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, binary.BigEndian, ownUsedMWM)
	if err != nil {
		return nil, err
	}
	err = binary.Write(buf, binary.BigEndian, SUPPORTED_PROTOCOL_VERSIONS)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// GetHandshakeFromByteSlice parses the given message into a Handshake object.
//
// msg the buffer containing the handshake info
// return the Handshake object
func GetHandshakeFromByteSlice(msg []byte) (*Handshake, error) {
	var serverSocketPort uint16
	var sentTimestamp uint64
	byteEncodedCooAddress := make([]byte, BYTE_ENCODED_COO_ADDRESS_BYTES_LENGTH)
	var mwm byte
	supportedVersions := make([]byte, 8)

	r := bytes.NewReader(msg)

	err := binary.Read(r, binary.BigEndian, &serverSocketPort)
	if err != nil {
		return nil, err
	}

	err = binary.Read(r, binary.BigEndian, &sentTimestamp)
	if err != nil {
		return nil, err
	}

	err = binary.Read(r, binary.BigEndian, &byteEncodedCooAddress)
	if err != nil {
		return nil, err
	}

	err = binary.Read(r, binary.BigEndian, &mwm)
	if err != nil {
		return nil, err
	}

	_, err = r.Read(supportedVersions)

	hs := &Handshake{State: HEADER_OK, ServerSocketPort: serverSocketPort, SentTimestamp: sentTimestamp, ByteEncodedCooAddress: byteEncodedCooAddress, MWM: mwm, SupportedVersions: supportedVersions}
	return hs, nil
}
