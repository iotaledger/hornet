package utils

import (
	"context"
)

// ReturnErrIfCtxDone returns the given error if the provided context is done.
func ReturnErrIfCtxDone(ctx context.Context, err error) error {
	select {
	case <-ctx.Done():
		return err
	default:
		return nil
	}
}
