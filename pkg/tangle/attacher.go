package tangle

import (
	"context"
	"time"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

var (
	ErrMessageAttacherInvalidMessage       = errors.New("invalid message")
	ErrMessageAttacherAttachingNotPossible = errors.New("attaching not possible")
	ErrMessageAttacherPoWNotAvailable      = errors.New("proof of work is not available on this node")
)

type MessageAttacherOption func(opts *MessageAttacherOptions)

type MessageAttacherOptions struct {
	tipSelFunc                pow.RefreshTipsFunc
	minPoWScore               float64
	messageProcessedTimeout   time.Duration
	deserializationParameters *iotago.DeSerializationParameters

	powHandler     *pow.Handler
	powWorkerCount int
}

func attacherOptions(opts []MessageAttacherOption) *MessageAttacherOptions {
	result := &MessageAttacherOptions{
		tipSelFunc:                nil,
		minPoWScore:               0,
		messageProcessedTimeout:   100 * time.Second,
		deserializationParameters: iotago.ZeroRentParas,
		powHandler:                nil,
		powWorkerCount:            0,
	}

	for _, opt := range opts {
		opt(result)
	}
	return result
}

func WithTimeout(messageProcessedTimeout time.Duration) MessageAttacherOption {
	return func(opts *MessageAttacherOptions) {
		opts.messageProcessedTimeout = messageProcessedTimeout
	}
}

func WithMinPoWScore(minPoWScore float64) MessageAttacherOption {
	return func(opts *MessageAttacherOptions) {
		opts.minPoWScore = minPoWScore
	}
}

func WithDeserializationParameters(deserializationParameters *iotago.DeSerializationParameters) MessageAttacherOption {
	return func(opts *MessageAttacherOptions) {
		opts.deserializationParameters = deserializationParameters
	}
}

func WithTipSel(tipsFunc pow.RefreshTipsFunc) MessageAttacherOption {
	return func(opts *MessageAttacherOptions) {
		opts.tipSelFunc = tipsFunc
	}
}

func WithPoW(handler *pow.Handler, workerCount int) MessageAttacherOption {
	return func(opts *MessageAttacherOptions) {
		opts.powHandler = handler
		opts.powWorkerCount = workerCount
	}
}

type MessageAttacher struct {
	tangle *Tangle
	opts   *MessageAttacherOptions
}

func (t *Tangle) MessageAttacher(opts ...MessageAttacherOption) *MessageAttacher {
	return &MessageAttacher{
		tangle: t,
		opts:   attacherOptions(opts),
	}
}

func (a *MessageAttacher) AttachMessage(ctx context.Context, msg *iotago.Message) (hornet.MessageID, error) {
	if len(msg.Parents) == 0 {
		if a.opts.tipSelFunc == nil {
			return nil, errors.WithMessage(ErrMessageAttacherInvalidMessage, "no parents given and node tipselection disabled")
		}

		tips, err := a.opts.tipSelFunc()
		if err != nil {
			return nil, errors.WithMessage(ErrMessageAttacherAttachingNotPossible, err.Error())
		}
		msg.Parents = tips.ToSliceOfArrays()
	}

	if msg.Nonce == 0 {
		score, err := msg.POW()
		if err != nil {
			return nil, errors.WithMessagef(ErrMessageAttacherInvalidMessage, err.Error())
		}

		if score < a.opts.minPoWScore {
			if a.opts.powHandler == nil {
				return nil, ErrMessageAttacherPoWNotAvailable
			}

			powCtx, ctxCancel := context.WithCancel(ctx)
			defer ctxCancel()

			if err := a.opts.powHandler.DoPoW(powCtx, msg, a.opts.powWorkerCount, a.opts.tipSelFunc); err != nil {
				return nil, err
			}
		}
	}

	message, err := storage.NewMessage(msg, serializer.DeSeriModePerformValidation, a.opts.deserializationParameters)
	if err != nil {
		return nil, errors.WithMessagef(ErrMessageAttacherInvalidMessage, err.Error())
	}

	msgProcessedChan := a.tangle.RegisterMessageProcessedEvent(message.MessageID())

	if err := a.tangle.messageProcessor.Emit(message); err != nil {
		a.tangle.DeregisterMessageProcessedEvent(message.MessageID())
		return nil, errors.WithMessagef(ErrMessageAttacherInvalidMessage, err.Error())
	}

	// wait for at most "messageProcessedTimeout" for the message to be processed
	ctx, cancel := context.WithTimeout(context.Background(), a.opts.messageProcessedTimeout)
	defer cancel()

	if err := utils.WaitForChannelClosed(ctx, msgProcessedChan); errors.Is(err, context.DeadlineExceeded) {
		a.tangle.DeregisterMessageProcessedEvent(message.MessageID())
	}

	return message.MessageID(), nil
}
