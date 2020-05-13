package spammer

import (
	"errors"
	"math/rand"
	"runtime"
	"time"

	"github.com/iotaledger/hive.go/syncutils"

	"github.com/shirou/gopsutil/cpu"
)

var (
	// CPUUsageTimePerSample for setting the polling frequency
	CPUUsageTimePerSample = time.Second / 4

	once              syncutils.Once
	cpuUsageErrorLock syncutils.Mutex

	// result and error get updated frequently based on CPUUsageTimePerSample
	cpuUsageResult float64
	cpuUsageError  error
)

// cpuUsageUpdater starts a goroutine that computes cpu usage each CPUUsageTimePerSample
func cpuUsageUpdater() {
	go func() {
		for {
			cpuUsagePSutil, err := cpu.Percent(CPUUsageTimePerSample, false) // percpu=false
			if err != nil {
				cpuUsageErrorLock.Lock()
				defer cpuUsageErrorLock.Unlock()
				cpuUsageError = errors.New("CPU percentage unknown")
				return
			}

			cpuUsageErrorLock.Lock()
			cpuUsageResult = cpuUsagePSutil[0] / 100.0
			cpuUsageErrorLock.Unlock()
		}
	}()
}

// CPUUsageAdjust guesstimates new cpu usage after start/stop of a worker so we don't idle/run all workers at once
func CPUUsageAdjust(nWorkerChange int) {
	cpuUsageErrorLock.Lock()
	defer cpuUsageErrorLock.Unlock()

	cpuUsageResult += float64(nWorkerChange) / float64(runtime.NumCPU()) // assume one core usage per worker
}

// CPUUsage starts background polling once and returns latest cpu usage and error
func CPUUsage() (float64, error) {
	once.Do(cpuUsageUpdater)

	cpuUsageErrorLock.Lock()
	defer cpuUsageErrorLock.Unlock()

	return cpuUsageResult, cpuUsageError
}

// randomSleep prefends workers from becoming active very often (and eating a lot of cpu cycles) when we don't use the rateLimit ticker
func randomSleep() {
	if rateLimit != 0 {
		return
	}
	time.Sleep(time.Duration(rand.Intn(int(2 * CPUUsageTimePerSample))))
	// time.Sleep(CPUUsageTimePerSample + time.Duration(rand.Intn(int(4 * CPUUsageTimePerSample))))
}
