package tangle

import (
	"context"
	"runtime"
	"time"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/events"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hornet/v2/pkg/metrics"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/pow"
	inxpow "github.com/iotaledger/inx-app/pow"
	iotago "github.com/iotaledger/iota.go/v3"
)

var (
	ErrBlockAttacherInvalidBlock         = errors.New("invalid block")
	ErrBlockAttacherAttachingNotPossible = errors.New("attaching not possible")
	ErrBlockAttacherPoWNotAvailable      = errors.New("proof of work is not available on this node")
)

type BlockAttacherOption func(opts *BlockAttacherOptions)

type BlockAttacherOptions struct {
	tipSelFunc            inxpow.RefreshTipsFunc
	blockProcessedTimeout time.Duration

	powHandler     *pow.Handler
	powWorkerCount int
	powMetrics     metrics.PoWMetrics
}

func attacherOptions(opts []BlockAttacherOption) *BlockAttacherOptions {
	result := &BlockAttacherOptions{
		tipSelFunc:            nil,
		blockProcessedTimeout: 100 * time.Second,
		powHandler:            nil,
		powWorkerCount:        0,
	}

	for _, opt := range opts {
		opt(result)
	}

	return result
}

func WithTimeout(blockProcessedTimeout time.Duration) BlockAttacherOption {
	return func(opts *BlockAttacherOptions) {
		opts.blockProcessedTimeout = blockProcessedTimeout
	}
}

func WithTipSel(tipsFunc inxpow.RefreshTipsFunc) BlockAttacherOption {
	return func(opts *BlockAttacherOptions) {
		opts.tipSelFunc = tipsFunc
	}
}

func WithPoW(handler *pow.Handler, workerCount int) BlockAttacherOption {
	return func(opts *BlockAttacherOptions) {
		opts.powHandler = handler
		opts.powWorkerCount = workerCount
	}
}

func WithPoWMetrics(powMetrics metrics.PoWMetrics) BlockAttacherOption {
	return func(opts *BlockAttacherOptions) {
		opts.powMetrics = powMetrics
	}
}

type BlockAttacher struct {
	tangle *Tangle
	opts   *BlockAttacherOptions
}

func (t *Tangle) BlockAttacher(opts ...BlockAttacherOption) *BlockAttacher {
	return &BlockAttacher{
		tangle: t,
		opts:   attacherOptions(opts),
	}
}

func (a *BlockAttacher) AttachBlock(ctx context.Context, iotaBlock *iotago.Block) (iotago.BlockID, error) {

	if iotaBlock.ProtocolVersion != a.tangle.protocolManager.Current().Version {
		return iotago.EmptyBlockID(), errors.WithMessagef(ErrBlockAttacherInvalidBlock, "protocolVersion invalid: %d", iotaBlock.ProtocolVersion)
	}

	targetScore := a.tangle.protocolManager.Current().MinPoWScore

	var tipSelFunc inxpow.RefreshTipsFunc

	if len(iotaBlock.Parents) == 0 {
		if a.opts.tipSelFunc == nil {
			return iotago.EmptyBlockID(), errors.WithMessage(ErrBlockAttacherInvalidBlock, "no parents given and node tipselection disabled")
		}

		if a.opts.powHandler == nil && targetScore != 0 {
			return iotago.EmptyBlockID(), errors.WithMessage(ErrBlockAttacherInvalidBlock, "no parents given and node PoW is disabled")
		}

		// only allow to update tips during proof of work if no parents were given
		tipSelFunc = a.opts.tipSelFunc

		tips, err := a.opts.tipSelFunc()
		if err != nil {
			return iotago.EmptyBlockID(), errors.WithMessagef(ErrBlockAttacherAttachingNotPossible, "tipselection failed, error: %s", err.Error())
		}

		iotaBlock.Parents = tips
	}

	switch iotaBlock.Payload.(type) {

	case *iotago.Milestone:
		// enforce milestone iotaBlock nonce == 0
		iotaBlock.Nonce = 0

	default:
		switch payload := iotaBlock.Payload.(type) {
		case *iotago.Transaction:
			if payload.Essence.NetworkID != a.tangle.protocolManager.Current().NetworkID() {
				return iotago.EmptyBlockID(), errors.WithMessagef(ErrBlockAttacherInvalidBlock, "invalid payload, error: wrong networkID: %d", payload.Essence.NetworkID)
			}
		}

		if iotaBlock.Nonce == 0 && targetScore != 0 {
			score, err := iotaBlock.POW()
			if err != nil {
				return iotago.EmptyBlockID(), errors.WithMessage(ErrBlockAttacherInvalidBlock, err.Error())
			}

			if score < float64(targetScore) {
				if a.opts.powHandler == nil {
					return iotago.EmptyBlockID(), ErrBlockAttacherPoWNotAvailable
				}

				powCtx, ctxCancel := context.WithCancel(ctx)
				defer ctxCancel()

				powWorkerCount := runtime.NumCPU() - 1
				if a.opts.powWorkerCount > 0 {
					powWorkerCount = a.opts.powWorkerCount
				}

				ts := time.Now()
				blockSize, err := a.opts.powHandler.DoPoW(powCtx, iotaBlock, targetScore, powWorkerCount, tipSelFunc)
				if err != nil {
					return iotago.EmptyBlockID(), err
				}
				if a.opts.powMetrics != nil {
					a.opts.powMetrics.PoWCompleted(blockSize, time.Since(ts))
				}
			}
		}
	}

	block, err := storage.NewBlock(iotaBlock, serializer.DeSeriModePerformValidation, a.tangle.protocolManager.Current())
	if err != nil {
		return iotago.EmptyBlockID(), errors.WithMessage(ErrBlockAttacherInvalidBlock, err.Error())
	}

	blockProcessedChan := a.tangle.RegisterBlockProcessedEvent(block.BlockID())

	//nolint:contextcheck // we don't pass a context here to not prevent emitting blocks at shutdown (COO etc).
	if err := a.tangle.messageProcessor.Emit(block); err != nil {
		a.tangle.DeregisterBlockProcessedEvent(block.BlockID())

		return iotago.EmptyBlockID(), errors.WithMessage(ErrBlockAttacherInvalidBlock, err.Error())
	}

	// wait for at most "blockProcessedTimeout" for the block to be processed
	ctxBlockProcessed, cancelBlockProcessed := context.WithTimeout(ctx, a.opts.blockProcessedTimeout)
	defer cancelBlockProcessed()

	if err := events.WaitForChannelClosed(ctxBlockProcessed, blockProcessedChan); errors.Is(err, context.DeadlineExceeded) {
		a.tangle.DeregisterBlockProcessedEvent(block.BlockID())
	}

	return block.BlockID(), nil
}
