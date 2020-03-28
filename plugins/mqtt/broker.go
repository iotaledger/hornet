package mqtt

import (
	"fmt"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/fhmq/hmq/broker"
	"github.com/gohornet/hornet/pkg/config"
)

// Simple mqtt publisher abstraction
type Broker struct {
	broker *broker.Broker
	config *broker.Config
}

// Create a new publisher.
func NewBroker() (*Broker, error) {
	mqttConfigFile := config.NodeConfig.GetString(config.CfgMQTTConfig)
	c, err := broker.ConfigureConfig([]string{fmt.Sprintf("--config=%s", mqttConfigFile)})
	if err != nil {
		log.Fatal("configure broker config error: ", err)
	}

	b, err := broker.NewBroker(c)
	if err != nil {
		log.Fatal("New Broker error: ", err)
	}

	return &Broker{
		broker: b,
		config: c,
	}, nil
}

// Start the broker
func (b *Broker) Start() error {
	b.broker.Start()
	return nil
}

// Stop the broker.
func (b *Broker) Shutdown() error {
	//return b.broker.Close()
	return nil
}

// Publish a new list of messages.
func (b *Broker) Send(topic string, message string) error {

	packet := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
	packet.TopicName = topic
	packet.Qos = 0
	packet.Payload = []byte(message)

	b.broker.PublishMessage(packet)

	return nil
}
