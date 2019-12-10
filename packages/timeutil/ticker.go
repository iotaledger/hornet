package timeutil

import (
	"time"
)

func Ticker(handler func(), interval time.Duration, shutdownSignal <-chan struct{}) {
	ticker := time.NewTicker(interval)
ticker:
	for {
		select {
		case <-shutdownSignal:
			break ticker
		case <-ticker.C:
			handler()
		}
	}
}
