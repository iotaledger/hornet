package node

import (
	"strings"

	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/daemon"
)

// PluginParams defines the parameters configuration of a plugin.
type PluginParams struct {
	// The parameters of the plugin under for the defined configuration.
	Params map[string]*flag.FlagSet
	// The configuration values to mask.
	Masked []string
}

// Pluggable is something which extends the Node's capabilities.
type Pluggable struct {
	// A reference to the Node instance.
	Node *Node
	// The name of the plugin.
	Name string
	// The config parameters for this plugin.
	Params *PluginParams
	// The function to call to initialize the plugin dependencies.
	DepsFunc interface{}
	// Provide gets called in the provide stage of node initialization.
	Provide ProvideFunc
	// Configure gets called in the configure stage of node initialization.
	Configure Callback
	// Run gets called in the run stage of node initialization.
	Run Callback
}

// InitPlugin is the module initializing configuration of the node.
// A Node can only have one of such modules.
type InitPlugin struct {
	Pluggable
	// Init gets called in the initialization stage of the node.
	Init InitFunc
	// The configs this InitPlugin brings to the node.
	Configs map[string]*configuration.Configuration
}

// CorePlugin is a plugin essential for node operation.
// It can not be disabled.
type CorePlugin struct {
	Pluggable
}

func (c *CorePlugin) Daemon() daemon.Daemon {
	return c.Node.Daemon()
}

const (
	Disabled = iota
	Enabled
)

type Plugin struct {
	Pluggable
	// The status of the plugin.
	Status int
}

func (p *Plugin) Daemon() daemon.Daemon {
	return p.Node.Daemon()
}

func (p *Plugin) GetIdentifier() string {
	return strings.ToLower(strings.Replace(p.Name, " ", "", -1))
}
