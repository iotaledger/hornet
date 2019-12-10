package tcp

import "github.com/gohornet/hornet/packages/network"

type Callback = func()

type ErrorConsumer = func(e error)

type PeerConsumer = func(conn *network.ManagedConnection)
