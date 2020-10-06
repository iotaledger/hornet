package utils

import (
	"context"
	"time"

	"github.com/iotaledger/hive.go/syncutils"
)

type syncEvent struct {
	syncutils.RWMutex

	syncMap map[interface{}]chan struct{}
}

func NewSyncEvent() *syncEvent {
	return &syncEvent{syncMap: make(map[interface{}]chan struct{})}
}

// RegisterEvent creates a unique channel for the key which can be used to signal global events.
func (se *syncEvent) RegisterEvent(key interface{}) chan struct{} {

	se.RLock()
	// check if the channel already exists
	if ch, exists := se.syncMap[key]; exists {
		se.RUnlock()
		return ch
	}
	se.RUnlock()

	se.Lock()
	defer se.Unlock()

	// check again if it was created in the meantime
	if ch, exists := se.syncMap[key]; exists {
		return ch
	}

	msgProcessedChan := make(chan struct{})
	se.syncMap[key] = msgProcessedChan

	return msgProcessedChan
}

func (se *syncEvent) Trigger(key interface{}) {

	se.RLock()
	// check if the key was registered
	if _, exists := se.syncMap[key]; !exists {
		se.RUnlock()
		return
	}
	se.RUnlock()

	se.Lock()
	defer se.Unlock()

	// check again if the key is still registered
	ch, exists := se.syncMap[key]
	if !exists {
		return
	}

	// trigger the event by closing the channel
	close(ch)

	delete(se.syncMap, key)
}

// WaitForChannelClosed waits until the channel is closed or the deadline is reached.
func WaitForChannelClosed(ch chan struct{}, timeout ...time.Duration) error {

	if len(timeout) > 0 {
		// wait for at most "timeout" for the channel to be closed
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(timeout[0]))
		defer cancel()

		// we wait either until the channel got closed or we reached the deadline
		select {
		case <-ch:
			return nil
		case <-ctx.Done():
			return context.DeadlineExceeded
		}
	}

	// wait until channel is closed
	<-ch
	return nil
}
