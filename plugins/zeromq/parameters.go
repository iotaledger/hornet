package zeromq

import (
	"github.com/gohornet/hornet/packages/config"
)

func init() {
	// protocol used to connect to the zmq feed [unix, tcp, udp, inproc]
	config.NodeConfig.SetDefault(config.CfgZMQProtocol, "tcp")

	// the bind address of the ZMQ feed
	config.NodeConfig.SetDefault(config.CfgZMQBindAddress, "localhost:5556")
}
