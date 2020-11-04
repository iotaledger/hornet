package node

import (
	"strings"
	"sync"

	"github.com/iotaledger/hive.go/daemon"
)

const (
	Disabled = iota
	Enabled
)

type Plugin struct {
	// A reference to the Node instance.
	Node *Node
	// The name of the plugin.
	Name string
	// The function to call to initialize the Plugin dependencies.
	DepsFunc interface{}
	// Provide gets called in the provide stage of node initialization.
	Provide ProvideFunc
	// Configure gets called in the configure stage of node initialization.
	Configure Callback
	// Run gets called in the run stage of node initialization.
	Run Callback
	// The status of the plugin.
	Status int
	wg     *sync.WaitGroup
}

func (p *Plugin) Daemon() daemon.Daemon {
	return p.Node.Daemon()
}

func (p *Plugin) GetIdentifier() string {
	return strings.ToLower(strings.Replace(p.Name, " ", "", -1))
}
