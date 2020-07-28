package pow

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/iotaledger/iota.go/pow"
	"github.com/iotaledger/iota.go/trinary"
	powsrvio "gitlab.com/powsrv.io/go/client"
)

var (
	errTooManyArguments = errors.New("too many arguments")
)

// Handler struct
type Handler struct {
	sync.RWMutex

	powsrvClient *powsrvio.PowClient
	localPoWFunc pow.ProofOfWorkFunc
	powType      string
}

// NewHandler creates a new PoW handler instance
func NewHandler(preferLocalPoW ...bool) (*Handler, error) {

	var localPoWFunc pow.ProofOfWorkFunc
	var powsrvClient *powsrvio.PowClient
	var powType string

	if len(preferLocalPoW) > 1 {
		return nil, fmt.Errorf("%w: Got %d arguments, wanted 1 argument", errTooManyArguments, len(preferLocalPoW))
	}

	prefLocalPoW := false
	if len(preferLocalPoW) == 1 {
		prefLocalPoW = preferLocalPoW[0]
	}

	// Get the fastest available local PoW func
	powType, localPoWFunc = pow.GetFastestProofOfWorkImpl()

	// Check wether a powsrv.io API key is set
	if powsrvAPIKey, isSet := os.LookupEnv("POWSRV_API_KEY"); isSet && !prefLocalPoW {
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
		localPoWFunc: localPoWFunc,
		powsrvClient: powsrvClient,
		powType:      powType,
	}, nil
}

// GetPoWType returns the fastest available PoW type which gets used for PoW requests
func (h *Handler) GetPoWType() string {
	return h.powType
}

// DoPoW calculates the PoW
func (h *Handler) DoPoW(trytes trinary.Trytes, mwm int, parallelism ...int) (nonce string, err error) {
	h.Lock()
	defer h.Unlock()

	// Use fast powsrv.io PoW if it's available and no error occurres
	// powsrv.io only accepts mwm <= 14
	if h.powsrvClient != nil && mwm <= 14 {
		nonce, err := h.powsrvClient.PowFunc(trytes, mwm, parallelism...)
		if err == nil {
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
