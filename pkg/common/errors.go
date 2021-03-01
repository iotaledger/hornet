package common

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
	// ErrNodeLoadTooHigh is returned when the load on the node is too high.
	ErrNodeLoadTooHigh = errors.New("node load is too high")
)

// CriticalError is an error which is critical, meaning that migration components no longer can run.
type CriticalError struct {
	Err error
}

func (ce CriticalError) Error() string { return ce.Err.Error() }
func (ce CriticalError) Unwrap() error { return ce.Err }

// SoftError is an error which is soft, meaning that migration components can still run.
type SoftError struct {
	Err error
}

func (se SoftError) Error() string { return se.Err.Error() }
func (se SoftError) Unwrap() error { return se.Err }
