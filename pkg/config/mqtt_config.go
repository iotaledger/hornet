package config

import (
	flag "github.com/spf13/pflag"
)

const (
	// path to the MQTT broker config file
	CfgMQTTConfig = "mqtt.config"
)

func init() {
	flag.String(CfgMQTTConfig, "mqtt_config.json", "path to the MQTT broker config file")
}
