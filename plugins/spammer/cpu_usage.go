package spammer

import (
	"errors"
	"math/rand"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/cpu"

	"github.com/iotaledger/hive.go/syncutils"
)

var (
	cpuUsageSampleTime = 200 * time.Millisecond
	cpuUsageSleepTime  = int(200 * time.Millisecond)

	cpuUsageLock   syncutils.RWMutex
	cpuUsageResult float64
	cpuUsageError  error

	ErrCPUPercentageUnknown = errors.New("CPU percentage unknown")
)

// cpuUsageUpdater starts a goroutine that computes cpu usage each cpuUsageSampleTime.
func cpuUsageUpdater() {
	go func() {
		for {
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
func waitForLowerCPUUsage() error {
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

		// sleep a random time between cpuUsageSleepTime and 2*cpuUsageSleepTime
		time.Sleep(time.Duration(cpuUsageSleepTime + rand.Intn(cpuUsageSleepTime)))
	}

	return nil
}
