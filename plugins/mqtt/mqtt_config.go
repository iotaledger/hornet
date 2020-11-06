package mqtt

import (
	"github.com/gohornet/hornet/core/cli"
)

const (
	// path to the MQTT broker config file
	CfgMQTTConfig = "mqtt.config"
)

func init() {
	cli.ConfigFlagSet.String(CfgMQTTConfig, "mqtt_config.json", "path to the MQTT broker config file")
}
