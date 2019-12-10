package tipselection

import (
	"github.com/iotaledger/hive.go/events"
	"github.com/gohornet/hornet/packages/logger"
	"github.com/gohornet/hornet/packages/node"
	"github.com/iotaledger/hive.go/parameter"
)

var PLUGIN = node.NewPlugin("Tip-Sel", node.Enabled, configure, run)
var log = logger.NewLogger("Tip-Sel")

// config options
var maxDepth int
var belowMaxDepthTransactionLimit int

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
	maxDepth = parameter.NodeConfig.GetInt("tipsel.maxDepth")
	belowMaxDepthTransactionLimit = parameter.NodeConfig.GetInt("tipsel.belowMaxDepthTransactionLimit")
}

func run(run *node.Plugin) {
}
