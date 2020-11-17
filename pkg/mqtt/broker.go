package mqtt

import (
	"fmt"
	"net"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/fhmq/hmq/broker"
)

const (
	workerNumber = 4096
)

// Broker is a simple mqtt publisher abstraction.
type Broker struct {
	broker       *broker.Broker
	config       *broker.Config
	topicManager *topicManager
}

// NewBroker creates a new broker.
func NewBroker(bindAddress string, wsPort int, wsPath string, onSubscribe OnSubscribeHandler, onUnsubscribe OnUnsubscribeHandler) (*Broker, error) {

	host, port, err := net.SplitHostPort(bindAddress)
	if err != nil {
		return nil, fmt.Errorf("configure broker config error: %w", err)
	}

	c, err := broker.ConfigureConfig([]string{
		fmt.Sprintf("--worker=%d", workerNumber),
		fmt.Sprintf("--host=%s", host),
		fmt.Sprintf("--port=%s", port),
		fmt.Sprintf("--wsport=%d", wsPort),
		fmt.Sprintf("--wspath=%s", wsPath),
	})

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
