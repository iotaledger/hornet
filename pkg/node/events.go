package node

import (
	"github.com/iotaledger/hive.go/events"
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

func coreModuleCaller(handler interface{}, params ...interface{}) {
	handler.(func(*CoreModule))(params[0].(*CoreModule))
}

func pluginCaller(handler interface{}, params ...interface{}) {
	handler.(func(*Plugin))(params[0].(*Plugin))
}
