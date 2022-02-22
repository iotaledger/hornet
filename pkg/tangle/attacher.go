package tangle

import (
	"context"
	"time"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

var (
	ErrMessageAttacherInvalidMessage       = errors.New("invalid message")
	ErrMessageAttacherAttachingNotPossible = errors.New("attaching not possible")
	ErrMessageAttacherPoWNotAvailable      = errors.New("proof of work is not available on this node")
)

type MessageAttacher struct {
	tangle *Tangle

	tipSelector               *tipselect.TipSelector
	minPoWScore               float64
	messageProcessedTimeout   time.Duration
	deserializationParameters *iotago.DeSerializationParameters

	powHandler     *pow.Handler
	powWorkerCount int
}

func (t *Tangle) MessageAttacher(tipSel *tipselect.TipSelector, minPoWScore float64, messageProcessedTimeout time.Duration, deserializationParameters *iotago.DeSerializationParameters) *MessageAttacher {
	return &MessageAttacher{
		tipSelector:               tipSel,
		minPoWScore:               minPoWScore,
		messageProcessedTimeout:   messageProcessedTimeout,
		deserializationParameters: deserializationParameters,
		tangle:                    t,
	}
}

func (a *MessageAttacher) WithPoW(handler *pow.Handler, workerCount int) *MessageAttacher {
	a.powHandler = handler
	a.powWorkerCount = workerCount
	return a
}

func (a *MessageAttacher) AttachMessage(ctx context.Context, msg *iotago.Message) (hornet.MessageID, error) {
	var refreshTipsFunc pow.RefreshTipsFunc

	if len(msg.Parents) == 0 {
		if a.tipSelector == nil {
			return nil, errors.WithMessage(ErrMessageAttacherInvalidMessage, "no parents given and node tipselection disabled")
		}

		tips, err := a.tipSelector.SelectNonLazyTips()
		if err != nil {
			if errors.Is(err, common.ErrNodeNotSynced) || errors.Is(err, tipselect.ErrNoTipsAvailable) {
				return nil, errors.WithMessage(ErrMessageAttacherAttachingNotPossible, err.Error())
			}
			return nil, err
		}
		msg.Parents = tips.ToSliceOfArrays()

		// this function pointer is used to refresh the tips of a message
		// if no parents were given and the PoW takes longer than a configured duration.
		refreshTipsFunc = a.tipSelector.SelectNonLazyTips
	}

	if msg.Nonce == 0 {
		score, err := msg.POW()
		if err != nil {
			return nil, errors.WithMessagef(ErrMessageAttacherInvalidMessage, err.Error())
		}

		if score < a.minPoWScore {
			if a.powHandler == nil {
				return nil, ErrMessageAttacherPoWNotAvailable
			}

			powCtx, ctxCancel := context.WithCancel(ctx)
			defer ctxCancel()

			if err := a.powHandler.DoPoW(powCtx, msg, a.powWorkerCount, refreshTipsFunc); err != nil {
				return nil, err
			}
		}
	}

	message, err := storage.NewMessage(msg, serializer.DeSeriModePerformValidation, a.deserializationParameters)
	if err != nil {
		return nil, errors.WithMessagef(ErrMessageAttacherInvalidMessage, err.Error())
	}

	msgProcessedChan := a.tangle.RegisterMessageProcessedEvent(message.MessageID())

	if err := a.tangle.messageProcessor.Emit(message); err != nil {
		a.tangle.DeregisterMessageProcessedEvent(message.MessageID())
		return nil, errors.WithMessagef(ErrMessageAttacherInvalidMessage, err.Error())
	}

	// wait for at most "messageProcessedTimeout" for the message to be processed
	ctx, cancel := context.WithTimeout(context.Background(), a.messageProcessedTimeout)
	defer cancel()

	if err := utils.WaitForChannelClosed(ctx, msgProcessedChan); errors.Is(err, context.DeadlineExceeded) {
		a.tangle.DeregisterMessageProcessedEvent(message.MessageID())
	}

	return message.MessageID(), nil
}
