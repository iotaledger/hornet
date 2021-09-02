package node

import (
	"strings"
	"sync"

	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
)

type PluginStatus int

const (
	StatusDisabled PluginStatus = iota
	StatusEnabled
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
	// InitConfigPars gets called in the init stage of node initialization.
	// This can be used to provide config parameters even if the pluggable is disabled.
	InitConfigPars InitConfigParsFunc
	// PreProvide gets called before the provide stage of node initialization.
	// This can be used to force disable other pluggables before they get initialized.
	PreProvide PreProvideFunc
	// Provide gets called in the provide stage of node initialization (enabled pluggables only).
	Provide ProvideFunc
	// Configure gets called in the configure stage of node initialization (enabled pluggables only).
	Configure Callback
	// Run gets called in the run stage of node initialization (enabled pluggables only).
	Run Callback

	// The logger instance used in this plugin.
	log     *logger.Logger
	logOnce sync.Once
}

// Logger instantiates and returns a logger with the name of the plugin.
func (p *Pluggable) Logger() *logger.Logger {
	p.logOnce.Do(func() {
		p.log = logger.NewLogger(p.Name)
	})

	return p.log
}

func (p *Pluggable) Daemon() daemon.Daemon {
	return p.Node.Daemon()
}

func (p *Pluggable) Identifier() string {
	return strings.ToLower(strings.Replace(p.Name, " ", "", -1))
}

// LogDebug uses fmt.Sprint to construct and log a message.
func (p *Pluggable) LogDebug(args ...interface{}) {
	p.Logger().Debug(args...)
}

// LogDebugf uses fmt.Sprintf to log a templated message.
func (p *Pluggable) LogDebugf(template string, args ...interface{}) {
	p.Logger().Debugf(template, args...)
}

// LogError uses fmt.Sprint to construct and log a message.
func (p *Pluggable) LogError(args ...interface{}) {
	p.Logger().Error(args...)
}

// LogErrorf uses fmt.Sprintf to log a templated message.
func (p *Pluggable) LogErrorf(template string, args ...interface{}) {
	p.Logger().Errorf(template, args...)
}

// LogFatal uses fmt.Sprint to construct and log a message, then calls os.Exit.
func (p *Pluggable) LogFatal(args ...interface{}) {
	p.Logger().Fatal(args...)
}

// LogFatalf uses fmt.Sprintf to log a templated message, then calls os.Exit.
func (p *Pluggable) LogFatalf(template string, args ...interface{}) {
	p.Logger().Fatalf(template, args...)
}

// LogInfo uses fmt.Sprint to construct and log a message.
func (p *Pluggable) LogInfo(args ...interface{}) {
	p.Logger().Info(args...)
}

// LogInfof uses fmt.Sprintf to log a templated message.
func (p *Pluggable) LogInfof(template string, args ...interface{}) {
	p.Logger().Infof(template, args...)
}

// LogWarn uses fmt.Sprint to construct and log a message.
func (p *Pluggable) LogWarn(args ...interface{}) {
	p.Logger().Warn(args...)
}

// LogWarnf uses fmt.Sprintf to log a templated message.
func (p *Pluggable) LogWarnf(template string, args ...interface{}) {
	p.Logger().Warnf(template, args...)
}

// Panic uses fmt.Sprint to construct and log a message, then panics.
func (p *Pluggable) Panic(args ...interface{}) {
	p.Logger().Panic(args...)
}

// Panicf uses fmt.Sprintf to log a templated message, then panics.
func (p *Pluggable) Panicf(template string, args ...interface{}) {
	p.Logger().Panicf(template, args...)
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

type Plugin struct {
	Pluggable
	// The status of the plugin.
	Status PluginStatus
}
