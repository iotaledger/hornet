package gossip

import (
	"github.com/iotaledger/iota.go/trinary"
	"github.com/iotaledger/hive.go/events"
	"github.com/gohornet/hornet/packages/iputils"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/network"
)

var Events = pluginEvents{
	// neighbor events
	RemovedNeighbor:                  events.NewEvent(neighborCaller),
	NeighborPutIntoConnectedPool:     events.NewEvent(neighborCaller),
	NeighborPutIntoInFlightPool:      events.NewEvent(neighborCaller),
	NeighborPutBackIntoReconnectPool: events.NewEvent(neighborCaller),
	NeighborPutIntoReconnectPool:     events.NewEvent(originAddrCaller),

	// low level network events
	IncomingConnection: events.NewEvent(connectionCaller),

	// high level protocol events
	NeighborHandshakeCompleted: events.NewEvent(handshakeCaller),
	SentTransaction:            events.NewEvent(hornet.TransactionCaller), // TODO
	SentTransactionRequest:     events.NewEvent(hornet.TransactionCaller), // TODO
	ReceivedTransaction:        events.NewEvent(hornet.TransactionCaller),
	ProtocolError:              events.NewEvent(hornet.TransactionCaller), // TODO

	// generic events
	Error: events.NewEvent(events.ErrorCaller),
}

type pluginEvents struct {
	// neighbor events
	RemovedNeighbor                  *events.Event
	NeighborPutIntoConnectedPool     *events.Event
	NeighborPutIntoInFlightPool      *events.Event
	NeighborPutBackIntoReconnectPool *events.Event
	NeighborPutIntoReconnectPool     *events.Event

	// low level network events
	IncomingConnection *events.Event

	// high level protocol events
	NeighborHandshakeCompleted *events.Event
	SentTransaction            *events.Event
	SentTransactionRequest     *events.Event
	ReceivedTransaction        *events.Event
	ProtocolError              *events.Event

	// generic events
	Error *events.Event
}

type protocolEvents struct {
	HandshakeCompleted                   *events.Event
	ReceivedLegacyTransactionGossipData  *events.Event
	ReceivedTransactionGossipData        *events.Event
	ReceivedTransactionRequestGossipData *events.Event
	ReceivedHeartbeatData                *events.Event
	ReceivedMilestoneRequestData         *events.Event
	Error                                *events.Event
}

type neighborEvents struct {
	ProtocolConnectionEstablished *events.Event
}

func originAddrCaller(handler interface{}, params ...interface{}) {
	handler.(func(*iputils.OriginAddress))(params[0].(*iputils.OriginAddress))
}

func connectionCaller(handler interface{}, params ...interface{}) {
	handler.(func(*network.ManagedConnection))(params[0].(*network.ManagedConnection))
}

func protocolCaller(handler interface{}, params ...interface{}) {
	handler.(func(*protocol))(params[0].(*protocol))
}

func neighborCaller(handler interface{}, params ...interface{}) {
	handler.(func(*Neighbor))(params[0].(*Neighbor))
}

func txHashAndMsIndexCaller(handler interface{}, params ...interface{}) {
	handler.(func(trinary.Hash, milestone_index.MilestoneIndex))(params[0].(trinary.Hash), params[1].(milestone_index.MilestoneIndex))
}

func handshakeCaller(handler interface{}, params ...interface{}) {
	handler.(func(*Neighbor, byte))(params[0].(*Neighbor), params[1].(byte))
}

func dataCaller(handler interface{}, params ...interface{}) {
	handler.(func(*protocol, []byte))(params[0].(*protocol), params[1].([]byte))
}
