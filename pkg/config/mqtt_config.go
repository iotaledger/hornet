package config

const (
	// path to the MQTT broker config file
	CfgMQTTConfig = "mqtt.config"
)

func init() {
	configFlagSet.String(CfgMQTTConfig, "mqtt_config.json", "path to the MQTT broker config file")
}
