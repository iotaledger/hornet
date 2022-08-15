package utils

import (
	"time"
)

// EstimateRemainingTime estimates the remaining time for a running operation and returns the finished percentage.
func EstimateRemainingTime(timeStart time.Time, current int64, total int64) (percentage float64, remaining time.Duration) {
	ratio := float64(current) / float64(total)
	totalTime := time.Duration(float64(time.Since(timeStart)) / ratio)
	remaining = time.Until(timeStart.Add(totalTime))

	return ratio * 100.0, remaining
}
