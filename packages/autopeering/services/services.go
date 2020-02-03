package services

import (
	"sync"

	"github.com/gohornet/hornet/packages/parameter"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
)

var gossipServiceKey service.Key
var gossipServiceKeyOnce sync.Once

func GossipServiceKey() service.Key {
	gossipServiceKeyOnce.Do(func() {
		gossipServiceKey = service.Key(parameter.NodeConfig.GetString("milestones.coordinator")[:10])
	})
	return gossipServiceKey
}
