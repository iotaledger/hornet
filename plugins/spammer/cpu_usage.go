package spammer

import (
	"errors"
	"io/ioutil"
	"math/rand"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/iotaledger/hive.go/syncutils"
)

var (
	// CPUUsageTimePerSample for setting the polling frequency
	CPUUsageTimePerSample = time.Second / 4

	once              syncutils.Once
	cpuUsageErrorLock syncutils.Mutex

	cpuLastSum uint64   // previous iteration
	cpuLast    []uint64 // previous iteration

	// result and error get updated frequently based on CPUUsageTimePerSample
	cpuUsageResult float64
	cpuUsageError  error
)

// cpuUsageUpdater starts a goroutine that computes cpu usage each CPUUsageTimePerSample
func cpuUsageUpdater() {
	go func() {
		for {
			// based on: https://www.idnt.net/en-GB/kb/941772

			procStat, err := ioutil.ReadFile("/proc/stat")
			if err != nil { // i.e. don't throttle on Windows
				cpuUsageErrorLock.Lock()
				defer cpuUsageErrorLock.Unlock()
				cpuUsageError = errors.New("Can't read /proc/stat")
				return
			}

			procStatString := string(procStat)
			procStatLines := strings.Split(procStatString, "\n")
			procStatSlice := strings.Split(procStatLines[0], " ")[2:] // ["7955046" "91" "189009" "6170128" "21650" "79349" "34869" "0" "0" "0"]

			cpuSum := uint64(0)
			cpuNow := make([]uint64, len(procStatSlice))
			for i, v := range procStatSlice {
				n, err := strconv.ParseUint(v, 10, 64)
				if err != nil {
					cpuUsageErrorLock.Lock()
					defer cpuUsageErrorLock.Unlock()
					cpuUsageError = errors.New("Can't convert from string to int")
					return
				}
				cpuSum += n
				cpuNow[i] = n
			}

			if len(cpuLast) != 0 { // not on first iteration
				cpuDelta := cpuSum - cpuLastSum
				cpuIdle := cpuNow[3] - cpuLast[3]
				cpuUsed := cpuDelta - cpuIdle

				cpuUsageErrorLock.Lock()
				cpuUsageResult = float64(cpuUsed) / float64(cpuDelta)
				cpuUsageErrorLock.Unlock()

				// fmt.Println(cpuNow, cpuDelta, cpuIdle, cpuUsed, cpuUsageResult)
			}

			cpuLastSum = cpuSum
			cpuLast = cpuNow

			time.Sleep(CPUUsageTimePerSample)
		} // next for...
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
