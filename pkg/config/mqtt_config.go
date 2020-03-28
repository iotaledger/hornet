package config

const (
	// path to the MQTT broker config file
	CfgMQTTConfig = "mqtt.config"
)

func init() {
	NodeConfig.SetDefault(CfgMQTTConfig, "mqtt_config.json")
}
