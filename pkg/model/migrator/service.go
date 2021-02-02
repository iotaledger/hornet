package migrator

import (
	"fmt"
	"os"

	"github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/iota.go/api"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/iotaledger/iota.go/v2"
)

const (
	// MaxReceipts defines the maximum size of a receipt returned by MigratorService.GetReceipt.
	MaxReceipts = 100
)

// State stores the latest state of the MigratorService.
type State struct {
	/*
	   4 bytes uint32 			LatestMilestoneIndex
	   4 bytes uint32 			LatestIncludedIndex
	*/
	LatestMilestoneIndex uint32
	LatestIncludedIndex  uint32
}

// MigratorService is a service querying and validating batches of migrated funds.
type MigratorService struct {
	validator *validator
	state     State

	mutex      syncutils.Mutex
	migrations chan *migrationResult

	stateFilePath string
}

type migrationResult struct {
	stopIndex     uint32
	lastBatch     bool
	migratedFunds []*iota.MigratedFundsEntry
}

// NewService creates a new MigratorService.
func NewService(api *api.API, stateFilePath string, coordinatorAddress trinary.Hash, coordinatorMerkleTreeDepth int) *MigratorService {
	return &MigratorService{
		validator:     newValidator(api, coordinatorAddress, coordinatorMerkleTreeDepth),
		migrations:    make(chan *migrationResult),
		stateFilePath: stateFilePath,
	}
}

// GetReceipt returns the next receipt of migrated funds.
// Each receipt can only consists of migrations confirmed by one milestone, it will never be larger than MaxReceipts.
// GetReceipt returns nil, if there are currently no new migrations available. Although the actual API calls and
// validations happen in the background, GetReceipt might block until the next receipt is ready.
// When s is stopped, GetReceipt will always return nil.
func (s *MigratorService) GetReceipt() *iota.Receipt {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	// make the channel receive and the state update atomic, so that the state always matches the result
	result, ok := <-s.migrations
	if !ok {
		return nil
	}
	s.updateState(result)
	return createReceipt(result.stopIndex, result.lastBatch, result.migratedFunds)
}

// PersistState persists the current state to a file.
// PersistState must be called when the receipt returned by the last call of GetReceipt has been send to the network.
func (s *MigratorService) PersistState() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return utils.WriteToFile(s.stateFilePath, &s.state, 0660)
}

// InitState initializes the state of s.
// If initialState is not nil, s is bootstrapped using it as its initial state.
// InitState must be called before Start.
func (s *MigratorService) InitState(state *State) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if state != nil {
		// check whether the state file exists
		if _, err := os.Stat(s.stateFilePath); !os.IsNotExist(err) {
			return coordinator.ErrNetworkBootstrapped
		}
		s.state = *state
		return nil
	}
	if err := utils.ReadFromFile(s.stateFilePath, &s.state); err != nil {
		return fmt.Errorf("failed to load state file: %w", err)
	}
	return nil
}

// Start stats the MigratorService s, it stops when shutdownSignal is closed.
func (s *MigratorService) Start(shutdownSignal <-chan struct{}) error {
	startIndex := s.state.LatestMilestoneIndex
	for {
		stopIndex, migratedFunds, err := s.validator.nextMigrations(startIndex)
		if err != nil {
			return err
		}
		// always continue with the next index
		startIndex = stopIndex + 1

		for {
			batch := migratedFunds
			lastBatch := true
			if len(batch) > MaxReceipts {
				batch = batch[:MaxReceipts]
				lastBatch = false
			}
			select {
			case s.migrations <- &migrationResult{stopIndex, lastBatch, batch}:
			case <-shutdownSignal:
				close(s.migrations)
				return nil
			}
			migratedFunds = migratedFunds[len(batch):]
			if len(migratedFunds) == 0 {
				break
			}
		}
	}
}

func (s *MigratorService) updateState(result *migrationResult) {
	if result.stopIndex < s.state.LatestMilestoneIndex {
		panic("invalid stop index")
	}
	// the result increases the latest milestone index
	if result.stopIndex != s.state.LatestMilestoneIndex {
		s.state.LatestMilestoneIndex = result.stopIndex
		s.state.LatestIncludedIndex = 0
	}
	s.state.LatestIncludedIndex += uint32(len(result.migratedFunds))
}

func createReceipt(migratedAt uint32, final bool, funds []*iota.MigratedFundsEntry) *iota.Receipt {
	// never create an empty receipt
	if len(funds) == 0 {
		return nil
	}
	receipt := &iota.Receipt{
		MigratedAt: migratedAt,
		Final:      final,
		Funds:      make([]iota.Serializable, len(funds)),
	}
	for i := range funds {
		receipt.Funds[i] = funds[i]
	}
	return receipt
}
