package config

import (
	flag "github.com/spf13/pflag"
)

const (
	// protocol used to connect to the zmq feed [unix, tcp, udp, inproc]
	CfgZMQBindAddress = "zmq.bindAddress"
	// the bind address of the ZMQ feed
	CfgZMQProtocol = "zmq.protocol"
)

func init() {
	flag.String(CfgZMQProtocol, "tcp", "protocol used to connect to the zmq feed [unix, tcp, udp, inproc]")
	flag.String(CfgZMQBindAddress, "localhost:5556", "the bind address of the ZMQ feed")
}
