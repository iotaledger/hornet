package utils

import (
	"context"

	"github.com/iotaledger/hive.go/syncutils"
)

type SyncEvent struct {
	syncutils.RWMutex

	syncMap map[interface{}]chan struct{}
}

func NewSyncEvent() *SyncEvent {
	return &SyncEvent{syncMap: make(map[interface{}]chan struct{})}
}

// RegisterEvent creates a unique channel for the key which can be used to signal global events.
func (se *SyncEvent) RegisterEvent(key interface{}) chan struct{} {

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

// DeregisterEvent removes a registered event to free the memory if not used.
func (se *SyncEvent) DeregisterEvent(key interface{}) {
	// the event is deregistered by triggering it
	se.Trigger(key)
}

func (se *SyncEvent) Trigger(key interface{}) {

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

// WaitForChannelClosed waits until the channel is closed or the context is done.
// If the context was done, the event should be manually deregistered afterwards to clean up memory.
func WaitForChannelClosed(ctx context.Context, ch chan struct{}) error {
	// we wait either until the channel got closed or the context is done
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return context.DeadlineExceeded
	}
}
