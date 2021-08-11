package mqtt

import (
	"fmt"
	"net"

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
func NewBroker(bindAddress string, wsPort int, wsPath string, workerCount int, onSubscribe OnSubscribeHandler, onUnsubscribe OnUnsubscribeHandler) (*Broker, error) {

	host, port, err := net.SplitHostPort(bindAddress)
	if err != nil {
		return nil, fmt.Errorf("configure broker config error: %w", err)
	}

	c, err := broker.ConfigureConfig([]string{
		fmt.Sprintf("--worker=%d", workerCount), // worker num to process message, perfer (client num)/10.
		fmt.Sprintf("--host=%s", host),          // network host to listen on
		fmt.Sprintf("--httpport=%s", ""),        // disable http port to listen on
		fmt.Sprintf("--port=%s", port),          // port to listen on
		fmt.Sprintf("--wsport=%d", wsPort),      // port for ws to listen on
		fmt.Sprintf("--wspath=%s", wsPath),      // path for ws to listen on
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

// Config returns the broker config instance.
func (b *Broker) Config() *broker.Config {
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
