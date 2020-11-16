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
	initPlugin  *InitPlugin
	daemon      daemon.Daemon
	corePlugins []*CorePlugin
	plugins     []*Plugin
}

// NodeOption is a function setting a NodeOptions option.
type NodeOption func(opts *NodeOptions)

// applies the given NodeOption.
func (no *NodeOptions) apply(opts ...NodeOption) {
	for _, opt := range opts {
		opt(no)
	}
}

// WithInitPlugin sets the init plugin.
func WithInitPlugin(initPlugin *InitPlugin) NodeOption {
	return func(opts *NodeOptions) {
		opts.initPlugin = initPlugin
	}
}

// WithDaemon sets the used daemon.
func WithDaemon(daemon daemon.Daemon) NodeOption {
	return func(args *NodeOptions) {
		args.daemon = daemon
	}
}

// WithCorePlugins sets the core plugins.
func WithCorePlugins(corePlugins ...*CorePlugin) NodeOption {
	return func(args *NodeOptions) {
		args.corePlugins = append(args.corePlugins, corePlugins...)
	}
}

// WithPlugins sets the plugins.
func WithPlugins(plugins ...*Plugin) NodeOption {
	return func(args *NodeOptions) {
		args.plugins = append(args.plugins, plugins...)
	}
}
