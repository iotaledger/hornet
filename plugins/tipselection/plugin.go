package tipselection

import (
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/parameter"

	"github.com/gohornet/hornet/packages/node"
)

var (
	PLUGIN = node.NewPlugin("Tip-Sel", node.Enabled, configure)
	log    *logger.Logger

	// config options
	maxDepth                      int
	belowMaxDepthTransactionLimit int
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
	log = logger.NewLogger("Tip-Sel", logger.LogLevel(parameter.NodeConfig.GetInt("node.logLevel")))

	maxDepth = parameter.NodeConfig.GetInt("tipsel.maxDepth")
	belowMaxDepthTransactionLimit = parameter.NodeConfig.GetInt("tipsel.belowMaxDepthTransactionLimit")
}
