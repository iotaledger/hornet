package tcp

import (
	"github.com/iotaledger/hive.go/events"
	"github.com/gohornet/hornet/packages/network"
)

type serverEvents struct {
	Start    *events.Event
	Shutdown *events.Event
	Connect  *events.Event
	Error    *events.Event
}

func managedConnectionCaller(handler interface{}, params ...interface{}) {
	handler.(func(*network.ManagedConnection))(params[0].(*network.ManagedConnection))
}
