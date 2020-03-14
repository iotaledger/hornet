package services

import (
	"sync"

	"github.com/gohornet/hornet/packages/config"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
)

var gossipServiceKey service.Key
var gossipServiceKeyOnce sync.Once

func GossipServiceKey() service.Key {
	gossipServiceKeyOnce.Do(func() {
		gossipServiceKey = service.Key(config.NodeConfig.GetString(config.CfgMilestoneCoordinator)[:10])
	})
	return gossipServiceKey
}
