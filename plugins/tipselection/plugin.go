package tipselection

import (
	"math/rand"
	"time"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/gohornet/hornet/pkg/config"
)

var (
	PLUGIN = node.NewPlugin("Tip-Sel", node.Enabled, configure, run)
	log    *logger.Logger

	// config options
	maxDepth int
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func WalkerStatsCaller(handler interface{}, params ...interface{}) {
	handler.(func(*TipSelStats))(params[0].(*TipSelStats))
}

var Events = tipselevents{
	TipSelPerformed: events.NewEvent(WalkerStatsCaller),
}

type tipselevents struct {
	TipSelPerformed *events.Event
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	maxDepth = config.NodeConfig.GetInt(config.CfgTipSelMaxDepth)
}

func run(_ *node.Plugin) {
	// nothing
}
