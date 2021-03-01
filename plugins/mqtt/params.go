package mqtt

import (
	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// the bind address on which the MQTT broker listens on
	CfgMQTTBindAddress = "mqtt.bindAddress"

	// the port of the WebSocket MQTT broker
	CfgMQTTWSPort = "mqtt.wsPort"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.String(CfgMQTTBindAddress, "localhost:1883", "the bind address on which the MQTT broker listens on")
			fs.String(CfgMQTTWSPort, "1888", "port of the WebSocket MQTT broker")
			return fs
		}(),
	},
	Masked: nil,
}
