package spammer

import (
	"context"
	"math/rand"
	"runtime"
	"time"

	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/cpu"

	"github.com/gohornet/hornet/pkg/common"
)

const (
	cpuUsageSampleTime = 200 * time.Millisecond
	cpuUsageSleepTime  = int(200 * time.Millisecond)
)

var (
	// ErrCPUPercentageUnknown is returned if the CPU usage couldn't be determined.
	ErrCPUPercentageUnknown = errors.New("CPU percentage unknown")
)

// cpuUsageUpdater starts a goroutine that computes cpu usage each cpuUsageSampleTime.
func cpuUsageUpdater() {
	go func() {
		for {
			if Plugin.Daemon().IsStopped() {
				return
			}

			cpuUsagePSutil, err := cpu.Percent(cpuUsageSampleTime, false)
			cpuUsageLock.Lock()
			if err != nil {
				cpuUsageError = ErrCPUPercentageUnknown
				cpuUsageLock.Unlock()
				return
			}
			cpuUsageError = nil
			cpuUsageResult = cpuUsagePSutil[0] / 100.0
			cpuUsageLock.Unlock()
		}
	}()
}

// cpuUsage returns latest cpu usage
func cpuUsage() (float64, error) {
	cpuUsageLock.RLock()
	defer cpuUsageLock.RUnlock()

	return cpuUsageResult, cpuUsageError
}

// cpuUsageGuessWithAdditionalWorker returns guessed cpu usage with another core running at 100% load
func cpuUsageGuessWithAdditionalWorker() (float64, error) {
	cpuUsage, err := cpuUsage()
	if err != nil {
		return 0.0, err
	}

	return cpuUsage + (1.0 / float64(runtime.NumCPU())), nil
}

// waitForLowerCPUUsage waits until the cpu usage drops below cpuMaxUsage.
func waitForLowerCPUUsage(ctx context.Context, cpuMaxUsage float64) error {
	if cpuMaxUsage == 0.0 {
		return nil
	}

	for {
		cpuUsage, err := cpuUsageGuessWithAdditionalWorker()
		if err != nil {
			return err
		}

		if cpuUsage < cpuMaxUsage {
			break
		}

		select {
		case <-ctx.Done():
			return common.ErrOperationAborted
		case <-time.After(time.Duration(cpuUsageSleepTime + rand.Intn(cpuUsageSleepTime))):
			// sleep a random time between cpuUsageSleepTime and 2*cpuUsageSleepTime
		}
	}

	return nil
}
