package pow

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/pow"
)

const (
	nonceBytes = 8 // len(uint64)
)

type proofOfWorkFunc func(ctx context.Context, data []byte, parallelism ...int) (uint64, error)

// RefreshTipsFunc refreshes tips of the message if PoW takes longer than a configured duration.
type RefreshTipsFunc = func() (tips hornet.MessageIDs, err error)

// Handler handles PoW requests of the node and uses local PoW.
// It refreshes the tips of messages during PoW.
type Handler struct {
	targetScore         float64
	refreshTipsInterval time.Duration

	localPoWFunc proofOfWorkFunc
	localPoWType string
}

// New creates a new PoW handler instance.
func New(targetScore float64, refreshTipsInterval time.Duration) *Handler {

	localPoWType := "local"
	localPoWFunc := func(ctx context.Context, data []byte, parallelism ...int) (uint64, error) {
		return pow.New(parallelism...).Mine(ctx, data, targetScore)
	}

	return &Handler{
		targetScore:         targetScore,
		refreshTipsInterval: refreshTipsInterval,
		localPoWFunc:        localPoWFunc,
		localPoWType:        localPoWType,
	}
}

// PoWType returns the fastest available PoW type which gets used for PoW requests
func (h *Handler) PoWType() string {
	return h.localPoWType
}

// DoPoW does the proof-of-work required to hit the target score configured on this Handler.
// The given iota.Message's nonce is automatically updated.
func (h *Handler) DoPoW(ctx context.Context, msg *iotago.Message, parallelism int, refreshTipsFunc ...RefreshTipsFunc) (err error) {

	if err := utils.ReturnErrIfCtxDone(ctx, common.ErrOperationAborted); err != nil {
		return err
	}

	getPoWData := func(msg *iotago.Message) (powData []byte, err error) {
		msgData, err := msg.Serialize(serializer.DeSeriModeNoValidation, iotago.ZeroRentParas)
		if err != nil {
			return nil, fmt.Errorf("unable to perform PoW as msg can't be serialized: %w", err)
		}

		return msgData[:len(msgData)-nonceBytes], nil
	}

	powData, err := getPoWData(msg)
	if err != nil {
		return err
	}

	refreshTips := len(refreshTipsFunc) > 0 && refreshTipsFunc[0] != nil

	doPow := func(ctx context.Context) (uint64, error) {
		powCtx, powCancel := context.WithCancel(ctx)
		defer powCancel()

		if refreshTips {
			var powTimeoutCancel context.CancelFunc
			powCtx, powTimeoutCancel = context.WithTimeout(powCtx, h.refreshTipsInterval)
			defer powTimeoutCancel()
		}

		nonce, err := h.localPoWFunc(powCtx, powData, parallelism)
		if err != nil {
			if errors.Is(err, pow.ErrCancelled) && refreshTips {
				// context was canceled and tips can be refreshed
				tips, err := refreshTipsFunc[0]()
				if err != nil {
					return 0, err
				}
				msg.Parents = tips.ToSliceOfArrays()

				// replace the powData to update the new tips
				powData, err = getPoWData(msg)
				if err != nil {
					return 0, err
				}

				return 0, pow.ErrCancelled
			}
			return 0, err
		}

		return nonce, nil
	}

	for {
		nonce, err := doPow(ctx)
		if err != nil {
			if errors.Is(err, pow.ErrCancelled) {
				// redo the PoW with new tips
				continue
			}
			return err
		}

		msg.Nonce = nonce
		return nil
	}
}
