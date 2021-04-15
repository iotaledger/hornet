package mqtt

import (
	"sync"

	"github.com/eclipse/paho.mqtt.golang/packets"
	"github.com/fhmq/hmq/broker/lib/topics"
)

type OnSubscribeHandler func(topic []byte)
type OnUnsubscribeHandler func(topic []byte)

// topicManager registers itself instead of the normal memtopic and implements the TopicsProvider interface.
// This allows to get notified when a topic is subscribed or unsubscribed
type topicManager struct {
	mem topics.TopicsProvider

	subscribedTopics     map[string]int
	subscribedTopicsLock sync.RWMutex

	onSubscribe   OnSubscribeHandler
	onUnsubscribe OnUnsubscribeHandler
}

func (t *topicManager) Subscribe(topic []byte, qos byte, subscriber interface{}) (byte, error) {
	t.subscribedTopicsLock.Lock()
	defer t.subscribedTopicsLock.Unlock()

	b, err := t.mem.Subscribe(topic, qos, subscriber)

	if err == nil {
		topicName := string(topic)
		count, has := t.subscribedTopics[topicName]
		if has {
			t.subscribedTopics[topicName] = count + 1
		} else {
			t.subscribedTopics[topicName] = 1
		}

		t.onSubscribe(topic)
	}

	return b, err
}

func (t *topicManager) Unsubscribe(topic []byte, subscriber interface{}) error {
	t.subscribedTopicsLock.Lock()
	defer t.subscribedTopicsLock.Unlock()

	err := t.mem.Unsubscribe(topic, subscriber)

	//Ignore error here, always unsubscribe to be safe

	topicName := string(topic)
	count, has := t.subscribedTopics[topicName]
	if has {
		if count <= 0 {
			delete(t.subscribedTopics, topicName)
		} else {
			t.subscribedTopics[topicName] = count - 1
		}
	}

	t.onUnsubscribe(topic)

	return err
}

func (t *topicManager) Subscribers(topic []byte, qos byte, subs *[]interface{}, qoss *[]byte) error {
	return t.mem.Subscribers(topic, qos, subs, qoss)
}

func (t *topicManager) Retain(msg *packets.PublishPacket) error {
	return t.mem.Retain(msg)
}

func (t *topicManager) Retained(topic []byte, msgs *[]*packets.PublishPacket) error {
	return t.mem.Retained(topic, msgs)
}

func (t *topicManager) Close() error {
	return t.mem.Close()
}

func (t *topicManager) hasSubscribers(topicName string) bool {
	t.subscribedTopicsLock.RLock()
	defer t.subscribedTopicsLock.RUnlock()

	count, has := t.subscribedTopics[topicName]
	return has && count > 0
}

func newTopicManager(onSubscribe OnSubscribeHandler, onUnsubscribe OnUnsubscribeHandler) *topicManager {

	mgr := &topicManager{
		mem:              topics.NewMemProvider(),
		subscribedTopics: make(map[string]int),
		onSubscribe:      onSubscribe,
		onUnsubscribe:    onUnsubscribe,
	}

	// The normal MQTT broker uses the `mem` topic manager internally, so first unregister the default one.
	topics.Unregister("mem")
	// Then register our custom topic manager as the new `mem` topic manager, so that is gets used automatically.
	topics.Register("mem", mgr)
	return mgr
}
