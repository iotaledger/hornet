package tangle

import (
	"context"
	"runtime"
	"time"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/iotaledger/hive.go/events"
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
	tipSelFunc              pow.RefreshTipsFunc
	messageProcessedTimeout time.Duration

	powHandler     *pow.Handler
	powWorkerCount int
	powMetrics     metrics.PoWMetrics
}

func attacherOptions(opts []MessageAttacherOption) *MessageAttacherOptions {
	result := &MessageAttacherOptions{
		tipSelFunc:              nil,
		messageProcessedTimeout: 100 * time.Second,
		powHandler:              nil,
		powWorkerCount:          0,
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

func WithPoWMetrics(powMetrics metrics.PoWMetrics) MessageAttacherOption {
	return func(opts *MessageAttacherOptions) {
		opts.powMetrics = powMetrics
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

	var tipSelFunc pow.RefreshTipsFunc

	if len(msg.Parents) == 0 {
		if a.opts.tipSelFunc == nil {
			return nil, errors.WithMessage(ErrMessageAttacherInvalidMessage, "no parents given and node tipselection disabled")
		}
		tipSelFunc = a.opts.tipSelFunc
		tips, err := a.opts.tipSelFunc()
		if err != nil {
			return nil, errors.WithMessage(ErrMessageAttacherAttachingNotPossible, err.Error())
		}
		msg.Parents = tips.ToSliceOfArrays()
	}

	switch msg.Payload.(type) {

	case *iotago.Milestone:
		// enforce milestone msg nonce == 0
		msg.Nonce = 0

	default:
		if msg.Nonce == 0 {
			score, err := msg.POW()
			if err != nil {
				return nil, errors.WithMessagef(ErrMessageAttacherInvalidMessage, err.Error())
			}

			if score < a.tangle.protoParas.MinPoWScore {
				if a.opts.powHandler == nil {
					return nil, ErrMessageAttacherPoWNotAvailable
				}

				powCtx, ctxCancel := context.WithCancel(ctx)
				defer ctxCancel()

				powWorkerCount := runtime.NumCPU() - 1
				if a.opts.powWorkerCount > 0 {
					powWorkerCount = a.opts.powWorkerCount
				}

				ts := time.Now()
				messageSize, err := a.opts.powHandler.DoPoW(powCtx, msg, powWorkerCount, tipSelFunc)
				if err != nil {
					return nil, err
				}
				if a.opts.powMetrics != nil {
					a.opts.powMetrics.PoWCompleted(messageSize, time.Since(ts))
				}
			}
		}
	}

	message, err := storage.NewMessage(msg, serializer.DeSeriModePerformValidation, a.tangle.protoParas)
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

	if err := events.WaitForChannelClosed(ctx, msgProcessedChan); errors.Is(err, context.DeadlineExceeded) {
		a.tangle.DeregisterMessageProcessedEvent(message.MessageID())
	}

	return message.MessageID(), nil
}
