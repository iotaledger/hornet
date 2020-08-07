package pow

import (
	"time"

	"github.com/iotaledger/iota.go/pow"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/syncutils"

	powsrvio "gitlab.com/powsrv.io/go/client"
)

// Handler handles PoW requests of the node and tunnels them to powsrv.io
// or uses local PoW if no API key was specified or the connection failed.
type Handler struct {
	log *logger.Logger

	powsrvClient       *powsrvio.PowClient
	powsrvLock         syncutils.RWMutex
	powsrvInitCooldown time.Duration
	powsrvLastInit     time.Time
	powsrvConnected    bool
	powsrvErrorHandled bool

	localPoWFunc pow.ProofOfWorkFunc
	localPowType string
}

// New creates a new PoW handler instance.
func New(log *logger.Logger, powsrvAPIKey string, powsrvInitCooldown time.Duration) *Handler {

	// Get the fastest available local PoW func
	localPoWType, localPoWFunc := pow.GetFastestProofOfWorkUnsyncImpl()

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
		log:                log,
		powsrvClient:       powsrvClient,
		powsrvInitCooldown: powsrvInitCooldown,
		powsrvLastInit:     time.Time{},
		powsrvConnected:    false,
		powsrvErrorHandled: false,
		localPoWFunc:       localPoWFunc,
		localPowType:       localPoWType,
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
			h.log.Warnf("Error connecting to powsrv.io: %w", err)
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

// GetPoWType returns the fastest available PoW type which gets used for PoW requests
func (h *Handler) GetPoWType() string {
	h.powsrvLock.RLock()
	defer h.powsrvLock.RUnlock()

	if h.powsrvConnected {
		return "powsrv.io"
	}

	return h.localPowType
}

// DoPoW calculates the PoW
// Either with the fastest available local PoW function or with the help of powsrv.io (optional, POWSRV_API_KEY env var must be available)
func (h *Handler) DoPoW(trytes trinary.Trytes, mwm int, parallelism ...int) (nonce string, err error) {

	if h.connectPowsrv() {
		// connected to powsrv.io
		// powsrv.io only accepts mwm <= 14
		if mwm <= 14 {
			h.powsrvLock.RLock()
			nonce, err := h.powsrvClient.PowFunc(trytes, mwm)
			if err == nil {
				h.powsrvLock.RUnlock()
				return nonce, nil
			}
			h.powsrvLock.RUnlock()

			h.powsrvLock.Lock()
			if !h.powsrvErrorHandled {
				// some error occurred => disconnect from powsrv.io
				if h.log != nil {
					h.log.Warnf("Error during PoW via powsrv.io: %w", err)
				}
				h.disconnectPowsrv()
			}
			h.powsrvLock.Unlock()
		}
	}

	// Local PoW
	return h.localPoWFunc(trytes, mwm, parallelism...)
}

// Close closes the PoW handler
func (h *Handler) Close() {
	h.powsrvLock.Lock()
	defer h.powsrvLock.Unlock()

	h.disconnectPowsrv()
}
