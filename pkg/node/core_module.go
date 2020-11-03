package node

import (
	"sync"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"go.uber.org/dig"
)

type CoreModuleCallback = func(container *dig.Container)

type CoreModule struct {
	Node   *Node
	Name   string
	Events coreModuleEvents
	wg     *sync.WaitGroup
}

func (c *CoreModule) Daemon() daemon.Daemon {
	return c.Node.Daemon()
}

// Creates a new plugin with the given name, default status and callbacks.
// The last specified callback is the mandatory run callback, while all other callbacks are configure callbacks.
func NewCoreModule(name string, callbacks ...CoreModuleCallback) *CoreModule {
	coreModule := &CoreModule{
		Name: name,
		Events: coreModuleEvents{
			Init:      events.NewEvent(containerCaller),
			Configure: events.NewEvent(containerCaller),
			Run:       events.NewEvent(containerCaller),
		},
	}

	switch len(callbacks) {
	case 0:
		// plugin doesn't have any callbacks (i.e. plugins that execute stuff on init())
	case 1:
		coreModule.Events.Run.Attach(events.NewClosure(callbacks[0]))
	case 2:
		coreModule.Events.Configure.Attach(events.NewClosure(callbacks[0]))
		coreModule.Events.Run.Attach(events.NewClosure(callbacks[1]))
	default:
		panic("too many callbacks in NewCoreModule(...)")
	}

	return coreModule
}
