package pow

import (
	"context"
	"time"

	"github.com/iotaledger/hive.go/serializer/v2"
	inxpow "github.com/iotaledger/inx-app/pkg/pow"
	iotago "github.com/iotaledger/iota.go/v3"
)

// Handler handles PoW requests of the node and uses local PoW.
// It refreshes the tips of blocks during PoW.
type Handler struct {
	refreshTipsInterval time.Duration
}

// New creates a new PoW handler instance.
func New(refreshTipsInterval time.Duration) *Handler {
	return &Handler{
		refreshTipsInterval: refreshTipsInterval,
	}
}

// DoPoW does the proof-of-work required to hit the target score configured on this Handler.
// The given iota.Block's nonce is automatically updated.
func (h *Handler) DoPoW(ctx context.Context, block *iotago.Block, deSeriMode serializer.DeSerializationMode, protoParams *iotago.ProtocolParameters, parallelism int, refreshTipsFunc inxpow.RefreshTipsFunc) (blockSize int, err error) {
	return inxpow.DoPoW(ctx, block, deSeriMode, protoParams, parallelism, h.refreshTipsInterval, refreshTipsFunc)
}
