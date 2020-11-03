package node

import (
	"strings"
	"sync"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
)

type Node struct {
	wg              *sync.WaitGroup
	disabledPlugins map[string]struct{}
	enabledPlugins  map[string]struct{}
	coreModulesMap  map[string]*CoreModule
	coreModules     []*CoreModule
	pluginsMap      map[string]*Plugin
	plugins         []*Plugin
	Logger          *logger.Logger
	options         *NodeOptions
}

func New(optionalOptions ...NodeOption) *Node {
	nodeOpts := &NodeOptions{}
	nodeOpts.apply(defaultNodeOptions...)
	nodeOpts.apply(optionalOptions...)

	node := &Node{
		wg:              &sync.WaitGroup{},
		disabledPlugins: make(map[string]struct{}),
		enabledPlugins:  make(map[string]struct{}),
		coreModulesMap:  make(map[string]*CoreModule),
		coreModules:     make([]*CoreModule, 0),
		pluginsMap:      make(map[string]*Plugin),
		plugins:         make([]*Plugin, 0),
		options:         nodeOpts,
	}

	// initialize the core modules and plugins
	node.init()

	// initialize logger after init phase because plugins could modify it
	node.Logger = logger.NewLogger("Node")

	// configure the core modules and enabled plugins
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

	for _, name := range n.options.enabledPlugins {
		n.enabledPlugins[strings.ToLower(name)] = struct{}{}
	}

	for _, name := range n.options.disabledPlugins {
		n.disabledPlugins[strings.ToLower(name)] = struct{}{}
	}

	forEachCoreModule(n.options.coreModules, func(coreModule *CoreModule) bool {
		n.addCoreModule(coreModule)
		return true
	})

	forEachPlugin(n.options.plugins, func(plugin *Plugin) bool {
		if n.IsSkipped(plugin) {
			return true
		}

		n.addPlugin(plugin)
		return true
	})

	n.ForEachCoreModule(func(coreModule *CoreModule) bool {
		coreModule.Events.Init.Trigger(coreModule)
		return true
	})

	n.ForEachPlugin(func(plugin *Plugin) bool {
		plugin.Events.Init.Trigger(plugin)
		return true
	})
}

func (n *Node) configure() {

	n.ForEachCoreModule(func(coreModule *CoreModule) bool {
		coreModule.wg = n.wg
		coreModule.Node = n

		coreModule.Events.Configure.Trigger(coreModule)
		n.Logger.Infof("Loading core module: %s ... done", coreModule.Name)

		return true
	})

	n.ForEachPlugin(func(plugin *Plugin) bool {
		plugin.wg = n.wg
		plugin.Node = n

		plugin.Events.Configure.Trigger(plugin)
		n.Logger.Infof("Loading plugin: %s ... done", plugin.Name)

		return true
	})
}

func (n *Node) execute() {
	n.Logger.Info("Executing core modules ...")

	n.ForEachCoreModule(func(coreModule *CoreModule) bool {
		coreModule.Events.Run.Trigger(coreModule)
		n.Logger.Infof("Starting core module: %s ... done", coreModule.Name)
		return true
	})

	n.Logger.Info("Executing plugins ...")

	n.ForEachPlugin(func(plugin *Plugin) bool {
		plugin.Events.Run.Trigger(plugin)
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

func (n *Node) addCoreModule(coreModule *CoreModule) {
	name := coreModule.Name

	if _, exists := n.coreModulesMap[name]; exists {
		panic("duplicate core module - \"" + name + "\" was defined already")
	}

	n.coreModulesMap[name] = coreModule
	n.coreModules = append(n.coreModules, coreModule)
}

func (n *Node) addPlugin(plugin *Plugin) {
	name := plugin.Name

	if _, exists := n.pluginsMap[name]; exists {
		panic("duplicate plugin - \"" + name + "\" was defined already")
	}

	n.pluginsMap[name] = plugin
	n.plugins = append(n.plugins, plugin)
}

// CoreModuleForEachFunc is used in ForEachCoreModule.
// Returning false indicates to stop looping.
type CoreModuleForEachFunc func(coreModule *CoreModule) bool

func forEachCoreModule(coreModules []*CoreModule, f CoreModuleForEachFunc) {
	for _, coreModule := range coreModules {
		if !f(coreModule) {
			break
		}
	}
}

// ForEachCoreModule calls the given CoreModuleForEachFunc on each loaded core module.
func (n *Node) ForEachCoreModule(f CoreModuleForEachFunc) {
	for _, coreModule := range n.coreModules {
		if !f(coreModule) {
			break
		}
	}
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
