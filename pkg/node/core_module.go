package node

import (
	"sync"

	"github.com/iotaledger/hive.go/daemon"
)

type CoreModule struct {
	// A reference to the Node instance.
	Node *Node
	// The name of the CoreModule.
	Name string
	// The function to call to initialize the CoreModule dependencies.
	DepsFunc interface{}
	// Provide gets called in the provide stage of node initialization.
	Provide ProvideFunc
	// Configure gets called in the configure stage of node initialization.
	Configure Callback
	// Run gets called in the run stage of node initialization.
	Run Callback
	wg  *sync.WaitGroup
}

func (c *CoreModule) Daemon() daemon.Daemon {
	return c.Node.Daemon()
}
