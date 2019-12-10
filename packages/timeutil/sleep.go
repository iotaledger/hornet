package timeutil

import (
	"time"
)

func Sleep(interval time.Duration, shutdownSignal <-chan struct{}) bool {
	select {
	case <-shutdownSignal:
		return false

	case <-time.After(interval):
		return true
	}
}
