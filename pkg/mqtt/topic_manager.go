package mqtt

import (
	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/fhmq/hmq/broker/lib/topics"
)

type OnSubscribeHandler func(topic []byte)
type OnUnsubscribeHandler func(topic []byte)

// topicManager registers itself instead of the normal memtopic and implements the TopicsProvider interface.
// This allows to get notified when a topic is subscribed or unsubscribed
type topicManager struct {
	mem           topics.TopicsProvider
	onSubscribe   OnSubscribeHandler
	onUnsubscribe OnUnsubscribeHandler
}

func (t topicManager) Subscribe(topic []byte, qos byte, subscriber interface{}) (byte, error) {
	b, err := t.mem.Subscribe(topic, qos, subscriber)
	if err == nil {
		t.onSubscribe(topic)
	}
	return b, err
}

func (t topicManager) Unsubscribe(topic []byte, subscriber interface{}) error {
	err := t.mem.Unsubscribe(topic, subscriber)
	if err == nil {
		t.onUnsubscribe(topic)
	}
	return err
}

func (t topicManager) Subscribers(topic []byte, qos byte, subs *[]interface{}, qoss *[]byte) error {
	return t.mem.Subscribers(topic, qos, subs, qoss)
}

func (t topicManager) Retain(msg *packets.PublishPacket) error {
	return t.mem.Retain(msg)
}

func (t topicManager) Retained(topic []byte, msgs *[]*packets.PublishPacket) error {
	return t.mem.Retained(topic, msgs)
}

func (t topicManager) Close() error {
	return t.mem.Close()
}

func newTopicManager(onSubscribe OnSubscribeHandler, onUnsubscribe OnUnsubscribeHandler) *topicManager {

	mgr := &topicManager{
		mem:           topics.NewMemProvider(),
		onSubscribe:   onSubscribe,
		onUnsubscribe: onUnsubscribe,
	}

	// The normal MQTT broker uses the `mem` topic manager internally, so first unregister the default one.
	topics.Unregister("mem")
	// Then register our custom topic manager as the new `mem` topic manager, so that is gets used automatically.
	topics.Register("mem", mgr)
	return mgr
}
