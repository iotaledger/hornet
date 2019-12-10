package zeromq

import (
	"sort"
	"sync"

	zmq "github.com/go-zeromq/zmq4"
	"github.com/iotaledger/iota.go/address"
)

// Topic names
const (
	topicLMI      = "lmi"
	topicLMSI     = "lmsi"
	topicLMHS     = "lmhs"
	topicSN       = "sn"
	topicTxTrytes = "tx_trytes"
	topicTX       = "tx"
)

var (
	// RegisteredZMQTopics lists the available topics
	RegisteredZMQTopics = []string{
		topicLMI,
		topicLMSI,
		topicLMHS,
		topicSN,
		topicTxTrytes,
		topicTX,
	}

	addressTopics AddressTopics
)

// SpecialTopics struct
type SpecialTopics struct {
	Topics []string
}

// AddressTopics stuct
type AddressTopics struct {
	mu         sync.Mutex
	Addressses []string
}

// GetSpecialTopics is a sortet list of special topics (e.g. Addresses)
func GetSpecialTopics() *SpecialTopics {
	topics := publisher.socket.(zmq.Topics).Topics()
	specialTopics := &SpecialTopics{}
	for _, t := range topics {
		found := false
		for _, rt := range RegisteredZMQTopics {
			if t == rt {
				found = true
				break
			}
		}
		// Topic not found in RegisteredZMQTopics. Add it to return slice
		if !found {
			specialTopics.Topics = append(specialTopics.Topics, t)
		}
	}

	sort.Strings(specialTopics.Topics)
	return specialTopics
}

// AddressTopics filters SpecialTopics for address topics
func (st *SpecialTopics) AddressTopics() {
	addrTopic := []string{}
	for _, topic := range st.Topics {
		err := address.ValidAddress(topic)
		if err == nil {
			if len(topic) == 90 {
				addrTopic = append(addrTopic, topic[:81])
			} else if len(topic) == 81 {
				addrTopic = append(addrTopic, topic)
			}
		}
	}
	addressTopics.mu.Lock()
	addressTopics.Addressses = addrTopic
	addressTopics.mu.Unlock()
}

// GetAddressTopics returns subscribed addresses
func GetAddressTopics() []string {
	addressTopics.mu.Lock()
	defer addressTopics.mu.Unlock()
	return addressTopics.Addressses
}

func updateAddressTopics() {
	GetSpecialTopics().AddressTopics()
}
