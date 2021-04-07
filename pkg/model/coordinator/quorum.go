package coordinator

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/syncutils"
	iotago "github.com/iotaledger/iota.go/v2"
)

var (
	// ErrQuorumMerkleTreeHashMismatch is fired when a client in the quorum returns a different merkle tree hash.
	ErrQuorumMerkleTreeHashMismatch = errors.New("coordinator quorum merkle tree hash mismatch")
	// ErrQuorumGroupNoAnswer is fired when none of the clients in a quorum group answers.
	ErrQuorumGroupNoAnswer = errors.New("coordinator quorum group did not answer in time")
)

// QuorumClientConfig holds the configuration of a quorum client.
type QuorumClientConfig struct {
	// optional alias of the quorum client.
	Alias string `json:"alias" koanf:"alias"`
	// baseURL of the quorum client.
	BaseURL string `json:"baseURL" koanf:"baseURL"`
	// optional username for basic auth.
	UserName string `json:"userName" koanf:"userName"`
	// optional password for basic auth.
	Password string `json:"password" koanf:"password"`
}

// QuorumClientStatistic holds statistics of a quorum client.
type QuorumClientStatistic struct {
	// name of the quorum group the client is member of.
	Group string
	// optional alias of the quorum client.
	Alias string
	// baseURL of the quorum client.
	BaseURL string
	// last response time of the whiteflag API call.
	ResponseTimeSeconds float64
	// error of last whiteflag API call.
	Error error
}

// QuorumFinishedResult holds statistics of a finished quorum.
type QuorumFinishedResult struct {
	Duration time.Duration
	Err      error
}

// quorumGroupEntry holds the api and statistics of a quorum client.
type quorumGroupEntry struct {
	api   *DebugNodeAPIClient
	stats *QuorumClientStatistic
}

// quorum is used to check the correct ledger state of the coordinator.
type quorum struct {
	// the different groups of the quorum.
	Groups map[string][]*quorumGroupEntry
	// the maximim timeout of a quorum request.
	Timeout time.Duration

	quorumStatsLock syncutils.RWMutex
}

// newQuorum creates a new quorum, which is used to check the correct ledger state of the coordinator.
// If no groups are given, nil is returned.
func newQuorum(quorumGroups map[string][]*QuorumClientConfig, timeout time.Duration) *quorum {
	if len(quorumGroups) == 0 {
		panic("coordinator quorum groups not found")
	}

	groups := make(map[string][]*quorumGroupEntry)
	for groupName, groupNodes := range quorumGroups {
		if len(groupNodes) == 0 {
			panic(fmt.Sprintf("invalid coo quorum group: %s, no nodes given", groupName))
		}

		groups[groupName] = make([]*quorumGroupEntry, len(groupNodes))
		for i, client := range groupNodes {
			var userInfo *url.Userinfo
			if client.UserName != "" || client.Password != "" {
				userInfo = url.UserPassword(client.UserName, client.Password)
			}

			groups[groupName][i] = &quorumGroupEntry{
				api: NewDebugNodeAPIClient(client.BaseURL,
					iotago.WithNodeHTTPAPIClientHTTPClient(&http.Client{Timeout: timeout}),
					iotago.WithNodeHTTPAPIClientUserInfo(userInfo),
				),
				stats: &QuorumClientStatistic{
					Group:   groupName,
					Alias:   client.Alias,
					BaseURL: client.BaseURL,
				},
			}
		}
	}

	return &quorum{
		Groups:  groups,
		Timeout: timeout,
	}
}

// checkMerkleTreeHashQuorumGroup asks all nodes in a quorum group for their merkle tree hash based on the given parents.
// Returns non-critical and critical errors.
// If no node of the group answers, a non-critical error is returned.
// If one of the nodes returns a different hash, a critical error is returned.
func (q *quorum) checkMerkleTreeHashQuorumGroup(cooMerkleTreeHash MerkleTreeHash, groupName string, quorumGroupEntries []*quorumGroupEntry, wg *sync.WaitGroup, quorumDoneChan chan struct{}, quorumErrChan chan error, index milestone.Index, parents hornet.MessageIDs, onGroupEntryError func(groupName string, entry *quorumGroupEntry, err error)) {
	// mark the group as done at the end
	defer wg.Done()

	// cancel the quorum after a certain timeout
	ctx, cancel := context.WithTimeout(context.Background(), q.Timeout)
	defer cancel()

	nodeResultChan := make(chan MerkleTreeHash)
	defer close(nodeResultChan)

	nodeErrorChan := make(chan error)
	defer close(nodeErrorChan)

	for _, entry := range quorumGroupEntries {
		go func(entry *quorumGroupEntry, nodeResultChan chan MerkleTreeHash, nodeErrorChan chan error) {
			ts := time.Now()

			nodeMerkleTreeHash, err := entry.api.Whiteflag(index, parents)

			// set the stats for the node
			entry.stats.ResponseTimeSeconds = time.Since(ts).Seconds()
			entry.stats.Error = err

			if err != nil {
				if onGroupEntryError != nil {
					onGroupEntryError(groupName, entry, err)
				}
				nodeErrorChan <- err
				return
			}
			nodeResultChan <- *nodeMerkleTreeHash
		}(entry, nodeResultChan, nodeErrorChan)
	}

	validResults := 0
QuorumLoop:
	for i := 0; i < len(quorumGroupEntries); i++ {
		// we wait either until the channel got closed or the context is done
		select {
		case <-quorumDoneChan:
			// quorum was aborted
			return

		case <-nodeErrorChan:
			// ignore errors of single nodes
			continue

		case nodeMerkleTreeHash := <-nodeResultChan:
			if cooMerkleTreeHash != nodeMerkleTreeHash {
				// mismatch of the merkle tree hash of the node => critical error
				quorumErrChan <- common.CriticalError(ErrQuorumMerkleTreeHashMismatch)
				return
			}
			validResults++

		case <-ctx.Done():
			// quorum timeout reached
			break QuorumLoop
		}
	}

	if validResults == 0 {
		// no node of the group answered, return a non-critical error.
		quorumErrChan <- common.SoftError(ErrQuorumGroupNoAnswer)
	}
}

// checkMerkleTreeHash asks all nodes in the quorum for their merkle tree hash based on the given parents.
// Returns non-critical and critical errors.
// If no node of a certain group answers, a non-critical error is returned.
// If one of the nodes returns a different hash, a critical error is returned.
func (q *quorum) checkMerkleTreeHash(cooMerkleTreeHash MerkleTreeHash, index milestone.Index, parents hornet.MessageIDs, onGroupEntryError func(groupName string, entry *quorumGroupEntry, err error)) error {
	q.quorumStatsLock.Lock()
	defer q.quorumStatsLock.Unlock()

	wg := &sync.WaitGroup{}
	quorumDoneChan := make(chan struct{})
	quorumErrChan := make(chan error)

	for groupName, quorumGroupEntries := range q.Groups {
		wg.Add(1)

		// ask all groups in parallel
		go q.checkMerkleTreeHashQuorumGroup(cooMerkleTreeHash, groupName, quorumGroupEntries, wg, quorumDoneChan, quorumErrChan, index, parents, onGroupEntryError)
	}

	go func(wg *sync.WaitGroup, doneChan chan struct{}) {
		// wait for all groups to be finished
		wg.Wait()

		// signal that all groups are finished
		close(doneChan)
	}(wg, quorumDoneChan)

	select {
	case <-quorumDoneChan:
		// quorum finished successfully
		close(quorumErrChan)
		return nil

	case err := <-quorumErrChan:
		// quorum encountered an error
		return err
	}
}

// quorumStatsSnapshot returns a snapshot of the statistics about the response time and errors of every node in the quorum.
func (q *quorum) quorumStatsSnapshot() []QuorumClientStatistic {
	q.quorumStatsLock.RLock()
	defer q.quorumStatsLock.RUnlock()

	var stats []QuorumClientStatistic

	for _, quorumGroup := range q.Groups {
		for _, entry := range quorumGroup {
			stats = append(stats, *entry.stats)
		}
	}

	return stats
}
