package spammer

import (
	"fmt"
	"time"
	"strings"
	"runtime"
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
)

var lastCpuUser int64
var lastCpuUserTime time.Time

func loadPerCPU() float64 {
	procStat, err := ioutil.ReadFile("/proc/stat")
	if err != nil { // i.e. don't throttle on Windows
		log.Infof("Error in reading /proc/stat, err: %s", err)
		return 0.0
	}

	procStatString := string(procStat)
	// log.Infof("procStatString %s", procStatString)
	procStatSplit := strings.Split(procStatString, " ") // ["cpu" "" "204075769" "445" . Why the extra "" ?
	// log.Infof("procStatSplit %q", procStatSplit)
	// log.Infof("procStatSplit[2] %s", procStatSplit[2])
	newCpuUser, err := strconv.Atoi(procStatSplit[2]) // TODO: sum all numbers
	if err != nil {
		log.Infof("Error in Atoi, err: %s", err)
		return 0.0
	}

	cpuUser := int64(newCpuUser) - lastCpuUser
	lastCpuUser = int64(newCpuUser)

	now := time.Now()
	duration :=  float64(now.Sub(lastCpuUserTime))
	lastCpuUserTime = now

	if lastCpuUser == 0 { // initialize
	        log.Infof("init lastCpuUser and lastCpuUserTime")
		return 0.0
	}

	accuracy := 10000 // TODO: remove this hardcording
	load := float64(cpuUser) * float64(accuracy * 1000) / float64(runtime.NumCPU()) / float64(duration)

	log.Infof("cpuUser=%d, duration=%f, load %f", cpuUser, duration, load)
	return load
}

func randomSleep() { // prefend gettings into a cpu eating loop
	minDuration := 1 * time.Second
	maxDuration := 5 * time.Second
	duration := minDuration + time.Duration(rand.Intn(int(maxDuration - minDuration)))
	// duration := 1 * time.Second
	// log.Infof("randomSleep duration %d", duration)
	time.Sleep(duration)
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
		// log.Infof("Skip doSpam because: !tangle.IsNodeSyncedWithThreshold()")
		randomSleep()
		return
	}

	if !tangle.IsNodeSynced() {
		// log.Infof("Skip doSpam because: !tangle.IsNodeSynced()")
		randomSleep()
		return
	}

	if peering.Manager().ConnectedPeerCount() == 0 {
		// log.Infof("Skip doSpam because: peering.Manager().ConnectedPeerCount() == 0")
		randomSleep()
		return
        }

	load := loadPerCPU()
	loadThreshold := 0.5
	if load >= loadThreshold {
		log.Infof("Skip doSpam because loadPerCpU >= loadThreshold (%f >= %f)", load, loadThreshold)
		randomSleep()
		return
	}
	// log.Infof("doSpam load = %f", load)

	timeStart := time.Now()
	tips, _, err := tipselection.SelectTips(depth, nil)
	if err != nil {
		log.Infof("Skip doSpam because SelectTips err: %s", err)
		randomSleep()
		return
	}
	durationGTTA := time.Since(timeStart)
	durGTTA := durationGTTA.Truncate(time.Millisecond)

	txCountValue := int(txCount.Inc())
	infoMsg := fmt.Sprintf("gTTA took %v (depth=%v)", durationGTTA.Truncate(time.Millisecond), depth)

	b, err := createBundle(address, message, tagSubstring, txCountValue, infoMsg)
	if err != nil {
		log.Infof("Skip doSpam because createBundle err: %s", err)
		randomSleep()
		return
	}

	err = doPow(b, tips[0], tips[1], mwm)
	if err != nil {
		log.Infof("Skip doSpam because doPow err: %s", err)
		randomSleep()
		return
	}

	durationPOW := time.Since(timeStart.Add(durationGTTA))
	durPOW := durationPOW.Truncate(time.Millisecond)

	for _, tx := range b {
		txTrits, _ := transaction.TransactionToTrits(&tx)
		if err := gossip.Processor().CompressAndEmit(&tx, txTrits); err != nil {
			log.Infof("Skip doSpam because CompressAndEmit err: %s", err)
			randomSleep()
			return
		}
		metrics.SharedServerMetrics.SentSpamTransactions.Inc()
	}

	durTotal := time.Since(timeStart).Truncate(time.Millisecond)
	log.Infof("Sent Spam Transaction: #%d, TxHash: %v, GTTA: %v, PoW: %v, Total: %v", txCountValue, b[0].Hash, durGTTA.Truncate(time.Millisecond), durPOW.Truncate(time.Millisecond), durTotal.Truncate(time.Millisecond))
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
