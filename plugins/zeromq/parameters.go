package zeromq

import flag "github.com/spf13/pflag"

func init() {
	flag.String("zmq.protocol", "tcp", "protocol used to connect to the zmq feed [unix, tcp, udp, inproc]")
	flag.String("zmq.host", "127.0.0.1", "host used to connect to the zmq feed")
	flag.Int("zmq.port", 5556, "port used to connect to the zmq feed")
}
