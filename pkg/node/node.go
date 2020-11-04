package node

import (
	"strings"
	"sync"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"go.uber.org/dig"
)

type Node struct {
	wg              *sync.WaitGroup
	disabledPlugins map[string]struct{}
	enabledPlugins  map[string]struct{}
	coreModulesMap  map[string]*CoreModule
	coreModules     []*CoreModule
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
		wg:              &sync.WaitGroup{},
		disabledPlugins: make(map[string]struct{}),
		enabledPlugins:  make(map[string]struct{}),
		coreModulesMap:  make(map[string]*CoreModule),
		coreModules:     make([]*CoreModule, 0),
		pluginsMap:      make(map[string]*Plugin),
		plugins:         make([]*Plugin, 0),
		container:       dig.New(dig.DeferAcyclicVerification()),
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
		if coreModule.Provide != nil {
			coreModule.Provide(n.container)
		}
		return true
	})

	n.ForEachCoreModule(func(coreModule *CoreModule) bool {
		if coreModule.DepsFunc != nil {
			if err := n.container.Invoke(coreModule.DepsFunc); err != nil {
				panic(err)
			}
		}
		return true
	})

	n.ForEachPlugin(func(plugin *Plugin) bool {
		if plugin.Provide != nil {
			plugin.Provide(n.container)
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

	n.ForEachCoreModule(func(coreModule *CoreModule) bool {
		coreModule.wg = n.wg
		coreModule.Node = n

		if coreModule.Configure != nil {
			coreModule.Configure()
		}
		n.Logger.Infof("Loading core module: %s ... done", coreModule.Name)
		return true
	})

	n.ForEachPlugin(func(plugin *Plugin) bool {
		plugin.wg = n.wg
		plugin.Node = n

		if plugin.Configure != nil {
			plugin.Configure()
		}
		n.Logger.Infof("Loading plugin: %s ... done", plugin.Name)
		return true
	})
}

func (n *Node) execute() {
	n.Logger.Info("Executing core modules ...")

	n.ForEachCoreModule(func(coreModule *CoreModule) bool {
		if coreModule.Run != nil {
			coreModule.Run()
		}
		n.Logger.Infof("Starting core module: %s ... done", coreModule.Name)
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

// ProvideFunc gets called with a dig.Container.
type ProvideFunc func(c *dig.Container)

// Callback is a function called without any arguments.
type Callback func()

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
