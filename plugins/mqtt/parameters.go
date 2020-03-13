package mqtt

import (
	"github.com/gohornet/hornet/packages/config"
)

func init() {
	// path to the MQTT broker config file
	config.NodeConfig.SetDefault(config.CfgMQTTConfig, "mqtt_config.json")
}
