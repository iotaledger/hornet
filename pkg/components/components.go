package components

import (
	"go.uber.org/dig"
)

func IsAutopeeringEntryNodeDisabled(c *dig.Container) bool {
	type entryNodeDeps struct {
		dig.In
		AutopeeringRunAsEntryNode bool `name:"autopeeringRunAsEntryNode"`
	}

	runAsEntryNode := false
	if err := c.Invoke(func(deps entryNodeDeps) {
		runAsEntryNode = deps.AutopeeringRunAsEntryNode
	}); err != nil {
		panic(err)
	}

	return !runAsEntryNode
}
