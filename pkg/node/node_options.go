package node

import (
	"github.com/iotaledger/hive.go/daemon"
)

// the default options applied to the Node.
var defaultNodeOptions = []NodeOption{
	WithDaemon(daemon.New()),
}

// NodeOptions defines options for a Node.
type NodeOptions struct {
	daemon          daemon.Daemon
	enabledPlugins  []string
	disabledPlugins []string
	coreModules     []*CoreModule
	plugins         []*Plugin
}

// NodeOption is a function setting a NodeOptions option.
type NodeOption func(opts *NodeOptions)

// applies the given NodeOption.
func (no *NodeOptions) apply(opts ...NodeOption) {
	for _, opt := range opts {
		opt(no)
	}
}

// WithDaemon sets the used daemon.
func WithDaemon(daemon daemon.Daemon) NodeOption {
	return func(args *NodeOptions) {
		args.daemon = daemon
	}
}

// WithDisabledPlugins sets the disabled plugins.
func WithDisabledPlugins(disabledPlugins ...string) NodeOption {
	return func(args *NodeOptions) {
		args.disabledPlugins = append(args.disabledPlugins, disabledPlugins...)
	}
}

// WithEnabledPlugins sets the enabled plugins.
func WithEnabledPlugins(enabledPlugins ...string) NodeOption {
	return func(args *NodeOptions) {
		args.enabledPlugins = append(args.enabledPlugins, enabledPlugins...)
	}
}

// WithCoreModules sets the available core modules.
func WithCoreModules(coreModules ...*CoreModule) NodeOption {
	return func(args *NodeOptions) {
		args.coreModules = append(args.coreModules, coreModules...)
	}
}

// WithPlugins sets the available plugins.
func WithPlugins(plugins ...*Plugin) NodeOption {
	return func(args *NodeOptions) {
		args.plugins = append(args.plugins, plugins...)
	}
}
