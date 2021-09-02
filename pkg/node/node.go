package node

import (
	"fmt"
	"strings"

	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
)

type Node struct {
	enabledPlugins          map[string]struct{}
	disabledPlugins         map[string]struct{}
	forceDisabledPluggables map[string]struct{}
	corePluginsMap          map[string]*CorePlugin
	corePlugins             []*CorePlugin
	pluginsMap              map[string]*Plugin
	plugins                 []*Plugin
	container               *dig.Container
	log                     *logger.Logger
	options                 *NodeOptions
}

func New(optionalOptions ...NodeOption) *Node {
	nodeOpts := &NodeOptions{}
	nodeOpts.apply(defaultNodeOptions...)
	nodeOpts.apply(optionalOptions...)

	node := &Node{
		enabledPlugins:          make(map[string]struct{}),
		disabledPlugins:         make(map[string]struct{}),
		forceDisabledPluggables: make(map[string]struct{}),
		corePluginsMap:          make(map[string]*CorePlugin),
		corePlugins:             make([]*CorePlugin, 0),
		pluginsMap:              make(map[string]*Plugin),
		plugins:                 make([]*Plugin, 0),
		container:               dig.New(dig.DeferAcyclicVerification()),
		options:                 nodeOpts,
	}

	// initialize the core plugins and plugins
	node.init()

	// initialize logger after init phase because plugins could modify it
	node.log = logger.NewLogger("Node")

	// configure the core plugins and enabled plugins
	node.configure()

	return node
}

// LogDebug uses fmt.Sprint to construct and log a message.
func (n *Node) LogDebug(args ...interface{}) {
	n.log.Debug(args...)
}

// LogDebugf uses fmt.Sprintf to log a templated message.
func (n *Node) LogDebugf(template string, args ...interface{}) {
	n.log.Debugf(template, args...)
}

// LogError uses fmt.Sprint to construct and log a message.
func (n *Node) LogError(args ...interface{}) {
	n.log.Error(args...)
}

// LogErrorf uses fmt.Sprintf to log a templated message.
func (n *Node) LogErrorf(template string, args ...interface{}) {
	n.log.Errorf(template, args...)
}

// LogFatal uses fmt.Sprint to construct and log a message, then calls os.Exit.
func (n *Node) LogFatal(args ...interface{}) {
	n.log.Fatal(args...)
}

// LogFatalf uses fmt.Sprintf to log a templated message, then calls os.Exit.
func (n *Node) LogFatalf(template string, args ...interface{}) {
	n.log.Fatalf(template, args...)
}

// LogInfo uses fmt.Sprint to construct and log a message.
func (n *Node) LogInfo(args ...interface{}) {
	n.log.Info(args...)
}

// LogInfof uses fmt.Sprintf to log a templated message.
func (n *Node) LogInfof(template string, args ...interface{}) {
	n.log.Infof(template, args...)
}

// LogWarn uses fmt.Sprint to construct and log a message.
func (n *Node) LogWarn(args ...interface{}) {
	n.log.Warn(args...)
}

// LogWarnf uses fmt.Sprintf to log a templated message.
func (n *Node) LogWarnf(template string, args ...interface{}) {
	n.log.Warnf(template, args...)
}

// Panic uses fmt.Sprint to construct and log a message, then panics.
func (n *Node) Panic(args ...interface{}) {
	n.log.Panic(args...)
}

// Panicf uses fmt.Sprintf to log a templated message, then panics.
func (n *Node) Panicf(template string, args ...interface{}) {
	n.log.Panicf(template, args...)
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

func (n *Node) isEnabled(identifier string) bool {
	_, exists := n.enabledPlugins[identifier]
	return exists
}

func (n *Node) isDisabled(identifier string) bool {
	_, exists := n.disabledPlugins[identifier]
	return exists
}

func (n *Node) isForceDisabled(identifier string) bool {
	_, exists := n.forceDisabledPluggables[identifier]
	return exists
}

// IsSkipped returns whether the plugin is loaded or skipped.
func (n *Node) IsSkipped(plugin *Plugin) bool {
	// list of disabled plugins has the highest priority
	if n.isDisabled(plugin.Identifier()) || n.isForceDisabled(plugin.Identifier()) {
		return true
	}

	// if the plugin was not in the list of disabled plugins, it is only skipped if
	// the plugin was not enabled and not in the list of enabled plugins.
	return plugin.Status != StatusEnabled && !n.isEnabled(plugin.Identifier())
}

func (n *Node) init() {

	if n.options.initPlugin == nil {
		panic("you must configure the node with an InitPlugin")
	}

	params := map[string][]*flag.FlagSet{}
	masked := []string{}

	//
	// Collect parameters
	//
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

	//
	// Init Stage
	//

	// the init plugin parses the config files and initializes the global logger
	initCfg, err := n.options.initPlugin.Init(params, masked)
	if err != nil {
		panic(fmt.Errorf("unable to initialize node: %w", err))
	}

	//
	// InitConfig Stage
	//
	if n.options.initPlugin.InitConfigPars == nil {
		panic(fmt.Errorf("the init plugin must have an InitConfigPars func"))
	}
	n.options.initPlugin.InitConfigPars(n.container)

	forEachCorePlugin(n.options.corePlugins, func(corePlugin *CorePlugin) bool {
		if corePlugin.InitConfigPars != nil {
			corePlugin.InitConfigPars(n.container)
		}
		return true
	})

	forEachPlugin(n.options.plugins, func(plugin *Plugin) bool {
		if plugin.InitConfigPars != nil {
			plugin.InitConfigPars(n.container)
		}
		return true
	})

	//
	// Pre-Provide Stage
	//
	if n.options.initPlugin.PreProvide != nil {
		n.options.initPlugin.PreProvide(n.container, n.options.initPlugin.Configs, initCfg)
	}

	forEachCorePlugin(n.options.corePlugins, func(corePlugin *CorePlugin) bool {
		if corePlugin.PreProvide != nil {
			corePlugin.PreProvide(n.container, n.options.initPlugin.Configs, initCfg)
		}
		return true
	})

	forEachPlugin(n.options.plugins, func(plugin *Plugin) bool {
		if plugin.PreProvide != nil {
			plugin.PreProvide(n.container, n.options.initPlugin.Configs, initCfg)
		}
		return true
	})

	//
	// Enable / (Force-) disable Pluggables
	//
	for _, name := range initCfg.EnabledPlugins {
		n.enabledPlugins[strings.ToLower(name)] = struct{}{}
	}

	for _, name := range initCfg.DisabledPlugins {
		n.disabledPlugins[strings.ToLower(name)] = struct{}{}
	}

	for _, name := range initCfg.forceDisabledPluggables {
		n.forceDisabledPluggables[strings.ToLower(name)] = struct{}{}
	}

	forEachCorePlugin(n.options.corePlugins, func(corePlugin *CorePlugin) bool {
		if n.isForceDisabled(corePlugin.Identifier()) {
			return true
		}

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

	//
	// Provide Stage
	//
	if n.options.initPlugin.Provide != nil {
		n.options.initPlugin.Provide(n.container)
	}

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

	//
	// Invoke Stage
	//
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
		n.LogInfof("Loading core plugin: %s ... done", corePlugin.Name)
		return true
	})

	n.ForEachPlugin(func(plugin *Plugin) bool {
		if plugin.Configure != nil {
			plugin.Configure()
		}
		n.LogInfof("Loading plugin: %s ... done", plugin.Name)
		return true
	})
}

func (n *Node) execute() {
	n.LogInfo("Executing core plugin ...")

	if n.options.initPlugin.Run != nil {
		n.options.initPlugin.Run()
	}

	n.ForEachCorePlugin(func(corePlugin *CorePlugin) bool {
		if corePlugin.Run != nil {
			corePlugin.Run()
		}
		n.LogInfof("Starting core plugin: %s ... done", corePlugin.Name)
		return true
	})

	n.LogInfo("Executing plugins ...")

	n.ForEachPlugin(func(plugin *Plugin) bool {
		if plugin.Run != nil {
			plugin.Run()
		}
		n.LogInfof("Starting plugin: %s ... done", plugin.Name)
		return true
	})
}

func (n *Node) Start() {
	n.execute()

	n.LogInfo("Starting background workers ...")
	n.Daemon().Start()
}

func (n *Node) Run() {
	n.execute()

	n.LogInfo("Starting background workers ...")
	n.Daemon().Run()

	n.LogInfo("Shutdown complete!")
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

// InitConfigParsFunc gets called with a dig.Container.
type InitConfigParsFunc func(c *dig.Container)

// PreProvideFunc gets called with a dig.Container, the configs the InitPlugin brings to the node and the InitConfig.
type PreProvideFunc func(c *dig.Container, configs map[string]*configuration.Configuration, initConf *InitConfig)

// ProvideFunc gets called with a dig.Container.
type ProvideFunc func(c *dig.Container)

// InitConfig describes the result of a node initialization.
type InitConfig struct {
	EnabledPlugins          []string
	DisabledPlugins         []string
	forceDisabledPluggables []string
}

// ForceDisablePluggable is used to force disable pluggables before the provide stage.
func (ic *InitConfig) ForceDisablePluggable(identifier string) {
	exists := false
	for _, entry := range ic.forceDisabledPluggables {
		if entry == identifier {
			exists = true
			break
		}
	}

	if !exists {
		ic.forceDisabledPluggables = append(ic.forceDisabledPluggables, identifier)
	}
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
