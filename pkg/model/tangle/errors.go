package tangle

import (
	"github.com/pkg/errors"
)

var (
	// ErrOperationAborted is returned when the operation was aborted e.g. by a shutdown signal.
	ErrOperationAborted = errors.New("operation was aborted")
	// ErrMessageNotFound is returned when a message was not found.
	ErrMessageNotFound = errors.New("message not found")
	// ErrNodeNotSynced is returned when the node is not synchronized.
	ErrNodeNotSynced = errors.New("node is not synchronized")
)

func NewDatabaseError(cause error) *ErrDatabaseError {
	return &ErrDatabaseError{Inner: cause}
}

type ErrDatabaseError struct {
	Inner error
}

func (e ErrDatabaseError) Cause() error {
	return e.Inner
}

func (e ErrDatabaseError) Error() string {
	return "database error: " + e.Inner.Error()
}
