package node

import (
	"fmt"
	"strings"

	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
)

type Node struct {
	disabledPlugins map[string]struct{}
	enabledPlugins  map[string]struct{}
	corePluginsMap  map[string]*CorePlugin
	corePlugins     []*CorePlugin
	pluginsMap      map[string]*Plugin
	plugins         []*Plugin
	container       *dig.Container
	Logger          *logger.Logger
	options         *NodeOptions
}

func New(optionalOptions ...NodeOption) *Node {
	nodeOpts := &NodeOptions{}
	nodeOpts.apply(defaultNodeOptions...)
	nodeOpts.apply(optionalOptions...)

	node := &Node{
		disabledPlugins: make(map[string]struct{}),
		enabledPlugins:  make(map[string]struct{}),
		corePluginsMap:  make(map[string]*CorePlugin),
		corePlugins:     make([]*CorePlugin, 0),
		pluginsMap:      make(map[string]*Plugin),
		plugins:         make([]*Plugin, 0),
		container:       dig.New(dig.DeferAcyclicVerification()),
		options:         nodeOpts,
	}

	// initialize the core plugins and plugins
	node.init()

	// initialize logger after init phase because plugins could modify it
	node.Logger = logger.NewLogger("Node")

	// configure the core plugins and enabled plugins
	node.configure()

	return node
}

func Start(optionalOptions ...NodeOption) *Node {
	node := New(optionalOptions...)
	node.Start()

	return node
}

func Run(optionalOptions ...NodeOption) *Node {
	node := New(optionalOptions...)
	node.Run()

	return node
}

// IsSkipped returns whether the plugin is loaded or skipped.
func (n *Node) IsSkipped(plugin *Plugin) bool {
	return (plugin.Status == Disabled || n.isDisabled(plugin)) &&
		(plugin.Status == Enabled || !n.isEnabled(plugin))
}

func (n *Node) isDisabled(plugin *Plugin) bool {
	_, exists := n.disabledPlugins[plugin.GetIdentifier()]
	return exists
}

func (n *Node) isEnabled(plugin *Plugin) bool {
	_, exists := n.enabledPlugins[plugin.GetIdentifier()]
	return exists
}

func (n *Node) init() {

	if n.options.initPlugin == nil {
		panic("you must configure the node with an InitPlugin")
	}

	params := map[string][]*flag.FlagSet{}
	masked := []string{}

	n.options.initPlugin.Node = n
	if n.options.initPlugin.Params != nil {
		for k, v := range n.options.initPlugin.Params.Params {
			params[k] = append(params[k], v)
		}
	}
	if n.options.initPlugin.Params.Masked != nil {
		masked = append(masked, n.options.initPlugin.Params.Masked...)
	}

	forEachCorePlugin(n.options.corePlugins, func(corePlugin *CorePlugin) bool {
		corePlugin.Node = n

		if corePlugin.Params == nil {
			return true
		}
		for k, v := range corePlugin.Params.Params {
			params[k] = append(params[k], v)
		}
		if corePlugin.Params.Masked != nil {
			masked = append(masked, corePlugin.Params.Masked...)
		}
		return true
	})

	forEachPlugin(n.options.plugins, func(plugin *Plugin) bool {
		plugin.Node = n

		if plugin.Params == nil {
			return true
		}
		for k, v := range plugin.Params.Params {
			params[k] = append(params[k], v)
		}
		if plugin.Params.Masked != nil {
			masked = append(masked, plugin.Params.Masked...)
		}
		return true
	})

	initCfg, err := n.options.initPlugin.Init(params, masked)
	if err != nil {
		panic(fmt.Errorf("unable to initialize node: %w", err))
	}

	for _, name := range initCfg.EnabledPlugins {
		n.enabledPlugins[strings.ToLower(name)] = struct{}{}
	}

	for _, name := range initCfg.DisabledPlugins {
		n.disabledPlugins[strings.ToLower(name)] = struct{}{}
	}

	forEachCorePlugin(n.options.corePlugins, func(corePlugin *CorePlugin) bool {
		n.addCorePlugin(corePlugin)
		return true
	})

	forEachPlugin(n.options.plugins, func(plugin *Plugin) bool {
		if n.IsSkipped(plugin) {
			return true
		}

		n.addPlugin(plugin)
		return true
	})

	if n.options.initPlugin.Provide == nil {
		panic(fmt.Errorf("the init plugin must have a provide func"))
	}
	n.options.initPlugin.Provide(n.container)

	n.ForEachCorePlugin(func(corePlugin *CorePlugin) bool {
		if corePlugin.Provide != nil {
			corePlugin.Provide(n.container)
		}
		return true
	})

	n.ForEachPlugin(func(plugin *Plugin) bool {
		if plugin.Provide != nil {
			plugin.Provide(n.container)
		}
		return true
	})

	if n.options.initPlugin.DepsFunc != nil {
		if err := n.container.Invoke(n.options.initPlugin.DepsFunc); err != nil {
			panic(err)
		}
	}

	n.ForEachCorePlugin(func(corePlugin *CorePlugin) bool {
		if corePlugin.DepsFunc != nil {
			if err := n.container.Invoke(corePlugin.DepsFunc); err != nil {
				panic(err)
			}
		}
		return true
	})

	n.ForEachPlugin(func(plugin *Plugin) bool {
		if plugin.DepsFunc != nil {
			if err := n.container.Invoke(plugin.DepsFunc); err != nil {
				panic(err)
			}
		}
		return true
	})
}

func (n *Node) configure() {

	if n.options.initPlugin.Configure != nil {
		n.options.initPlugin.Configure()
	}

	n.ForEachCorePlugin(func(corePlugin *CorePlugin) bool {
		if corePlugin.Configure != nil {
			corePlugin.Configure()
		}
		n.Logger.Infof("Loading core plugin: %s ... done", corePlugin.Name)
		return true
	})

	n.ForEachPlugin(func(plugin *Plugin) bool {
		if plugin.Configure != nil {
			plugin.Configure()
		}
		n.Logger.Infof("Loading plugin: %s ... done", plugin.Name)
		return true
	})
}

func (n *Node) execute() {
	n.Logger.Info("Executing core plugin ...")

	if n.options.initPlugin.Run != nil {
		n.options.initPlugin.Run()
	}

	n.ForEachCorePlugin(func(corePlugin *CorePlugin) bool {
		if corePlugin.Run != nil {
			corePlugin.Run()
		}
		n.Logger.Infof("Starting core plugin: %s ... done", corePlugin.Name)
		return true
	})

	n.Logger.Info("Executing plugins ...")

	n.ForEachPlugin(func(plugin *Plugin) bool {
		if plugin.Run != nil {
			plugin.Run()
		}
		n.Logger.Infof("Starting plugin: %s ... done", plugin.Name)
		return true
	})
}

func (n *Node) Start() {
	n.execute()

	n.Logger.Info("Starting background workers ...")
	n.Daemon().Start()
}

func (n *Node) Run() {
	n.execute()

	n.Logger.Info("Starting background workers ...")
	n.Daemon().Run()

	n.Logger.Info("Shutdown complete!")
}

func (n *Node) Shutdown() {
	n.Daemon().ShutdownAndWait()
}

func (n *Node) Daemon() daemon.Daemon {
	return n.options.daemon
}

func (n *Node) addCorePlugin(corePlugin *CorePlugin) {
	name := corePlugin.Name

	if _, exists := n.corePluginsMap[name]; exists {
		panic("duplicate core plugin - \"" + name + "\" was defined already")
	}

	n.corePluginsMap[name] = corePlugin
	n.corePlugins = append(n.corePlugins, corePlugin)
}

func (n *Node) addPlugin(plugin *Plugin) {
	name := plugin.Name

	if _, exists := n.pluginsMap[name]; exists {
		panic("duplicate plugin - \"" + name + "\" was defined already")
	}

	n.pluginsMap[name] = plugin
	n.plugins = append(n.plugins, plugin)
}

// ProvideFunc gets called with a dig.Container.
type ProvideFunc func(c *dig.Container)

// InitConfig describes the result of a node initialization.
type InitConfig struct {
	EnabledPlugins  []string
	DisabledPlugins []string
}

// InitFunc gets called as the initialization function of the node.
type InitFunc func(params map[string][]*flag.FlagSet, maskedKeys []string) (*InitConfig, error)

// Callback is a function called without any arguments.
type Callback func()

// CorePluginForEachFunc is used in ForEachCorePlugin.
// Returning false indicates to stop looping.
type CorePluginForEachFunc func(corePlugin *CorePlugin) bool

func forEachCorePlugin(corePlugins []*CorePlugin, f CorePluginForEachFunc) {
	for _, corePlugin := range corePlugins {
		if !f(corePlugin) {
			break
		}
	}
}

// ForEachCorePlugin calls the given CorePluginForEachFunc on each loaded core plugins.
func (n *Node) ForEachCorePlugin(f CorePluginForEachFunc) {
	forEachCorePlugin(n.corePlugins, f)
}

// PluginForEachFunc is used in ForEachPlugin.
// Returning false indicates to stop looping.
type PluginForEachFunc func(plugin *Plugin) bool

func forEachPlugin(plugins []*Plugin, f PluginForEachFunc) {
	for _, plugin := range plugins {
		if !f(plugin) {
			break
		}
	}
}

// ForEachPlugin calls the given PluginForEachFunc on each loaded plugin.
func (n *Node) ForEachPlugin(f PluginForEachFunc) {
	forEachPlugin(n.plugins, f)
}
