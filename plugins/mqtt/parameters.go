package mqtt

import (
	"github.com/gohornet/hornet/packages/parameter"
)

func init() {

	// "Path to the MQTT broker config file"
	parameter.NodeConfig.SetDefault("mqtt.config", "mqtt_config.json")
}
