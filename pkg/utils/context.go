package utils

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
)

var (
	ErrMergedContextCanceled = errors.New("merged context canceled")
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

// ReturnErrIfChannelClosed returns the given error if the provided channel was closed.
func ReturnErrIfChannelClosed(channel <-chan struct{}, err error) error {
	select {
	case <-channel:
		return err
	default:
		return nil
	}
}

// mergedContext is a merged context based on two contexts.
type mergedContext struct {
	sync.RWMutex

	ctxPrimary   context.Context
	ctxSecondary context.Context

	cancelCtx context.Context

	done chan struct{}
	err  error
}

// MergeContexts creates a new mergedContext based on two contexts.
func MergeContexts(ctxPrimary context.Context, ctxSecondary context.Context) (context.Context, context.CancelFunc) {
	ctxMain, mainCancelFunc := context.WithCancel(context.Background())

	mc := &mergedContext{
		ctxPrimary:   ctxPrimary,
		ctxSecondary: ctxSecondary,
		cancelCtx:    ctxMain,
		done:         make(chan struct{}),
		err:          nil,
	}

	setCtxDoneFunc := func(err error) {
		mc.Lock()
		defer mc.Unlock()
		if mc.err != nil {
			// error already set
			return
		}
		mc.err = err

		// we can't use the Done channel from the main context,
		// otherwise Done would be closed before the error was set.
		close(mc.done)

		// cancel the main context
		mainCancelFunc()
	}

	go func() {
		select {
		case <-mc.cancelCtx.Done():
			setCtxDoneFunc(ErrMergedContextCanceled)
		case <-mc.ctxPrimary.Done():
			setCtxDoneFunc(mc.ctxPrimary.Err())
		case <-mc.ctxSecondary.Done():
			setCtxDoneFunc(mc.ctxSecondary.Err())
		}
	}()

	var mergedCancelFunc context.CancelFunc = func() {
		setCtxDoneFunc(ErrMergedContextCanceled)
	}

	return mc, mergedCancelFunc
}

// Deadline returns the minimum time of both contexts when work
// done on behalf of the contexts should be canceled.
// Deadline returns ok==false when no deadline is set.
// Successive calls to Deadline return the same results.
func (mc *mergedContext) Deadline() (time.Time, bool) {
	min := time.Time{}

	if dl, ok := mc.ctxPrimary.Deadline(); ok {
		min = dl
	}

	if dl, ok := mc.ctxSecondary.Deadline(); ok {
		// if deadline not set yet or secondary deadline is before current deadline
		if min.IsZero() || dl.Before(min) {
			min = dl
		}
	}

	return min, !min.IsZero()
}

// Done returns a channel that's closed when work done on behalf of the
// contexts should be canceled. Done may return nil if the contexts can
// never be canceled. Successive calls to Done return the same value.
// The close of the Done channel may happen asynchronously,
// after the cancel function returns.
func (mc *mergedContext) Done() <-chan struct{} {
	return mc.done
}

// Err returns nil if Done is not yet closed.
// If Done is closed, Err returns a non-nil error explaining why:
// Canceled if one the contexts was canceled
// or DeadlineExceeded if one of the contexts deadline passed.
// After Err returns a non-nil error, successive calls to Err return the same error.
func (mc *mergedContext) Err() error {
	mc.RLock()
	defer mc.RUnlock()
	return mc.err
}

// Value returns the value associated with the key in one of the two contexts,
// or nil if no value is associated with the key. Successive calls to Value with
// the same key returns the same result.
func (mc *mergedContext) Value(key interface{}) interface{} {
	if value := mc.ctxPrimary.Value(key); value != nil {
		return value
	}

	return mc.ctxSecondary.Value(key)
}
