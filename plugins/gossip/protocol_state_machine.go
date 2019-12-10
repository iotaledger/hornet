package gossip

import (
	"github.com/iotaledger/iota.go/trinary"
	"github.com/pkg/errors"
	"github.com/gohornet/hornet/packages/byteutils"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/iotaledger/hive.go/parameter"
	"github.com/gohornet/hornet/plugins/gossip/server"
)

const (
	VERSION_1   = byte(1)
	SIZE_HEADER = 3
)

var (
	ownByteEncodedCooAddress []byte
	ownMWM                   uint64
	ownSrvSocketPort         uint16
)

func configureProtocol() {
	ownByteEncodedCooAddress = trinary.MustTrytesToBytes(parameter.NodeConfig.GetString("milestones.coordinator"))[:BYTE_ENCODED_COO_ADDRESS_BYTES_LENGTH]
	ownMWM = uint64(parameter.NodeConfig.GetInt("protocol.mwm"))
	ownSrvSocketPort = uint16(parameter.NodeConfig.GetInt("network.port"))
}

// region protocolV1 ///////////////////////////////////////////////////////////////////////////////////////////////////

func protocolV1(protocol *protocol) error {
	// This is the initializer of the protocol => handshaking with the other node
	handshakePaket, err := CreateHandshakePacket(ownSrvSocketPort, ownByteEncodedCooAddress, byte(ownMWM))
	if err != nil {
		return errors.Wrap(NewHandshakeError(err), "Can't create handshake packet")
	}
	if err := protocol.Send(handshakePaket); err != nil {
		return errors.Wrap(NewHandshakeError(err), "Can't send handshake packet")
	}

	return nil
}

func sendLegacyTransaction(protocol *protocol, truncatedTxData []byte, reqHash []byte) {

	server.SharedServerMetrics.IncrSentTransactionsCount()

	packet, err := CreateLegacyTransactionGossipPacket(truncatedTxData, reqHash)
	if err != nil {
		gossipLogger.Error(err.Error())
		return
	}

	protocol.sendMutex.Lock()
	defer protocol.sendMutex.Unlock()

	if _, ok := protocol.SendState.(*headerState); ok {
		if err := protocol.send(packet); err != nil {
			return
		}
		protocol.Neighbor.Metrics.IncrSentTransactionsCount()
	} else {
		gossipLogger.Warning("SendState was not in headerState. Message dropped")
	}
}

func sendTransaction(protocol *protocol, truncatedTxData []byte) {

	server.SharedServerMetrics.IncrSentTransactionsCount()

	packet, err := CreateTransactionGossipPacket(truncatedTxData)
	if err != nil {
		gossipLogger.Error(err.Error())
		return
	}

	protocol.sendMutex.Lock()
	defer protocol.sendMutex.Unlock()

	if _, ok := protocol.SendState.(*headerState); ok {
		if err := protocol.send(packet); err != nil {
			return
		}
		protocol.Neighbor.Metrics.IncrSentTransactionsCount()
	} else {
		gossipLogger.Warning("SendState was not in headerState. Message dropped")
	}
}

func sendTransactionRequest(protocol *protocol, reqHash []byte) {

	// TODO: add metric

	packet, err := CreateTransactionRequestGossipPacket(reqHash)
	if err != nil {
		gossipLogger.Error(err.Error())
		return
	}

	protocol.sendMutex.Lock()
	defer protocol.sendMutex.Unlock()

	if _, ok := protocol.SendState.(*headerState); ok {
		if err := protocol.send(packet); err != nil {
			return
		}
	} else {
		gossipLogger.Warning("SendState was not in headerState. Message dropped")
	}
}

func sendHeartbeat(protocol *protocol, hb *Heartbeat) {

	// TODO: add metric

	packet, err := CreateHeartbeatPacket(hb.SolidMilestoneIndex, hb.PrunedMilestoneIndex)
	if err != nil {
		gossipLogger.Error(err.Error())
		return
	}

	protocol.sendMutex.Lock()
	defer protocol.sendMutex.Unlock()

	if _, ok := protocol.SendState.(*headerState); ok {
		if err := protocol.send(packet); err != nil {
			return
		}
	} else {
		gossipLogger.Warning("SendState was not in headerState. Message dropped")
	}
}

func sendMilestoneRequest(protocol *protocol, reqMilestoneIndex milestone_index.MilestoneIndex) {

	server.SharedServerMetrics.IncrSentMilestoneRequestsCount()

	packet, err := CreateMilestoneRequestPacket(reqMilestoneIndex)
	if err != nil {
		gossipLogger.Error(err.Error())
		return
	}

	protocol.sendMutex.Lock()
	defer protocol.sendMutex.Unlock()

	if _, ok := protocol.SendState.(*headerState); ok {
		if err := protocol.send(packet); err != nil {
			return
		}
		//gossipLogger.Infof("REQUESTED MS: %d", reqMilestoneIndex)
	} else {
		gossipLogger.Warning("SendState was not in headerState. Message dropped")
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region headerState ///////////////////////////////////////////////////////////////////////////////////////

type headerState struct {
	protocol *protocol
	buffer   []byte
	offset   int
}

func newHeaderState(protocol *protocol) *headerState {
	return &headerState{
		protocol: protocol,
		buffer:   make([]byte, HEADER_TLV_BYTES_LENGTH),
		offset:   0,
	}
}

func (state *headerState) Receive(data []byte, offset int, length int) (int, error) {
	bytesRead := byteutils.ReadAvailableBytesToBuffer(state.buffer, state.offset, data, offset, length)

	state.offset += bytesRead
	if state.offset == SIZE_HEADER {
		header, err := ParseHeader(state.buffer)
		if err != nil {
			return bytesRead, errors.Wrap(NewHandshakeError(err), "invalid header")
		}

		protocol := state.protocol

		switch header.MsgType {

		case PROTOCOL_MSG_TYPE_LEGACY_TX_GOSSIP:
			protocol.ReceivingState = newLegacyTransactionGossipState(protocol, int(header.MessageLength))

		case PROTOCOL_MSG_TYPE_HANDSHAKE:
			protocol.ReceivingState = newHandshakeState(protocol, int(header.MessageLength))

		case PROTOCOL_MSG_TYPE_MS_REQUEST:
			protocol.ReceivingState = newRequestMilestoneState(protocol, int(header.MessageLength))

		case PROTOCOL_MSG_TYPE_TX_REQ_GOSSIP:
			protocol.ReceivingState = newTransactionRequestGossipState(protocol, int(header.MessageLength))

		case PROTOCOL_MSG_TYPE_TX_GOSSIP:
			protocol.ReceivingState = newTransactionGossipState(protocol, int(header.MessageLength))

		case PROTOCOL_MSG_TYPE_HEARTBEAT:
			protocol.ReceivingState = newHeartbeatState(protocol, int(header.MessageLength))

		default:
			return bytesRead, errors.Wrap(NewHandshakeError(err), "invalid protocol msg type")
		}

		state.offset = 0
	}

	return bytesRead, nil
}

func (state *headerState) Send(param interface{}) error {
	if data, ok := param.([]byte); ok {
		protocol := state.protocol

		if _, err := protocol.Conn.Write(data[:3]); err != nil {
			return errors.Wrap(NewSendError(err), "failed to send packet header")
		}

		switch ProtocolMsgType(data[0]) {
		case PROTOCOL_MSG_TYPE_LEGACY_TX_GOSSIP:
			protocol.SendState = newLegacyTransactionGossipState(protocol, 0)
		case PROTOCOL_MSG_TYPE_HANDSHAKE:
			protocol.SendState = newHandshakeState(protocol, 0)
		case PROTOCOL_MSG_TYPE_TX_GOSSIP:
			protocol.SendState = newTransactionGossipState(protocol, 0)
		case PROTOCOL_MSG_TYPE_TX_REQ_GOSSIP:
			protocol.SendState = newTransactionRequestGossipState(protocol, 0)
		case PROTOCOL_MSG_TYPE_HEARTBEAT:
			protocol.SendState = newHeartbeatState(protocol, 0)
		case PROTOCOL_MSG_TYPE_MS_REQUEST:
			protocol.SendState = newRequestMilestoneState(protocol, 0)
		}

		// send subsequent data immediately
		return protocol.SendState.Send(data[3:])
	}

	return errors.Wrap(ErrInvalidSendParam, "passed in parameter is not a valid packet header")
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region handshakeState ///////////////////////////////////////////////////////////////////////////////////////

type handshakeState struct {
	protocol *protocol
	size     int
	buffer   []byte
	offset   int
}

func newHandshakeState(protocol *protocol, size int) *handshakeState {
	return &handshakeState{
		protocol: protocol,
		size:     size,
		buffer:   make([]byte, size),
		offset:   0,
	}
}

func (state *handshakeState) Receive(data []byte, offset int, length int) (int, error) {
	bytesRead := byteutils.ReadAvailableBytesToBuffer(state.buffer, state.offset, data, offset, length)

	state.offset += bytesRead
	if state.offset != state.size {
		return bytesRead, nil
	}

	handshake, err := GetHandshakeFromByteSlice(state.buffer)
	if err != nil {
		return bytesRead, errors.Wrap(NewHandshakeError(err), "invalid authentication message")
	}

	protocol := state.protocol
	if err := finalizeHandshake(protocol, handshake); err != nil {
		return bytesRead, errors.Wrap(NewHandshakeError(err), "handshake failed")
	}

	protocol.ReceivingState = newHeaderState(protocol)
	state.offset = 0

	return bytesRead, nil
}

func (state *handshakeState) Send(param interface{}) error {

	if data, ok := param.([]byte); ok {
		protocol := state.protocol

		if _, err := protocol.Conn.Write(data); err != nil {
			return errors.Wrap(NewSendError(err), "failed to send handshake packet")
		}

		protocol.SentHandshake()
		protocol.SendState = newHeaderState(protocol)
		return nil
	}

	return errors.Wrap(ErrInvalidSendParam, "passed in parameter is not a valid handshake packet")
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region legacyTransactionGossipState ///////////////////////////////////////////////////////////////////////////////////////////

type legacyTransactionGossipState struct {
	protocol *protocol
	size     int
	buffer   []byte
	offset   int
}

func newLegacyTransactionGossipState(protocol *protocol, size int) *legacyTransactionGossipState {
	return &legacyTransactionGossipState{
		protocol: protocol,
		size:     size,
		buffer:   make([]byte, size),
		offset:   0,
	}
}

func (state *legacyTransactionGossipState) Receive(data []byte, offset int, length int) (int, error) {
	bytesRead := byteutils.ReadAvailableBytesToBuffer(state.buffer, state.offset, data, offset, length)

	state.offset += bytesRead
	if state.offset == state.size {
		protocol := state.protocol

		data := make([]byte, state.size)
		copy(data, state.buffer)

		protocol.Events.ReceivedLegacyTransactionGossipData.Trigger(state.protocol, data)
		protocol.ReceivingState = newHeaderState(protocol)
		state.offset = 0
	}

	return bytesRead, nil
}

func (state *legacyTransactionGossipState) Send(param interface{}) error {
	if tx, ok := param.([]byte); ok {
		protocol := state.protocol

		if _, err := protocol.Conn.Write(tx); err != nil {
			return errors.Wrap(NewSendError(err), "failed to send legacy transaction gossip packet")
		}
		protocol.SendState = newHeaderState(protocol)
		return nil
	}

	return errors.Wrap(ErrInvalidSendParam, "passed in parameter is not a valid legacy transaction gossip packet")
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region transactionGossipState ///////////////////////////////////////////////////////////////////////////////////////////

type transactionGossipState struct {
	protocol *protocol
	size     int
	buffer   []byte
	offset   int
}

func newTransactionGossipState(protocol *protocol, size int) *transactionGossipState {
	return &transactionGossipState{
		protocol: protocol,
		size:     size,
		buffer:   make([]byte, size),
		offset:   0,
	}
}

func (state *transactionGossipState) Receive(data []byte, offset int, length int) (int, error) {
	bytesRead := byteutils.ReadAvailableBytesToBuffer(state.buffer, state.offset, data, offset, length)

	state.offset += bytesRead
	if state.offset == state.size {
		protocol := state.protocol

		data := make([]byte, state.size)
		copy(data, state.buffer)

		protocol.Events.ReceivedTransactionGossipData.Trigger(state.protocol, data)
		protocol.ReceivingState = newHeaderState(protocol)
		state.offset = 0
	}

	return bytesRead, nil
}

func (state *transactionGossipState) Send(param interface{}) error {
	if tx, ok := param.([]byte); ok {
		protocol := state.protocol

		if _, err := protocol.Conn.Write(tx); err != nil {
			return errors.Wrap(NewSendError(err), "failed to send transaction gossip packet")
		}
		protocol.SendState = newHeaderState(protocol)
		return nil
	}

	return errors.Wrap(ErrInvalidSendParam, "passed in parameter is not a valid transaction gossip packet")
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region transactionRequestGossipState ///////////////////////////////////////////////////////////////////////////////////////////

type transactionRequestGossipState struct {
	protocol *protocol
	size     int
	buffer   []byte
	offset   int
}

func newTransactionRequestGossipState(protocol *protocol, size int) *transactionRequestGossipState {
	return &transactionRequestGossipState{
		protocol: protocol,
		size:     size,
		buffer:   make([]byte, size),
		offset:   0,
	}
}

func (state *transactionRequestGossipState) Receive(data []byte, offset int, length int) (int, error) {
	bytesRead := byteutils.ReadAvailableBytesToBuffer(state.buffer, state.offset, data, offset, length)

	state.offset += bytesRead
	if state.offset == state.size {
		protocol := state.protocol

		data := make([]byte, state.size)
		copy(data, state.buffer)

		protocol.Events.ReceivedTransactionRequestGossipData.Trigger(state.protocol, data)
		protocol.ReceivingState = newHeaderState(protocol)
		state.offset = 0
	}

	return bytesRead, nil
}

func (state *transactionRequestGossipState) Send(param interface{}) error {
	if tx, ok := param.([]byte); ok {
		protocol := state.protocol

		if _, err := protocol.Conn.Write(tx); err != nil {
			return errors.Wrap(NewSendError(err), "failed to send transaction request packet")
		}
		protocol.SendState = newHeaderState(protocol)
		return nil
	}

	return errors.Wrap(ErrInvalidSendParam, "passed in parameter is not a valid transaction request packet")
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region heartbeatState  ///////////////////////////////////////////////////////////////////////////////////////////

type heartbeatState struct {
	protocol *protocol
	size     int
	buffer   []byte
	offset   int
}

func newHeartbeatState(protocol *protocol, size int) *heartbeatState {
	return &heartbeatState{
		protocol: protocol,
		size:     size,
		buffer:   make([]byte, size),
		offset:   0,
	}
}

func (state *heartbeatState) Receive(data []byte, offset int, length int) (int, error) {
	bytesRead := byteutils.ReadAvailableBytesToBuffer(state.buffer, state.offset, data, offset, length)

	state.offset += bytesRead
	if state.offset == state.size {
		protocol := state.protocol

		data := make([]byte, state.size)
		copy(data, state.buffer)

		protocol.Events.ReceivedHeartbeatData.Trigger(state.protocol, data)
		protocol.ReceivingState = newHeaderState(protocol)
		state.offset = 0
	}

	return bytesRead, nil
}

func (state *heartbeatState) Send(param interface{}) error {
	if tx, ok := param.([]byte); ok {
		protocol := state.protocol

		if _, err := protocol.Conn.Write(tx); err != nil {
			return errors.Wrap(NewSendError(err), "failed to send Heartbeat packet")
		}
		protocol.SendState = newHeaderState(protocol)
		return nil
	}

	return errors.Wrap(ErrInvalidSendParam, "passed in parameter is not a valid Heartbeat packet")
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region requestMilestoneState ///////////////////////////////////////////////////////////////////////////////////////////

type requestMilestoneState struct {
	protocol *protocol
	size     int
	buffer   []byte
	offset   int
}

func newRequestMilestoneState(protocol *protocol, size int) *requestMilestoneState {
	return &requestMilestoneState{
		protocol: protocol,
		size:     size,
		buffer:   make([]byte, size),
		offset:   0,
	}
}

func (state *requestMilestoneState) Receive(data []byte, offset int, length int) (int, error) {
	bytesRead := byteutils.ReadAvailableBytesToBuffer(state.buffer, state.offset, data, offset, length)

	state.offset += bytesRead
	if state.offset == state.size {
		protocol := state.protocol

		milestoneRequestData := make([]byte, state.size)
		copy(milestoneRequestData, state.buffer)

		protocol.Events.ReceivedMilestoneRequestData.Trigger(state.protocol, milestoneRequestData)
		protocol.ReceivingState = newHeaderState(protocol)
		state.offset = 0
	}

	return bytesRead, nil
}

func (state *requestMilestoneState) Send(param interface{}) error {
	if tx, ok := param.([]byte); ok {
		protocol := state.protocol

		if _, err := protocol.Conn.Write(tx); err != nil {
			return errors.Wrap(NewSendError(err), "failed to send milestone request")
		}
		state.protocol.Neighbor.Metrics.IncrSentMilestoneRequestsCount()
		protocol.SendState = newHeaderState(protocol)
		return nil
	}

	return errors.Wrap(ErrInvalidSendParam, "passed in parameter is not a valid milestone request packet")
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
