package zeromq

import (
	"context"
	"fmt"
	"strings"

	zmq "github.com/go-zeromq/zmq4"

	"github.com/iotaledger/hive.go/parameter"
)

// Publisher is a simple zmq publisher abstraction
type Publisher struct {
	socket zmq.Socket
}

// NewPublisher creates a new publisher.
func NewPublisher() (*Publisher, error) {

	//socket := zmq.NewPub(context.Background())
	socket := zmq.NewPub(context.Background())
	return &Publisher{
		socket: socket,
	}, nil
}

// Start the publisher on the given port.
func (pub *Publisher) Start() error {
	return pub.socket.Listen(fmt.Sprintf("%s://%s:%d", parameter.NodeConfig.GetString("zmq.protocol"), parameter.NodeConfig.GetString("zmq.host"), parameter.NodeConfig.GetInt("zmq.port")))
}

// Shutdown stops the publisher.
func (pub *Publisher) Shutdown() error {
	return pub.socket.Close()
}

// Send sends a new list of messages.
func (pub *Publisher) Send(topic string, messages []string) error {
	if len(messages) == 0 || len(messages[0]) == 0 {
		log.Error("Publisher: Invalid messages")
	}
	if topic == "" {
		log.Error("Publisher: No topic provided")
	}

	data := strings.Join(messages, " ")
	msg := zmq.NewMsgString(topic + " " + data)

	err := pub.socket.Send(msg)
	if err != nil {
		return err
	}
	return nil
}
