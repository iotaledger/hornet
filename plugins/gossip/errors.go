package gossip

import (
	"fmt"
	"github.com/pkg/errors"
)

var (
	ErrInvalidSendParam       = errors.New("invalid parameter passed to send")
)

func NewConnectionFailureError(cause error) *ConnectionFailureError {
	return &ConnectionFailureError{Inner: cause}
}

type ConnectionFailureError struct {
	Inner error
}

func (e ConnectionFailureError) Error() string {
	return fmt.Sprintf("could not connect to neighbor: %s", e.Inner.Error())
}

func (e ConnectionFailureError) Cause() error {
	return e.Inner
}

func NewHandshakeError(cause error) *HandshakeError {
	return &HandshakeError{Inner: cause}
}

type HandshakeError struct {
	Inner error
}

func (e HandshakeError) Error() string {
	return fmt.Sprintf("couldn't handshake properly: %s", e.Inner.Error())
}

func (e HandshakeError) Cause() error {
	return e.Inner
}

func NewSendError(cause error) *SendError {
	return &SendError{Inner: cause}
}

type SendError struct {
	Inner error
}

func (e SendError) Error() string {
	return fmt.Sprintf("couldn't send: %s", e.Inner.Error())
}

func (e SendError) Cause() error {
	return e.Inner
}
