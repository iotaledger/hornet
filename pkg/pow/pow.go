package pow

import (
	"sync"
	"time"

	"github.com/iotaledger/iota.go/pow"
	"github.com/iotaledger/iota.go/trinary"
	powsrvio "gitlab.com/powsrv.io/go/client"
)

// Handler struct
type Handler struct {
	sync.RWMutex

	powsrvClient         *powsrvio.PowClient
	localPoWFunc         pow.ProofOfWorkFunc
	powType              string
	powsrvReinitCooldown time.Duration
	lastInitTimestamp    time.Time
}

// New creates a new PoW handler instance
func New(powsrvAPIKey string, powsrvReinitCooldown time.Duration) *Handler {

	var localPoWFunc pow.ProofOfWorkFunc
	var powsrvClient *powsrvio.PowClient
	var powType string

	// Get the fastest available local PoW func
	powType, localPoWFunc = pow.GetFastestProofOfWorkImpl()

	// Check whether a powsrv.io API key is set
	if powsrvAPIKey != "" {
		powsrvClient = &powsrvio.PowClient{
			APIKey:        powsrvAPIKey,
			ReadTimeOutMs: 3000,
			Verbose:       false,
		}

		// Try to init the powsrv.io client. If it fails, fall back to local PoW
		if err := powsrvClient.Init(); err != nil {
			powsrvClient = nil
		} else {
			powType = "powsrv.io"
		}
	}

	return &Handler{
		localPoWFunc:         localPoWFunc,
		powsrvClient:         powsrvClient,
		powType:              powType,
		powsrvReinitCooldown: powsrvReinitCooldown,
		lastInitTimestamp:    time.Now(),
	}
}

// tryReinitPowsrvWithoutLocking tries to reinit powsrv.io if the connection got lost.
// the lock needs to be handled by the caller
func (h *Handler) tryReinitPowsrvWithoutLocking() {
	if h.powsrvClient == nil {
		return
	}

	if time.Since(h.lastInitTimestamp) >= h.powsrvReinitCooldown {
		h.powsrvClient.Close()
		h.powsrvClient.Init()
		h.lastInitTimestamp = time.Now()
	}
}

// GetPoWType returns the fastest available PoW type which gets used for PoW requests
func (h *Handler) GetPoWType() string {
	return h.powType
}

// DoPoW calculates the PoW
// Either with the fastest available local PoW function or with the help of powsrv.io (optional, POWSRV_API_KEY env var must be available)
func (h *Handler) DoPoW(trytes trinary.Trytes, mwm int, parallelism ...int) (nonce string, err error) {
	h.Lock()
	defer h.Unlock()

	// Use fast powsrv.io PoW if it's available and no error occurres
	// powsrv.io only accepts mwm <= 14
	if h.powsrvClient != nil && mwm <= 14 {
		nonce, err := h.powsrvClient.PowFunc(trytes, mwm, parallelism...)
		if err != nil {
			h.tryReinitPowsrvWithoutLocking()
		} else {
			return nonce, nil
		}
	}

	// Local PoW
	nonce, err = h.localPoWFunc(trytes, mwm, parallelism...)
	if err != nil {
		return "", err
	}

	return nonce, nil
}

// Close closes the PoW handler
func (h *Handler) Close() {
	h.Lock()
	defer h.Unlock()

	if h.powsrvClient != nil {
		h.powsrvClient.Close()
	}
}
