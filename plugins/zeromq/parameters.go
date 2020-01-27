package zeromq

import (
	"github.com/gohornet/hornet/packages/parameter"
)

func init() {
	// "protocol used to connect to the zmq feed [unix, tcp, udp, inproc]"
	parameter.NodeConfig.SetDefault("zmq.protocol", "tcp")

	// "host used to connect to the zmq feed"
	parameter.NodeConfig.SetDefault("zmq.bindAddress", "127.0.0.1")

	// "port used to connect to the zmq feed"
	parameter.NodeConfig.SetDefault("zmq.port", 5556)
}
