package services

import (
	"fmt"
	"sync"

	"github.com/gohornet/hornet/packages/config"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
)

var gossipServiceKey service.Key
var gossipServiceKeyOnce sync.Once

func GossipServiceKey() service.Key {
	gossipServiceKeyOnce.Do(func() {
		cooAddr := config.NodeConfig.GetString(config.CfgMilestoneCoordinator)[:10]
		mwm := config.NodeConfig.GetInt(config.CfgProtocolMWM)
		gossipServiceKey = service.Key(fmt.Sprintf("%s%d", cooAddr, mwm))
	})
	return gossipServiceKey
}
