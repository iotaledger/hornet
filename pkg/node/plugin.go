package node

import (
	"strings"
	"sync"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"go.uber.org/dig"
)

const (
	Disabled = iota
	Enabled
)

type PluginCallback = func(c *dig.Container)

type Plugin struct {
	Node   *Node
	Name   string
	Status int
	Events pluginEvents
	wg     *sync.WaitGroup
}

func (p *Plugin) Daemon() daemon.Daemon {
	return p.Node.Daemon()
}

func (p *Plugin) GetIdentifier() string {
	return strings.ToLower(strings.Replace(p.Name, " ", "", -1))
}

// Creates a new plugin with the given name, default status and callbacks.
// The last specified callback is the mandatory run callback, while all other callbacks are configure callbacks.
func NewPlugin(name string, status int, callbacks ...PluginCallback) *Plugin {
	plugin := &Plugin{
		Name:   name,
		Status: status,
		Events: pluginEvents{
			Init:      events.NewEvent(pluginCaller),
			Configure: events.NewEvent(pluginCaller),
			Run:       events.NewEvent(pluginCaller),
		},
	}

	switch len(callbacks) {
	case 0:
		// plugin doesn't have any callbacks (i.e. plugins that execute stuff on init())
	case 1:
		plugin.Events.Run.Attach(events.NewClosure(callbacks[0]))
	case 2:
		plugin.Events.Configure.Attach(events.NewClosure(callbacks[0]))
		plugin.Events.Run.Attach(events.NewClosure(callbacks[1]))
	default:
		panic("too many callbacks in NewPlugin(...)")
	}

	return plugin
}
