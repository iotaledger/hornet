package mqtt

import (
	"github.com/gohornet/hornet/pkg/node"
	flag "github.com/spf13/pflag"
)

const (
	// host of the MQTT broker
	CfgMQTTHost = "mqtt.host"

	// port of the MQTT broker
	CfgMQTTPort = "mqtt.port"

	//port of the WebSocket MQTT broker
	CfgMQTTWSPort = "mqtt.wsPort"

	//path of the WebSocket MQTT broker
	CfgMQTTWSPath = "mqtt.wsPath"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.String(CfgMQTTHost, "0.0.0.0", "host of the MQTT broker")
			fs.String(CfgMQTTPort, "1883", "port of the MQTT broker")
			fs.String(CfgMQTTWSPort, "1888", "port of the WebSocket MQTT broker")
			fs.String(CfgMQTTWSPath, "/ws", "path of the WebSocket MQTT broker")
			return fs
		}(),
	},
	Masked: nil,
}
