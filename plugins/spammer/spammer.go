package spammer

import (
	"fmt"
	"time"

	"github.com/iotaledger/iota.go/bundle"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/pow"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/compressed"
	"github.com/gohornet/hornet/packages/curl"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/plugins/gossip"
	"github.com/gohornet/hornet/plugins/tipselection"
)

var (
	_, powFunc       = pow.GetFastestProofOfWorkImpl()
	rateLimitChannel chan struct{}
	txCount          = 0
)

func doSpam(shutdownSignal <-chan struct{}) {

	if int64(rateLimit) != 0 {
		select {
		case <-shutdownSignal:
			return
		case <-rateLimitChannel:
		}
	}

	timeGTTA := time.Now()
	tips, _, err := tipselection.SelectTips(depth, nil)
	if err != nil {
		return
	}
	durationGTTA := time.Since(timeGTTA)

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

	for _, tx := range b {
		err = broadcastTransaction(&tx)
		if err != nil {
			return
		}
	}
}

// transactionHash makes a transaction hash from the given transaction.
func transactionHash(t *transaction.Transaction) trinary.Hash {
	trits, _ := transaction.TransactionToTrits(t)
	hashTrits := curl.CURLP81.Hash(trits)
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

func broadcastTransaction(tx *transaction.Transaction) error {

	if !transaction.HasValidNonce(tx, uint64(mwm)) {
		return consts.ErrInvalidTransactionHash
	}

	txTrits, err := transaction.TransactionToTrits(tx)
	if err != nil {
		return err
	}

	txBytesTruncated := compressed.TruncateTx(trinary.TritsToBytes(txTrits))
	hornetTx := hornet.NewTransactionFromAPI(tx, txBytesTruncated)

	gossip.Events.ReceivedTransaction.Trigger(hornetTx)
	gossip.BroadcastTransaction(make(map[string]struct{}), txBytesTruncated, trinary.MustTrytesToBytes(hornetTx.GetHash())[:49])

	return nil
}
