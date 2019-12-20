package zeromq

import (
	"github.com/gohornet/hornet/packages/parameter"
)

func init() {
	parameter.NodeConfig.SetDefault("zmq.protocol", "tcp")
	parameter.NodeConfig.SetDefault("zmq.host", "127.0.0.1")
	parameter.NodeConfig.SetDefault("zmq.port", 5556)
}
