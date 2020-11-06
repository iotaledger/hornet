package mqtt

import (
	"github.com/gohornet/hornet/pkg/node"
	flag "github.com/spf13/pflag"
)

const (
	// path to the MQTT broker config file
	CfgMQTTConfig = "mqtt.config"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.String(CfgMQTTConfig, "mqtt_config.json", "path to the MQTT broker config file")
			return fs
		}(),
	},
	Hide: nil,
}
