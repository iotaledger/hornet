package spammer

import (
	"bytes"
	"fmt"
	"time"

	"github.com/iotaledger/iota.go/bundle"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/batchhasher"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/gohornet/hornet/plugins/gossip"
	"github.com/gohornet/hornet/plugins/peering"
	"github.com/gohornet/hornet/plugins/pow"
	"github.com/gohornet/hornet/plugins/urts"

	"go.uber.org/atomic"
)

var (
	rateLimitChannel chan struct{}
	txCount          = atomic.NewInt32(0)
	seed             = utils.RandomTrytesInsecure(81)
	addrIndex        = atomic.NewInt32(0)
)

func doSpam(shutdownSignal <-chan struct{}) {

	if rateLimit != 0 {
		select {
		case <-shutdownSignal:
			return
		case <-rateLimitChannel:
		}
	}

	if !tangle.IsNodeSyncedWithThreshold() {
		time.Sleep(time.Second)
		return
	}

	if checkPeersConnected && peering.Manager().ConnectedPeerCount() == 0 {
		time.Sleep(time.Second)
		return
	}

	if err := waitForLowerCPUUsage(shutdownSignal); err != nil {
		if err != tangle.ErrOperationAborted {
			log.Warn(err.Error())
		}
		return
	}

	if spammerStartTime.IsZero() {
		// Set the start time for the metrics
		spammerStartTime = time.Now()
	}

	timeStart := time.Now()

	tipselFunc := urts.TipSelector.SelectNonLazyTips
	tag := tagSubstring

	reduceSemiLazyTips := semiLazyTipsLimit != 0 && metrics.SharedServerMetrics.TipsSemiLazy.Load() > semiLazyTipsLimit
	if reduceSemiLazyTips {
		tipselFunc = urts.TipSelector.SelectSemiLazyTips
		tag = tagSemiLazySubstring
	}

	tips, err := tipselFunc()
	if err != nil {
		return
	}

	if reduceSemiLazyTips && bytes.Equal(tips[0], tips[1]) {
		// do not spam if the tip is equal since that would not reduce the semi lazy count
		return
	}

	durationGTTA := time.Since(timeStart)

	txCountValue := int(txCount.Add(int32(bundleSize)))
	infoMsg := fmt.Sprintf("gTTA took %v", durationGTTA.Truncate(time.Millisecond))

	b, err := createBundle(txAddress, message, tag, bundleSize, valueSpam, txCountValue, infoMsg)
	if err != nil {
		return
	}

	err = doPow(b, tips[0].Trytes(), tips[1].Trytes(), mwm, shutdownSignal)
	if err != nil {
		return
	}

	durationPOW := time.Since(timeStart.Add(durationGTTA))

	for _, t := range b {
		tx := t // assign to new variable, otherwise it would be overwritten by the loop before processed
		txTrits, _ := transaction.TransactionToTrits(&tx)
		if err := gossip.Processor().CompressAndEmit(&tx, txTrits); err != nil {
			return
		}
		metrics.SharedServerMetrics.SentSpamTransactions.Inc()
	}

	Events.SpamPerformed.Trigger(&SpamStats{GTTA: float32(durationGTTA.Seconds()), POW: float32(durationPOW.Seconds())})
}

// transactionHash makes a transaction hash from the given transaction.
func transactionHash(t *transaction.Transaction) trinary.Hash {
	trits, _ := transaction.TransactionToTrits(t)
	hashTrits := batchhasher.CURLP81.Hash(trits)
	return trinary.MustTritsToTrytes(hashTrits)
}

func doPow(b bundle.Bundle, trunk trinary.Hash, branch trinary.Hash, mwm int, shutdownSignal <-chan struct{}) error {
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

		select {
		case <-shutdownSignal:
			return tangle.ErrOperationAborted
		default:
		}

		nonce, err := pow.Handler().DoPoW(trytes, mwm, 1)
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
