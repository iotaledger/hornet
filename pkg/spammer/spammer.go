package spammer

import (
	"fmt"
	"time"

	iotago "github.com/iotaledger/iota.go"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/pow"
)

// SendMessageFunc is a function which sends a message to the network.
type SendMessageFunc = func(msg *tangle.Message) error

// SpammerTipselFunc selects tips for the spammer.
type SpammerTipselFunc = func() (isSemiLazy bool, tips hornet.Hashes, err error)

// Spammer is used to issue messages to the IOTA network to create load on the tangle.
type Spammer struct {

	// config options
	message         string
	index           string
	indexSemiLazy   string
	tipselFunc      SpammerTipselFunc
	mwm             int
	powHandler      *pow.Handler
	sendMessageFunc SendMessageFunc
}

// New creates a new spammer instance.
func New(message string, index string, indexSemiLazy string, tipselFunc SpammerTipselFunc, mwm int, powHandler *pow.Handler, sendMessageFunc SendMessageFunc) *Spammer {

	return &Spammer{
		message:         message,
		index:           index,
		indexSemiLazy:   indexSemiLazy,
		tipselFunc:      tipselFunc,
		mwm:             mwm,
		powHandler:      powHandler,
		sendMessageFunc: sendMessageFunc,
	}
}

func (s *Spammer) DoSpam(shutdownSignal <-chan struct{}) (time.Duration, time.Duration, error) {

	timeStart := time.Now()
	isSemiLazy, tips, err := s.tipselFunc()
	if err != nil {
		return time.Duration(0), time.Duration(0), err
	}
	durationGTTA := time.Since(timeStart)

	indexation := s.index
	if isSemiLazy {
		indexation = s.indexSemiLazy
	}

	txCount := int(metrics.SharedServerMetrics.SentSpamTransactions.Load()) + 1

	now := time.Now()
	messageString := s.message
	messageString += fmt.Sprintf("\nCount: %06d", txCount)
	messageString += fmt.Sprintf("\nTimestamp: %s", now.Format(time.RFC3339))
	messageString += fmt.Sprintf("\nTipselection: %v", durationGTTA.Truncate(time.Microsecond))

	iotaMsg := &iotago.Message{Version: 1, Parent1: tips[0].ID(), Parent2: tips[1].ID(), Payload: &iotago.IndexationPayload{Index: indexation, Data: []byte(messageString)}}

	msg, err := tangle.NewMessage(iotaMsg)
	if err != nil {
		return time.Duration(0), time.Duration(0), err
	}

	timeStart = time.Now()
	if err := s.powHandler.DoPoW(msg, s.mwm, shutdownSignal, 1); err != nil {
		return time.Duration(0), time.Duration(0), err
	}
	durationPOW := time.Since(timeStart)

	if err := s.sendMessageFunc(msg); err != nil {
		return time.Duration(0), time.Duration(0), err
	}

	return durationGTTA, durationPOW, nil
}
