package config

const (
	// protocol used to connect to the zmq feed [unix, tcp, udp, inproc]
	CfgZMQBindAddress = "zmq.bindAddress"
	// the bind address of the ZMQ feed
	CfgZMQProtocol = "zmq.protocol"
)

func init() {
	configFlagSet.String(CfgZMQProtocol, "tcp", "protocol used to connect to the zmq feed [unix, tcp, udp, inproc]")
	configFlagSet.String(CfgZMQBindAddress, "localhost:5556", "the bind address of the ZMQ feed")
}
