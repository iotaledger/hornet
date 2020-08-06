package pow

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/iotaledger/iota.go/pow"
	"github.com/iotaledger/iota.go/trinary"
	powsrvio "gitlab.com/powsrv.io/go/client"
)

// Handler handles PoW requests of the node and tunnels them to powsrv.io
// or uses local PoW if no API key was specified or the connection failed.
type Handler struct {
	sync.RWMutex

	powsrvClient         *powsrvio.PowClient
	powsrvReinitCooldown time.Duration
	powsrvLastInit       time.Time
	powsrvConnected      int32

	localPoWFunc pow.ProofOfWorkFunc
	localPowType string
}

// New creates a new PoW handler instance.
func New(powsrvAPIKey string, powsrvReinitCooldown time.Duration) *Handler {

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
		powsrvClient:         powsrvClient,
		powsrvReinitCooldown: powsrvReinitCooldown,
		powsrvLastInit:       time.Time{},
		powsrvConnected:      0,
		localPoWFunc:         localPoWFunc,
		localPowType:         localPoWType,
	}
}

// tryReinitPowsrvWithoutLocking tries to reinit powsrv.io if the connection got lost.
// the lock needs to be handled by the caller
func (h *Handler) tryReinitPowsrvWithoutLocking() {
	if h.powsrvClient == nil {
		return
	}

	if time.Since(h.powsrvLastInit) >= h.powsrvReinitCooldown {

		h.powsrvLastInit = time.Now()
		if err := h.powsrvClient.Init(); err != nil {
			return
		}
		atomic.StoreInt32(&(h.powsrvConnected), 1)
	}
}

// GetPoWType returns the fastest available PoW type which gets used for PoW requests
func (h *Handler) GetPoWType() string {
	if atomic.LoadInt32(&(h.powsrvConnected)) == 1 {
		return "powsrv.io"
	}

	return h.localPowType
}

func (h *Handler) disconnect() {
	atomic.StoreInt32(&(h.powsrvConnected), 0)
	h.powsrvClient.Close()
}

// DoPoW calculates the PoW
// Either with the fastest available local PoW function or with the help of powsrv.io (optional, POWSRV_API_KEY env var must be available)
func (h *Handler) DoPoW(trytes trinary.Trytes, mwm int, parallelism ...int) (nonce string, err error) {
	h.Lock()
	defer h.Unlock()

	if atomic.LoadInt32(&(h.powsrvConnected)) == 0 {
		// powsrv.io not connected
		h.tryReinitPowsrvWithoutLocking()
	}

	if atomic.LoadInt32(&(h.powsrvConnected)) == 1 {
		// connected to powsrv.io
		// powsrv.io only accepts mwm <= 14
		if mwm <= 14 {
			nonce, err := h.powsrvClient.PowFunc(trytes, mwm)
			if err == nil {
				return nonce, nil
			}

			// some error occured => disconnect from powsrv.io
			h.disconnect()
		}
	}

	// Local PoW
	return h.localPoWFunc(trytes, mwm, parallelism...)
}

// Close closes the PoW handler
func (h *Handler) Close() {
	h.Lock()
	defer h.Unlock()

	if h.powsrvClient != nil {
		h.powsrvClient.Close()
	}
}
