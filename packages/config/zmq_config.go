package config

const (
	// protocol used to connect to the zmq feed [unix, tcp, udp, inproc]
	CfgZMQBindAddress = "zmq.bindAddress"
	// the bind address of the ZMQ feed
	CfgZMQProtocol    = "zmq.protocol"
)

func init() {
	NodeConfig.SetDefault(CfgZMQProtocol, "tcp")
	NodeConfig.SetDefault(CfgZMQBindAddress, "localhost:5556")
}
