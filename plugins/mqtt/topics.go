package mqtt

// Topic names
const (
	topicLMI          = "lmi"
	topicLMSI         = "lmsi"
	topicLMHS         = "lmhs"
	topicLM           = "lm"
	topicLSM          = "lsm"
	topicSN           = "sn"
	topicConfTrytes   = "conf_trytes"
	topicTxTrytes     = "trytes"
	topicTX           = "tx"
	topicSpentAddress = "spent_address"
	//topicPrefixAddress = "addr/"
)

/*
var (
	addressTopics AddressTopics
)

// AddressTopics stuct
type AddressTopics struct {
	mu         sync.Mutex
	Addressses []string
}

// GetAddressTopics returns subscribed addresses
func GetAddressTopics() []string {
	addressTopics.mu.Lock()
	defer addressTopics.mu.Unlock()
	return addressTopics.Addressses
}

// updateAddressTopics filters Subscribers for address topics
func updateAddressTopics() {

	addrTopic := []string{}
	if mqttBroker != nil {

		var subs []interface{}
		var qoss []byte

		err := mqttBroker.Subscribers([]byte(topicPrefixAddress), packet.Qos, &subs, &qoss)
		if err != nil {
			log.Errorf("search sub client error, %v", err)
		}

		for _, sub := range subs {
			if s, ok := sub.(*subscription); ok {
				err := address.ValidAddress(s)
				if err == nil {
					if len(s) == 90 {
						addrTopic = append(addrTopic, s[:81])
					} else if len(s) == 81 {
						addrTopic = append(addrTopic, s)
					}
				}
			}
		}
	}

	sort.Strings(addrTopic)

	addressTopics.mu.Lock()
	addressTopics.Addressses = addrTopic
	addressTopics.mu.Unlock()
}
*/
