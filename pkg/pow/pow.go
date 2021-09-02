package pow

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"

	powsrvio "gitlab.com/powsrv.io/go/client"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/syncutils"
	iotago "github.com/iotaledger/iota.go/v2"
	"github.com/iotaledger/iota.go/v2/pow"
)

const (
	nonceBytes = 8 // len(uint64)
)

type proofOfWorkFunc func(ctx context.Context, data []byte, parallelism ...int) (uint64, error)

// RefreshTipsFunc refreshes tips of the message if PoW takes longer than a configured duration.
type RefreshTipsFunc = func() (tips hornet.MessageIDs, err error)

// Handler handles PoW requests of the node and tunnels them to powsrv.io
// or uses local PoW if no API key was specified or the connection failed.
type Handler struct {
	log *logger.Logger

	targetScore         float64
	refreshTipsInterval time.Duration

	powsrvClient       *powsrvio.PowClient
	powsrvLock         syncutils.RWMutex
	powsrvInitCooldown time.Duration
	powsrvLastInit     time.Time
	powsrvConnected    bool
	powsrvErrorHandled bool

	localPoWFunc proofOfWorkFunc
	localPoWType string
}

// New creates a new PoW handler instance.
// If the given powsrv.io API key is not empty, powsrv.io will be used to do proof-of-work.
func New(log *logger.Logger, targetScore float64, refreshTipsInterval time.Duration, powsrvAPIKey string, powsrvInitCooldown time.Duration) *Handler {

	localPoWType := "local"
	localPoWFunc := func(ctx context.Context, data []byte, parallelism ...int) (uint64, error) {
		return pow.New(parallelism...).Mine(ctx, data, targetScore)
	}

	var powsrvClient *powsrvio.PowClient

	// Check if powsrv.io API key is set
	if powsrvAPIKey != "" {
		powsrvClient = &powsrvio.PowClient{
			APIKey:        powsrvAPIKey,
			ReadTimeOutMs: 3000,
			Verbose:       false,
		}
	}

	return &Handler{
		log:                 log,
		targetScore:         targetScore,
		refreshTipsInterval: refreshTipsInterval,
		powsrvClient:        powsrvClient,
		powsrvInitCooldown:  powsrvInitCooldown,
		powsrvLastInit:      time.Time{},
		powsrvConnected:     false,
		powsrvErrorHandled:  false,
		localPoWFunc:        localPoWFunc,
		localPoWType:        localPoWType,
	}
}

// connectPowsrv tries to connect to powsrv.io if not connected already.
// it returns if the powsrv is connected or not.
func (h *Handler) connectPowsrv() bool {

	if h.powsrvClient == nil {
		return false
	}

	h.powsrvLock.RLock()
	if h.powsrvConnected {
		h.powsrvLock.RUnlock()
		return true
	}

	if time.Since(h.powsrvLastInit) < h.powsrvInitCooldown {
		h.powsrvLock.RUnlock()
		return false
	}
	h.powsrvLock.RUnlock()

	// acquire write lock
	h.powsrvLock.Lock()
	defer h.powsrvLock.Unlock()

	// check again after acquiring the write lock
	if h.powsrvConnected || time.Since(h.powsrvLastInit) < h.powsrvInitCooldown {
		return h.powsrvConnected
	}

	h.powsrvLastInit = time.Now()

	// close an existing connection first
	h.powsrvClient.Close()

	// connect to powsrv.io
	if err := h.powsrvClient.Init(); err != nil {
		if h.log != nil {
			h.log.Warnf("Error connecting to powsrv.io: %s", err)
		}
		return false
	}

	h.powsrvConnected = true
	h.powsrvErrorHandled = false
	return true
}

// disconnectPowsrv disconnects from powsrv.io
// write lock must be acquired outside.
func (h *Handler) disconnectPowsrv() {

	if h.powsrvErrorHandled {
		// error was already handled
		// we don't have to disconnect twice because of an error
		return
	}
	h.powsrvErrorHandled = true

	if !h.powsrvConnected {
		// already disconnected
		return
	}

	h.powsrvConnected = false

	if h.powsrvClient == nil {
		return
	}

	h.powsrvClient.Close()
}

// PoWType returns the fastest available PoW type which gets used for PoW requests
func (h *Handler) PoWType() string {
	h.powsrvLock.RLock()
	defer h.powsrvLock.RUnlock()

	if h.powsrvConnected {
		return "powsrv.io"
	}

	return h.localPoWType
}

// DoPoW does the proof-of-work required to hit the target score configured on this Handler.
// The given iota.Message's nonce is automatically updated.
// If a powsrv.io key was provided, then powsrv.io is used to commence the proof-of-work.
func (h *Handler) DoPoW(msg *iotago.Message, shutdownSignal <-chan struct{}, parallelism int, refreshTipsFunc ...RefreshTipsFunc) (err error) {

	select {
	case <-shutdownSignal:
		return common.ErrOperationAborted
	default:
	}

	getPoWData := func(msg *iotago.Message) (powData []byte, err error) {
		msgData, err := msg.Serialize(iotago.DeSeriModeNoValidation)
		if err != nil {
			return nil, fmt.Errorf("unable to perform PoW as msg can't be serialized: %w", err)
		}

		return msgData[:len(msgData)-nonceBytes], nil
	}

	powData, err := getPoWData(msg)
	if err != nil {
		return err
	}

	if h.connectPowsrv() {
		// connected to powsrv.io
		// powsrv.io only accepts targetScore <= 4000
		if h.targetScore <= 4000 {

			h.powsrvLock.RLock()
			nonce, err := h.powsrvClient.Mine(powData, h.targetScore)
			if err == nil {
				h.powsrvLock.RUnlock()
				msg.Nonce = nonce
				return nil
			}
			h.powsrvLock.RUnlock()

			h.powsrvLock.Lock()
			if !h.powsrvErrorHandled {
				// some error occurred => disconnect from powsrv.io
				if h.log != nil {
					h.log.Warnf("Error during PoW via powsrv.io: %s", err)
				}
				h.disconnectPowsrv()
			}
			h.powsrvLock.Unlock()
		}
	}

	refreshTips := len(refreshTipsFunc) > 0 && refreshTipsFunc[0] != nil

	// Fall back to local PoW
	for {
		powCtx, powCancel := context.WithCancel(context.Background())
		if refreshTips {
			powCtx, powCancel = context.WithTimeout(powCtx, h.refreshTipsInterval)
		}

		nonce, err := h.localPoWFunc(powCtx, powData, parallelism)
		powCancel()

		if err != nil {
			if errors.Is(err, pow.ErrCancelled) && refreshTips {
				// context was canceled and tips can be refreshed
				tips, err := refreshTipsFunc[0]()
				if err != nil {
					return err
				}
				msg.Parents = tips.ToSliceOfArrays()

				powData, err = getPoWData(msg)
				if err != nil {
					return err
				}

				// redo the PoW with new tips
				continue
			}
			return err
		}

		msg.Nonce = nonce
		return nil
	}
}

// Close closes the PoW handler
func (h *Handler) Close() {
	h.powsrvLock.Lock()
	defer h.powsrvLock.Unlock()

	h.disconnectPowsrv()
}
