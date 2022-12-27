package mqtt

import (
	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hornet/pkg/node"
)

const (
	// the bind address on which the MQTT broker listens on.
	CfgMQTTBindAddress = "mqtt.bindAddress"
	// the port of the WebSocket MQTT broker.
	CfgMQTTWSPort = "mqtt.wsPort"
	// the number of parallel workers the MQTT broker uses to publish messages.
	CfgMQTTWorkerCount = "mqtt.workerCount"
	// the number of deleted topics that trigger a garbage collection of the topic manager.
	CfgMQTTTopicCleanupThreshold = "mqtt.topicCleanupThreshold"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.String(CfgMQTTBindAddress, "localhost:1883", "bind address on which the MQTT broker listens on")
			fs.Int(CfgMQTTWSPort, 1888, "port of the WebSocket MQTT broker")
			fs.Int(CfgMQTTWorkerCount, 100, "number of parallel workers the MQTT broker uses to publish messages")
			fs.Int(CfgMQTTTopicCleanupThreshold, 10000, "number of deleted topics that trigger a garbage collection of the topic manager")
			return fs
		}(),
	},
	Masked: nil,
}
