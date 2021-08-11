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

// CriticalError wraps the given error as a critical error.
func CriticalError(err error) error {
	return &criticalError{err: err}
}

// IsCriticalError unwraps the inner error held by the critical error if the given error is a critical error.
// If the given error is not a critical error, nil is returned.
func IsCriticalError(err error) error {
	var critErr *criticalError
	if errors.As(err, &critErr) {
		return critErr.Unwrap()
	}
	return nil
}

// SoftError wraps the given error as a soft error.
func SoftError(err error) error {
	return &softError{err: err}
}

// IsSoftError unwraps the inner error held by the soft error if the given error is a soft error.
// If the given error is not a soft error, nil is returned.
func IsSoftError(err error) error {
	var softErr *softError
	if errors.As(err, &softErr) {
		return softErr.Unwrap()
	}
	return nil
}

// criticalError is an error which is critical, meaning that the node must halt operation.
type criticalError struct {
	err error
}

func (ce criticalError) Error() string { return ce.err.Error() }
func (ce criticalError) Unwrap() error { return ce.err }

// softError is an error which is soft, meaning that the node should probably log it but continue operation.
type softError struct {
	err error
}

func (se softError) Error() string { return se.err.Error() }
func (se softError) Unwrap() error { return se.err }
