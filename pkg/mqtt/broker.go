package mqtt

import (
	"fmt"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/fhmq/hmq/broker"
)

// Broker is a simple mqtt publisher abstraction.
type Broker struct {
	broker       *broker.Broker
	config       *broker.Config
	topicManager *topicManager
}

// NewBroker creates a new broker.
func NewBroker(mqttConfigFilePath string, onSubscribe OnSubscribeHandler, onUnsubscribe OnUnsubscribeHandler) (*Broker, error) {

	c, err := broker.ConfigureConfig([]string{fmt.Sprintf("--config=%s", mqttConfigFilePath)})
	if err != nil {
		return nil, fmt.Errorf("configure broker config error: %w", err)
	}

	t := newTopicManager(onSubscribe, onUnsubscribe)

	b, err := broker.NewBroker(c)
	if err != nil {
		return nil, fmt.Errorf("create new broker error: %w", err)
	}

	return &Broker{
		broker:       b,
		config:       c,
		topicManager: t,
	}, nil
}

// Start the broker.
func (b *Broker) Start() {
	b.broker.Start()
}

// GetConfig returns the broker config instance.
func (b *Broker) GetConfig() *broker.Config {
	return b.config
}

func (b *Broker) HasSubscribers(topic string) bool {
	return b.topicManager.hasSubscribers(topic)
}

// Send publishes a message.
func (b *Broker) Send(topic string, payload []byte) {

	packet := packets.NewControlPacket(packets.Publish).(*packets.PublishPacket)
	packet.TopicName = topic
	packet.Qos = 0
	packet.Payload = payload

	b.broker.PublishMessage(packet)
}
