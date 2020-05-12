package spammer

import (
	"errors"
	"io/ioutil"
	"math/rand"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	CPUUsageTimePerSample = time.Second / 4

	activeWorkerCount = 0

	once sync.Once
	mu   sync.Mutex

	cpu_last_sum uint64   // previous iteration
	cpu_last     []uint64 // previous iteration

	cpuUsageResult float64 // current result
	cpuUsageError  error   // nil || current error
)

func cpuUsageUpdater() {
	if runtime.GOOS == "windows" {
		mu.Lock()
		defer mu.Unlock()
		cpuUsageError = errors.New("spammer.maxCPUUsage on Windows is not supported")
		return
	}

	go func() {
		for {
			procStat, err := ioutil.ReadFile("/proc/stat")
			if err != nil { // i.e. don't throttle on Windows
				mu.Lock()
				defer mu.Unlock()
				cpuUsageError = errors.New("Can't read /proc/stat")
				return
			}

			procStatString := string(procStat)
			procStatLines := strings.Split(procStatString, "\n")
			procStatSlice := strings.Split(procStatLines[0], " ")[2:] // ["7955046" "91" "189009" "6170128" "21650" "79349" "34869" "0" "0" "0"]

			cpu_sum := uint64(0)
			cpu_now := make([]uint64, len(procStatSlice))
			for i, v := range procStatSlice {
				n, err := strconv.ParseUint(v, 10, 64)
				if err != nil {
					mu.Lock()
					defer mu.Unlock()
					cpuUsageError = errors.New("Can't convert from string to int")
					return
				}
				cpu_sum += n
				cpu_now[i] = n
			}

			if len(cpu_last) != 0 { // not on first iteration
				cpu_delta := cpu_sum - cpu_last_sum
				cpu_idle := cpu_now[3] - cpu_last[3]
				cpu_used := cpu_delta - cpu_idle

				mu.Lock()
				cpuUsageResult = float64(cpu_used) / float64(cpu_delta)
				mu.Unlock()

				// fmt.Println(cpu_now, cpu_delta, cpu_idle, cpu_used, cpuUsageResult)
			}

			cpu_last_sum = cpu_sum
			cpu_last = cpu_now

			time.Sleep(CPUUsageTimePerSample)
		} // next for...
	}()
}

func CPUUsageAdjust(nWorkerChange int) { // preemptive adjust cpu usage so we don't idle/run all workers at once
	mu.Lock()
	activeWorkerCount += nWorkerChange
	// cpuUsageResultOld := cpuUsageResult
	cpuUsageResult += float64(nWorkerChange) / float64(runtime.NumCPU()) // assume one core usage per worker
	// log.Infof("CPUUsageAdjust active workers with %d to %d. cpu %.2f -> %.2f",
	// 	nWorkerChange, activeWorkerCount,
	// 	cpuUsageResultOld, cpuUsageResult)
	mu.Unlock()
}

func CPUUsage() (float64, error) { // see: https://www.idnt.net/en-GB/kb/941772
	once.Do(cpuUsageUpdater)

	mu.Lock()
	defer mu.Unlock()
	return cpuUsageResult, cpuUsageError // XXX mu.Lock() around this?
}

func randomSleep() { // prefend gettings into a cpu eating loop
	time.Sleep(time.Duration(rand.Intn(int(2 * CPUUsageTimePerSample))))
	// time.Sleep(CPUUsageTimePerSample + time.Duration(rand.Intn(int(4 * CPUUsageTimePerSample))))
}
