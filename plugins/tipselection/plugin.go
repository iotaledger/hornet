package tipselection

import (
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/gohornet/hornet/packages/parameter"
)

var (
	PLUGIN = node.NewPlugin("Tip-Sel", node.Enabled, configure)
	log    *logger.Logger

	// config options
	maxDepth int
)

func WalkerStatsCaller(handler interface{}, params ...interface{}) {
	handler.(func(*TipSelStats))(params[0].(*TipSelStats))
}

var Events = tipselevents{
	TipSelPerformed: events.NewEvent(WalkerStatsCaller),
}

type tipselevents struct {
	TipSelPerformed *events.Event
}

func configure(node *node.Plugin) {
	log = logger.NewLogger("Tip-Sel")

	maxDepth = parameter.NodeConfig.GetInt("tipsel.maxDepth")
}
