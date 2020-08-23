package spammer

import (
	"fmt"
	"time"

	"github.com/iotaledger/iota.go/bundle"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/batchhasher"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/utils"

	"go.uber.org/atomic"
)

// SendBundleFunc is a function which sends a bundle to the network.
type SendBundleFunc = func(b bundle.Bundle) error

// SpammerTipselFunc selects tips for the spammer.
type SpammerTipselFunc = func() (isSemiLazy bool, tips hornet.Hashes, err error)

// Spammer is used to issue transactions to the IOTA network to create load on the tangle.
type Spammer struct {

	// config options
	txAddress            string
	message              string
	tagSubstring         string
	tagSemiLazySubstring string
	tipselFunc           SpammerTipselFunc
	mwm                  int
	powHandler           *pow.Handler
	sendBundleFunc       SendBundleFunc

	seed      trinary.Trytes
	addrIndex *atomic.Uint64
}

// New creates a new spammer instance.
func New(txAddress string, message string, tag string, tagSemiLazy string, tipselFunc SpammerTipselFunc, mwm int, powHandler *pow.Handler, sendBundleFunc SendBundleFunc) *Spammer {

	tagSubstring := trinary.MustPad(tag, consts.TagTrinarySize/3)[:consts.TagTrinarySize/3]
	tagSemiLazySubstring := tagSubstring
	if tagSemiLazy != "" {
		tagSemiLazySubstring = trinary.MustPad(tagSemiLazy, consts.TagTrinarySize/3)[:consts.TagTrinarySize/3]
	}

	if len(tagSubstring) > 20 {
		tagSubstring = string([]rune(tagSubstring)[:20])
	}
	if len(tagSemiLazySubstring) > 20 {
		tagSemiLazySubstring = string([]rune(tagSemiLazySubstring)[:20])
	}

	return &Spammer{
		txAddress:            trinary.MustPad(txAddress, consts.AddressTrinarySize/3)[:consts.AddressTrinarySize/3],
		message:              message,
		tagSubstring:         tagSubstring,
		tagSemiLazySubstring: tagSemiLazySubstring,
		tipselFunc:           tipselFunc,
		mwm:                  mwm,
		powHandler:           powHandler,
		sendBundleFunc:       sendBundleFunc,
		seed:                 utils.RandomTrytesInsecure(81),
		addrIndex:            atomic.NewUint64(0),
	}
}

func (s *Spammer) DoSpam(bundleSize int, valueSpam bool, shutdownSignal <-chan struct{}) (time.Duration, time.Duration, error) {

	tag := s.tagSubstring

	timeStart := time.Now()
	isSemiLazy, tips, err := s.tipselFunc()
	if err != nil {
		return time.Duration(0), time.Duration(0), err
	}
	durationGTTA := time.Since(timeStart)

	if isSemiLazy {
		tag = s.tagSemiLazySubstring
	}

	infoMsg := fmt.Sprintf("gTTA took %v", durationGTTA.Truncate(time.Millisecond))

	var seedIndex uint64
	if valueSpam {
		seedIndex = s.addrIndex.Inc()
	}

	txCountValue := int(metrics.SharedServerMetrics.SentSpamTransactions.Load()) + bundleSize
	b, err := createBundle(s.seed, seedIndex, s.txAddress, s.message, tag, bundleSize, valueSpam, txCountValue, infoMsg)
	if err != nil {
		return time.Duration(0), time.Duration(0), err
	}

	timeStart = time.Now()
	err = s.doPow(b, tips[0].Trytes(), tips[1].Trytes(), s.mwm, shutdownSignal)
	if err != nil {
		return time.Duration(0), time.Duration(0), err
	}
	durationPOW := time.Since(timeStart)

	if err := s.sendBundleFunc(b); err != nil {
		return time.Duration(0), time.Duration(0), err
	}

	return durationGTTA, durationPOW, nil
}

func (s *Spammer) doPow(b bundle.Bundle, trunk trinary.Hash, branch trinary.Hash, mwm int, shutdownSignal <-chan struct{}) error {
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

		nonce, err := s.powHandler.DoPoW(trytes, mwm, 1)
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

// transactionHash makes a transaction hash from the given transaction.
func transactionHash(t *transaction.Transaction) trinary.Hash {
	trits, _ := transaction.TransactionToTrits(t)
	hashTrits := batchhasher.CURLP81.Hash(trits)
	return trinary.MustTritsToTrytes(hashTrits)
}
