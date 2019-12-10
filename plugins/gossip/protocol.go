package gossip

import (
	"fmt"

	"github.com/iotaledger/hive.go/events"
	"github.com/gohornet/hornet/packages/network"
	"github.com/gohornet/hornet/packages/syncutils"
)

// region constants and variables //////////////////////////////////////////////////////////////////////////////////////

var DEFAULT_PROTOCOL = protocolDefinition{
	version:     VERSION_1,
	initializer: protocolV1,
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region protocol /////////////////////////////////////////////////////////////////////////////////////////////////////

type protocol struct {
	Conn                      *network.ManagedConnection
	Neighbor                  *Neighbor
	Version                   byte
	sendHandshakeCompleted    bool
	receiveHandshakeCompleted bool
	SendState                 protocolState
	ReceivingState            protocolState
	Events                    protocolEvents
	sendMutex                 syncutils.Mutex
	handshakeMutex            syncutils.Mutex
}

func newProtocol(conn *network.ManagedConnection) *protocol {
	protocol := &protocol{
		Conn: conn,
		Events: protocolEvents{
			HandshakeCompleted:                   events.NewEvent(events.ByteCaller),
			ReceivedLegacyTransactionGossipData:  events.NewEvent(dataCaller),
			ReceivedTransactionGossipData:        events.NewEvent(dataCaller),
			ReceivedTransactionRequestGossipData: events.NewEvent(dataCaller),
			ReceivedHeartbeatData:                events.NewEvent(dataCaller),
			ReceivedMilestoneRequestData:         events.NewEvent(dataCaller),
			Error:                                events.NewEvent(events.ErrorCaller),
		},
		sendHandshakeCompleted:    false,
		receiveHandshakeCompleted: false,
	}

	protocol.SendState = newHeaderState(protocol)
	protocol.ReceivingState = newHeaderState(protocol)

	return protocol
}

func (protocol *protocol) SupportsSTING() bool {
	return protocol.Version&PROTOCOL_VERSION_STING > 0
}

func (protocol *protocol) Init() {
	//  register event handlers
	onReceiveData := events.NewClosure(protocol.Receive)
	protocol.Conn.Events.ReceiveData.Attach(onReceiveData)
	protocol.Conn.Events.Close.Attach(events.NewClosure(func() {
		protocol.Conn.Events.ReceiveData.Detach(onReceiveData)
	}))

	// initialize default protocol
	if err := DEFAULT_PROTOCOL.initializer(protocol); err != nil {
		protocol.SendState = nil
		_ = protocol.Conn.Close()
		protocol.Events.Error.Trigger(err)
		return
	}

	// start reading from the connection
	_, _ = protocol.Conn.Read(make([]byte, 1656)) // Header (3) + Full Msg (1604) + ReqHash (49)
}

func (protocol *protocol) ReceivedHandshake() {
	protocol.handshakeMutex.Lock()
	defer protocol.handshakeMutex.Unlock()

	protocol.receiveHandshakeCompleted = true
	if protocol.sendHandshakeCompleted {
		// this will automatically move the neighbor from the in-flight pool
		// or if it is an inbound neighbor, to the connected pool
		protocol.Events.HandshakeCompleted.Trigger(protocol.Version)
	}
}

func (protocol *protocol) SentHandshake() {
	protocol.handshakeMutex.Lock()
	defer protocol.handshakeMutex.Unlock()

	protocol.sendHandshakeCompleted = true
	if protocol.receiveHandshakeCompleted {
		// protocol version is initialized as we received the handshake from the neighbor
		protocol.Events.HandshakeCompleted.Trigger(protocol.Version)
	}
}

func (protocol *protocol) Receive(data []byte) {
	offset := 0
	length := len(data)

	for offset < length && protocol.ReceivingState != nil {
		if readBytes, err := protocol.ReceivingState.Receive(data, offset, length); err != nil {
			println(fmt.Sprintf("ReceivingState error: %s", err.Error()))
			Events.Error.Trigger(err)

			_ = protocol.Conn.Close()

			return
		} else {
			offset += readBytes
		}
	}
}

func (protocol *protocol) Send(data interface{}) error {
	protocol.sendMutex.Lock()
	defer protocol.sendMutex.Unlock()

	return protocol.send(data)
}

func (protocol *protocol) send(data interface{}) error {
	if protocol.SendState != nil {
		if err := protocol.SendState.Send(data); err != nil {
			protocol.SendState = nil

			_ = protocol.Conn.Close()

			println(fmt.Sprintf("SendState error: %s", err.Error()))
			protocol.Events.Error.Trigger(err)

			return err
		}
	}

	return nil
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region types and interfaces /////////////////////////////////////////////////////////////////////////////////////////

type protocolState interface {
	Send(param interface{}) error
	Receive(data []byte, offset int, length int) (int, error)
}

type protocolDefinition struct {
	version     byte
	initializer func(*protocol) error
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
