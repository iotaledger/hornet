package spammer

import (
	"fmt"
	"time"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/pow"
	iotago "github.com/iotaledger/iota.go/v2"
)

// SendMessageFunc is a function which sends a message to the network.
type SendMessageFunc = func(msg *storage.Message) error

// SpammerTipselFunc selects tips for the spammer.
type SpammerTipselFunc = func() (isSemiLazy bool, tips hornet.MessageIDs, err error)

// Spammer is used to issue messages to the IOTA network to create load on the tangle.
type Spammer struct {
	networkID       uint64
	message         string
	index           string
	indexSemiLazy   string
	tipselFunc      SpammerTipselFunc
	powHandler      *pow.Handler
	sendMessageFunc SendMessageFunc
	serverMetrics   *metrics.ServerMetrics
}

// New creates a new spammer instance.
func New(networkID uint64, message string, index string, indexSemiLazy string, tipselFunc SpammerTipselFunc, powHandler *pow.Handler, sendMessageFunc SendMessageFunc, serverMetrics *metrics.ServerMetrics) *Spammer {

	return &Spammer{
		networkID:       networkID,
		message:         message,
		index:           index,
		indexSemiLazy:   indexSemiLazy,
		tipselFunc:      tipselFunc,
		powHandler:      powHandler,
		sendMessageFunc: sendMessageFunc,
		serverMetrics:   serverMetrics,
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

	index := []byte(indexation)
	if len(index) > storage.IndexationIndexLength {
		index = index[:storage.IndexationIndexLength]
	}

	txCount := int(s.serverMetrics.SentSpamMessages.Load()) + 1

	now := time.Now()
	messageString := s.message
	messageString += fmt.Sprintf("\nCount: %06d", txCount)
	messageString += fmt.Sprintf("\nTimestamp: %s", now.Format(time.RFC3339))
	messageString += fmt.Sprintf("\nTipselection: %v", durationGTTA.Truncate(time.Microsecond))

	iotaMsg := &iotago.Message{
		NetworkID: s.networkID,
		Parents:   tips.ToSliceOfArrays(),
		Payload:   &iotago.Indexation{Index: index, Data: []byte(messageString)},
	}

	timeStart = time.Now()
	if err := s.powHandler.DoPoW(iotaMsg, shutdownSignal, 1); err != nil {
		return time.Duration(0), time.Duration(0), err
	}
	durationPOW := time.Since(timeStart)

	msg, err := storage.NewMessage(iotaMsg, iotago.DeSeriModePerformValidation)
	if err != nil {
		return time.Duration(0), time.Duration(0), err
	}

	if err := s.sendMessageFunc(msg); err != nil {
		return time.Duration(0), time.Duration(0), err
	}

	return durationGTTA, durationPOW, nil
}
