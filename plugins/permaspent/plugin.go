package permaspent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/OneOfOne/xxhash"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/gohornet/hornet/packages/parameter"
)

var (
	ErrNoAddressesSpecified   = errors.New("no adddresses specified")
	ErrTooManyFailedResponses = errors.New("could not determine spent state as too many nodes failed to respond")
	ErrNoQuorumReached        = errors.New("could not determine spent state as no quorum was reached")
)

var (
	PLUGIN              = node.NewPlugin("Permaspent", node.Enabled, configure)
	log                 *logger.Logger
	client              http.Client
	nodes               []string
	nodesLen            float64
	noResponseTolerance float64
	quorumThreshold     float64
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger("Permaspent")
	client = http.Client{Timeout: time.Duration(3) * time.Second}

	// grab config
	nodes = parameter.NodeConfig.GetStringSlice("permaspent.nodes")
	noResponseTolerance = parameter.NodeConfig.GetFloat64("permaspent.thresholds.noResponseTolerance")
	quorumThreshold = parameter.NodeConfig.GetFloat64("permaspent.thresholds.quorum")
	nodesLen = float64(len(nodes))

	log.Infof("Using following nodes for spent addresses: %v", nodes)
}

type (
	request struct {
		Addresses trinary.Hashes `json:"addresses"`
	}

	response struct {
		States []bool `json:"states"`
	}

	vote struct {
		count   int
		payload []bool
	}
)

func WereAddressesSpentFrom(addrs ...trinary.Hash) ([]bool, error) {
	if len(addrs) == 0 {
		return nil, ErrNoAddressesSpecified
	}

	var err error
	var reqBodyBytes []byte
	reqBody := &request{Addresses: addrs}
	if reqBodyBytes, err = json.Marshal(reqBody); err != nil {
		return nil, fmt.Errorf("can't create request body: %w", err)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	failedOps := make([]interface{}, 0)
	collectError := func(any error) {
		log.Error(any)
		mu.Lock()
		failedOps = append(failedOps, any)
		mu.Unlock()
	}
	votes := map[uint32]*vote{}
	for _, node := range nodes {
		wg.Add(1)
		node := node
		func() {
			defer wg.Done()
			req, err := http.NewRequest(http.MethodPost, node, bytes.NewReader(reqBodyBytes))
			if err != nil {
				collectError(err)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			res, err := client.Do(req)
			if err != nil {
				collectError(err)
				return
			}

			resBody, err := ioutil.ReadAll(res.Body)
			if err != nil {
				collectError(fmt.Errorf("unable to read response body: %w", err))
				return
			}
			defer res.Body.Close()

			if res.StatusCode != http.StatusOK {
				collectError(fmt.Errorf("request failed: %s", string(resBody)))
				return
			}

			mu.Lock()
			defer mu.Unlock()
			hash := xxhash.Checksum32(resBody)
			existing, has := votes[hash]
			if has {
				existing.count++
				return
			}

			resObj := &response{}
			if err := json.Unmarshal(resBody, resObj); err != nil {
				collectError(err)
				return
			}

			votes[hash] = &vote{count: 1, payload: resObj.States}
		}()
	}
	wg.Wait()

	// check error threshold
	noResCount := float64(len(failedOps))
	if noResCount/nodesLen > noResponseTolerance {
		return nil, fmt.Errorf("%w: %d of %d", ErrTooManyFailedResponses, int(noResCount), int(nodesLen))
	}

	var winningVote *vote
	for _, vote := range votes {
		if winningVote == nil || winningVote.count < vote.count {
			winningVote = vote
		}
	}

	// check whether quorum threshold is reached
	if float64(winningVote.count)/(nodesLen-noResCount) < quorumThreshold {
		return nil, fmt.Errorf("%w: %d of %d", ErrNoQuorumReached, winningVote.count, int(nodesLen-noResCount))
	}

	return winningVote.payload, nil
}
