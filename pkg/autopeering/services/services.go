package services

import (
	"fmt"
	"sync"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
)

var gossipServiceKey service.Key
var gossipServiceKeyOnce sync.Once

func GossipServiceKey() service.Key {
	gossipServiceKeyOnce.Do(func() {
		cooAddr := config.NodeConfig.GetString(config.CfgCoordinatorAddress)[:10]
		mwm := config.NodeConfig.GetInt(config.CfgCoordinatorMWM)
		gossipServiceKey = service.Key(fmt.Sprintf("%s%d", cooAddr, mwm))
	})
	return gossipServiceKey
}
