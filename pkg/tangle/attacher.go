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
	ErrBlockAttacherInvalidBlock         = errors.New("invalid block")
	ErrBlockAttacherAttachingNotPossible = errors.New("attaching not possible")
	ErrBlockAttacherPoWNotAvailable      = errors.New("proof of work is not available on this node")
)

type BlockAttacherOption func(opts *BlockAttacherOptions)

type BlockAttacherOptions struct {
	tipSelFunc            pow.RefreshTipsFunc
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

func WithTipSel(tipsFunc pow.RefreshTipsFunc) BlockAttacherOption {
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

func (a *BlockAttacher) AttachBlock(ctx context.Context, iotaBlock *iotago.Block) (hornet.BlockID, error) {

	var tipSelFunc pow.RefreshTipsFunc

	if len(iotaBlock.Parents) == 0 {
		if a.opts.tipSelFunc == nil {
			return nil, errors.WithMessage(ErrBlockAttacherInvalidBlock, "no parents given and node tipselection disabled")
		}
		tipSelFunc = a.opts.tipSelFunc
		tips, err := a.opts.tipSelFunc()
		if err != nil {
			return nil, errors.WithMessage(ErrBlockAttacherAttachingNotPossible, err.Error())
		}
		iotaBlock.Parents = tips.ToSliceOfArrays()
	}

	switch iotaBlock.Payload.(type) {

	case *iotago.Milestone:
		// enforce milestone iotaBlock nonce == 0
		iotaBlock.Nonce = 0

	default:
		if iotaBlock.Nonce == 0 {
			score, err := iotaBlock.POW()
			if err != nil {
				return nil, errors.WithMessagef(ErrBlockAttacherInvalidBlock, err.Error())
			}

			if score < a.tangle.protoParas.MinPoWScore {
				if a.opts.powHandler == nil {
					return nil, ErrBlockAttacherPoWNotAvailable
				}

				powCtx, ctxCancel := context.WithCancel(ctx)
				defer ctxCancel()

				powWorkerCount := runtime.NumCPU() - 1
				if a.opts.powWorkerCount > 0 {
					powWorkerCount = a.opts.powWorkerCount
				}

				ts := time.Now()
				blockSize, err := a.opts.powHandler.DoPoW(powCtx, iotaBlock, powWorkerCount, tipSelFunc)
				if err != nil {
					return nil, err
				}
				if a.opts.powMetrics != nil {
					a.opts.powMetrics.PoWCompleted(blockSize, time.Since(ts))
				}
			}
		}
	}

	block, err := storage.NewBlock(iotaBlock, serializer.DeSeriModePerformValidation, a.tangle.protoParas)
	if err != nil {
		return nil, errors.WithMessagef(ErrBlockAttacherInvalidBlock, err.Error())
	}

	blockProcessedChan := a.tangle.RegisterBlockProcessedEvent(block.BlockID())

	if err := a.tangle.messageProcessor.Emit(block); err != nil {
		a.tangle.DeregisterBlockProcessedEvent(block.BlockID())
		return nil, errors.WithMessagef(ErrBlockAttacherInvalidBlock, err.Error())
	}

	// wait for at most "blockProcessedTimeout" for the block to be processed
	ctx, cancel := context.WithTimeout(context.Background(), a.opts.blockProcessedTimeout)
	defer cancel()

	if err := events.WaitForChannelClosed(ctx, blockProcessedChan); errors.Is(err, context.DeadlineExceeded) {
		a.tangle.DeregisterBlockProcessedEvent(block.BlockID())
	}

	return block.BlockID(), nil
}
