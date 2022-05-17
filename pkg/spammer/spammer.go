package spammer

import (
	"context"
	"fmt"
	"time"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

// SendBlockFunc is a function which sends a block to the network.
type SendBlockFunc = func(block *storage.Block) error

// SpammerTipselFunc selects tips for the spammer.
type SpammerTipselFunc = func() (isSemiLazy bool, tips iotago.BlockIDs, err error)

// Spammer is used to issue blocks to the IOTA network to create load on the tangle.
type Spammer struct {
	// Deserialization parameters including byte costs
	protoParas    *iotago.ProtocolParameters
	message       string
	tag           string
	tagSemiLazy   string
	tipselFunc    SpammerTipselFunc
	powHandler    *pow.Handler
	sendBlockFunc SendBlockFunc
	serverMetrics *metrics.ServerMetrics
}

// New creates a new spammer instance.
func New(protoParas *iotago.ProtocolParameters,
	message string,
	tag string,
	tagSemiLazy string,
	tipselFunc SpammerTipselFunc,
	powHandler *pow.Handler,
	sendBlockFunc SendBlockFunc,
	serverMetrics *metrics.ServerMetrics) *Spammer {

	return &Spammer{
		protoParas:    protoParas,
		message:       message,
		tag:           tag,
		tagSemiLazy:   tagSemiLazy,
		tipselFunc:    tipselFunc,
		powHandler:    powHandler,
		sendBlockFunc: sendBlockFunc,
		serverMetrics: serverMetrics,
	}
}

func (s *Spammer) DoSpam(ctx context.Context) (time.Duration, time.Duration, error) {

	timeStart := time.Now()
	isSemiLazy, tips, err := s.tipselFunc()
	if err != nil {
		return time.Duration(0), time.Duration(0), err
	}
	durationGTTA := time.Since(timeStart)

	tag := s.tag
	if isSemiLazy {
		tag = s.tagSemiLazy
	}

	tagBytes := []byte(tag)
	if len(tagBytes) > iotago.MaxTagLength {
		tagBytes = tagBytes[:iotago.MaxTagLength]
	}

	txCount := int(s.serverMetrics.SentSpamBlocks.Load()) + 1

	now := time.Now()
	messageString := s.message
	messageString += fmt.Sprintf("\nCount: %06d", txCount)
	messageString += fmt.Sprintf("\nTimestamp: %s", now.Format(time.RFC3339))
	messageString += fmt.Sprintf("\nTipselection: %v", durationGTTA.Truncate(time.Microsecond))

	iotaBlock := &iotago.Block{
		ProtocolVersion: s.protoParas.Version,
		Parents:         tips,
		Payload:         &iotago.TaggedData{Tag: tagBytes, Data: []byte(messageString)},
	}

	timeStart = time.Now()
	if _, err := s.powHandler.DoPoW(ctx, iotaBlock, 1, func() (tips iotago.BlockIDs, err error) {
		// refresh tips of the spammer if PoW takes longer than a configured duration.
		_, refreshedTips, err := s.tipselFunc()
		return refreshedTips, err
	}); err != nil {
		return time.Duration(0), time.Duration(0), err
	}
	durationPOW := time.Since(timeStart)

	block, err := storage.NewBlock(iotaBlock, serializer.DeSeriModePerformValidation, s.protoParas)
	if err != nil {
		return time.Duration(0), time.Duration(0), err
	}

	if err := s.sendBlockFunc(block); err != nil {
		return time.Duration(0), time.Duration(0), err
	}

	return durationGTTA, durationPOW, nil
}
