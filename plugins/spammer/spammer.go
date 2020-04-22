package spammer

import (
	"fmt"
	"time"

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
)

var (
	_, powFunc       = pow.GetFastestProofOfWorkImpl()
	rateLimitChannel chan struct{}
	txCount          = 0
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
		return
	}

	timeStart := time.Now()
	tips, _, err := tipselection.SelectTips(depth, nil)
	if err != nil {
		return
	}
	durationGTTA := time.Since(timeStart)
	durGTTA := durationGTTA.Truncate(time.Millisecond)

	txCount++
	infoMsg := fmt.Sprintf("gTTA took %v (depth=%v)", durationGTTA.Truncate(time.Millisecond), depth)

	b, err := createBundle(address, message, tagSubstring, txCount, infoMsg)
	if err != nil {
		return
	}

	err = doPow(b, tips[0], tips[1], mwm)
	if err != nil {
		return
	}

	durationPOW := time.Since(timeStart.Add(durationGTTA))
	durPOW := durationPOW.Truncate(time.Millisecond)

	for _, tx := range b {
		txTrits, _ := transaction.TransactionToTrits(&tx)
		if err := gossip.Processor().CompressAndEmit(&tx, txTrits); err != nil {
			return
		}
		metrics.SharedServerMetrics.SentSpamTransactions.Inc()
	}

	durTotal := time.Since(timeStart).Truncate(time.Millisecond)
	log.Infof("Sent Spam Transaction: #%d, TxHash: %v, GTTA: %v, PoW: %v, Total: %v", txCount, b[0].Hash, durGTTA.Truncate(time.Millisecond), durPOW.Truncate(time.Millisecond), durTotal.Truncate(time.Millisecond))
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

		nonce, err := powFunc(trytes, mwm)
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
