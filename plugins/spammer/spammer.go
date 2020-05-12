package spammer

import (
	"fmt"
	"time"
	"sync"
	"runtime"
	"strings"
	"errors"
	"strconv"
	"io/ioutil"
	"math/rand"

	"github.com/iotaledger/iota.go/bundle"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/pow"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/batchhasher"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/plugins/gossip"
	"github.com/gohornet/hornet/plugins/tipselection"
    "github.com/gohornet/hornet/plugins/peering"

	"go.uber.org/atomic"
)

var (
	_, powFunc       = pow.GetFastestProofOfWorkUnsyncImpl()
	rateLimitChannel chan struct{}
	txCount          = atomic.NewInt32(0)
	activeWorkerCount = 0

	// cpu usage...
	CPUUsageTimePerSample = time.Second / 4
	once sync.Once
	mu sync.Mutex

	cpu_last_sum uint64		// previous iteration
	cpu_last []uint64		// previous iteration

	cpuUsageResult float64 	// current result
	cpuUsageError error 	// nil || current error
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
			for i, v := range(procStatSlice) {
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

func doSpam(shutdownSignal <-chan struct{}) {

	if rateLimit != 0 {
		select {
		case <-shutdownSignal:
			return
		case <-rateLimitChannel:
		}
	}

	if !tangle.IsNodeSyncedWithThreshold() {
		// log.Infof("worker idle because: !tangle.IsNodeSyncedWithThreshold()")
		randomSleep()
		return
	}

	if !tangle.IsNodeSynced() {
		// log.Infof("worker idle because: !tangle.IsNodeSynced()")
		randomSleep()
		return
	}

	if peering.Manager().ConnectedPeerCount() == 0 {
		// log.Infof("worker idle because: peering.Manager().ConnectedPeerCount() == 0")
		randomSleep()
		return
    }

	if maxCPUUsage > 0.0 {
		cpuUsage, err := CPUUsage()
		if (err == nil) {
			// log.Infof("cpuUsage %.2f\n", cpuUsage)
			if cpuUsage > maxCPUUsage {
				// log.Infof("worker idle with cpuUsage %.2f > %.2f", cpuUsage, maxCPUUsage)
				randomSleep()
				return
			}
		} else { // else cpu usage detection not supported (Windows?)
			log.Infof("Error in CPUUsage. %s", err)
		}
	}

	CPUUsageAdjust(+1)
	defer CPUUsageAdjust(-1)

	timeStart := time.Now()
	tips, _, err := tipselection.SelectTips(depth, nil)
	if err != nil {
		log.Infof("worker idle because SelectTips err: %s", err)
		randomSleep()
		return
	}
	durationGTTA := time.Since(timeStart)
	durGTTA := durationGTTA.Truncate(time.Millisecond)

	txCountValue := int(txCount.Inc())
	infoMsg := fmt.Sprintf("gTTA took %v (depth=%v)", durationGTTA.Truncate(time.Millisecond), depth)

	b, err := createBundle(address, message, tagSubstring, txCountValue, infoMsg)
	if err != nil {
		log.Infof("worker idle because createBundle err: %s", err)
		randomSleep()
		return
	}

	err = doPow(b, tips[0], tips[1], mwm)
	if err != nil {
		log.Infof("worker idle because doPow err: %s", err)
		randomSleep()
		return
	}

	durationPOW := time.Since(timeStart.Add(durationGTTA))
	durPOW := durationPOW.Truncate(time.Millisecond)

	for _, tx := range b {
		txTrits, _ := transaction.TransactionToTrits(&tx)
		if err := gossip.Processor().CompressAndEmit(&tx, txTrits); err != nil {
			log.Infof("worker idle because CompressAndEmit err: %s", err)
			randomSleep()
			return
		}
		metrics.SharedServerMetrics.SentSpamTransactions.Inc()
	}

	durTotal := time.Since(timeStart).Truncate(time.Millisecond)
	log.Infof("sent transaction: #%d, TxHash: %v, GTTA: %v, PoW: %v, Total: %v", txCountValue, b[0].Hash, durGTTA.Truncate(time.Millisecond), durPOW.Truncate(time.Millisecond), durTotal.Truncate(time.Millisecond))
}

// transactionHash makes a transaction hash from the given transaction.
func transactionHash(t *transaction.Transaction) trinary.Hash {
	trits, _ := transaction.TransactionToTrits(t)
	hashTrits := batchhasher.CURLP81.Hash(trits)
	return trinary.MustTritsToTrytes(hashTrits)
}

func doPow(b bundle.Bundle, trunk trinary.Hash, branch trinary.Hash, mwm int) error {
	var prev trinary.Hash

	for i := len(b) - 1; i >= 0; i-- {
		switch {
		case i == len(b)-1:
			// Last tx in the bundle
			b[i].TrunkTransaction = trunk
			b[i].BranchTransaction = branch
		default:
			b[i].TrunkTransaction = prev
			b[i].BranchTransaction = trunk
		}

		b[i].AttachmentTimestamp = time.Now().UnixNano() / int64(time.Millisecond)
		b[i].AttachmentTimestampLowerBound = consts.LowerBoundAttachmentTimestamp
		b[i].AttachmentTimestampUpperBound = consts.UpperBoundAttachmentTimestamp

		trytes, err := transaction.TransactionToTrytes(&b[i])
		if err != nil {
			return err
		}

		nonce, err := powFunc(trytes, mwm, 1)
		if err != nil {
			return err
		}

		b[i].Nonce = nonce

		// set new transaction hash
		b[i].Hash = transactionHash(&b[i])
		prev = b[i].Hash
	}
	return nil
}
