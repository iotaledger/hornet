package services

import (
	"fmt"
	"sync"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
)

var (
	gossipServiceKey     service.Key
	gossipServiceKeyOnce sync.Once
)

func GossipServiceKey(networkID uint8) service.Key {
	gossipServiceKeyOnce.Do(func() {
		mwm := byte(config.NodeConfig.Int(config.CfgCoordinatorMWM) & 0xFF)
		gossipServiceKey = service.Key(fmt.Sprintf("%d%d", networkID, mwm))
	})
	return gossipServiceKey
}
