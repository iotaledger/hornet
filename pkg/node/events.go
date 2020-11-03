package node

import (
	"github.com/iotaledger/hive.go/events"
	"go.uber.org/dig"
)

type coreModuleEvents struct {
	Init      *events.Event
	Configure *events.Event
	Run       *events.Event
}

type pluginEvents struct {
	Init      *events.Event
	Configure *events.Event
	Run       *events.Event
}

func containerCaller(handler interface{}, params ...interface{}) {
	handler.(func(container *dig.Container))(params[0].(*dig.Container))
}
