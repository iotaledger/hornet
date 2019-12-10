package zeromq

import flag "github.com/spf13/pflag"

func init() {
	flag.Int("zmq.port", 5556, "tcp port used to connect to the zmq feed")
}
