package migrator

import (
	"fmt"
	"os"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/syncutils"
	iotago "github.com/iotaledger/iota.go/v2"
)

const (
	// SensibleMaxEntriesCount defines an amount of entries within receipts which allows a milestone with 8 parents and 2 sigs/pub keys
	// to fly under the next pow requirement step.
	SensibleMaxEntriesCount = 110
)

var (
	// ErrStateFileAlreadyExists is returned when a new state is tried to be initialized but a state file already exists.
	ErrStateFileAlreadyExists = errors.New("migrator state file already exists")
	// ErrInvalidState is returned when the content of the state file is invalid.
	ErrInvalidState = errors.New("invalid migrator state")
)

// MigratorServiceEvents are events happening around a MigratorService.
type MigratorServiceEvents struct {
	// SoftError is triggered when a soft error is encountered.
	SoftError *events.Event
	// MigratedFundsFetched is triggered when new migration funds were fetched from a legacy node.
	MigratedFundsFetched *events.Event
}

// MigratedFundsCaller is an event caller which gets migrated funds passed.
func MigratedFundsCaller(handler interface{}, params ...interface{}) {
	handler.(func([]*iotago.MigratedFundsEntry))(params[0].([]*iotago.MigratedFundsEntry))
}

// Queryer defines the interface used to query the migrated funds.
type Queryer interface {
	QueryMigratedFunds(uint32) ([]*iotago.MigratedFundsEntry, error)
	QueryNextMigratedFunds(uint32) (uint32, []*iotago.MigratedFundsEntry, error)
}

// MigratorService is a service querying and validating batches of migrated funds.
type MigratorService struct {
	Events *MigratorServiceEvents

	queryer Queryer
	state   State

	mutex      syncutils.Mutex
	migrations chan *migrationResult

	stateFilePath     string
	receiptMaxEntries int
}

// State stores the latest state of the MigratorService.
type State struct {
	/*
	   4 bytes uint32 			LatestMigratedAtIndex
	   4 bytes uint32 			LatestIncludedIndex
	*/
	LatestMigratedAtIndex uint32
	LatestIncludedIndex   uint32
	SendingReceipt        bool
}

type migrationResult struct {
	stopIndex     uint32
	lastBatch     bool
	migratedFunds []*iotago.MigratedFundsEntry
}

// NewService creates a new MigratorService.
func NewService(queryer Queryer, stateFilePath string, receiptMaxEntries int) *MigratorService {
	return &MigratorService{
		Events: &MigratorServiceEvents{
			SoftError:            events.NewEvent(events.ErrorCaller),
			MigratedFundsFetched: events.NewEvent(MigratedFundsCaller),
		},
		queryer:           queryer,
		migrations:        make(chan *migrationResult),
		receiptMaxEntries: receiptMaxEntries,
		stateFilePath:     stateFilePath,
	}
}

// Receipt returns the next receipt of migrated funds.
// Each receipt can only consists of migrations confirmed by one milestone, it will never be larger than MaxMigratedFundsEntryCount.
// Receipt returns nil, if there are currently no new migrations available. Although the actual API calls and
// validations happen in the background, Receipt might block until the next receipt is ready.
// When s is stopped, Receipt will always return nil.
func (s *MigratorService) Receipt() *iotago.Receipt {
	// make the channel receive and the state update atomic, so that the state always matches the result
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// non-blocking receive; return nil if the channel is closed or value available
	var result *migrationResult
	select {
	case result = <-s.migrations:
	default:
	}
	if result == nil {
		return nil
	}
	s.updateState(result)
	return createReceipt(result.stopIndex, result.lastBatch, result.migratedFunds)
}

// PersistState persists the current state to a file.
// PersistState must be called when the receipt returned by the last call of Receipt has been send to the network.
func (s *MigratorService) PersistState(sendingReceipt bool) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.state.SendingReceipt = sendingReceipt
	return utils.WriteToFile(s.stateFilePath, &s.state, 0660)
}

// InitState initializes the state of s.
// If msIndex is not nil, s is bootstrapped using that index as its initial state,
// otherwise the state is loaded from file.
// The optional utxoManager is used to validate the initialized state against the DB.
// InitState must be called before Start.
func (s *MigratorService) InitState(msIndex *uint32, utxoManager *utxo.Manager) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	var state State
	if msIndex == nil {
		// restore state from file
		if err := utils.ReadFromFile(s.stateFilePath, &state); err != nil {
			return fmt.Errorf("failed to load state file: %w", err)
		}
	} else {
		// for bootstrapping the state file must not exist
		if _, err := os.Stat(s.stateFilePath); !os.IsNotExist(err) {
			return ErrStateFileAlreadyExists
		}
		state = State{
			LatestMigratedAtIndex: *msIndex,
			LatestIncludedIndex:   0,
		}
	}

	if state.SendingReceipt {
		return fmt.Errorf("%w: 'sending receipt' flag is set which means the node didn't shutdown correctly", ErrInvalidState)
	}

	// validate the state
	if state.LatestMigratedAtIndex == 0 {
		return fmt.Errorf("%w: latest migrated at index must not be zero", ErrInvalidState)
	}

	if utxoManager != nil {
		highestMigratedAtIndex, err := utxoManager.SearchHighestReceiptMigratedAtIndex()
		if err != nil {
			return fmt.Errorf("unable to determine highest migrated at index: %w", err)
		}
		// if highestMigratedAtIndex is zero no receipt in the DB, so we cannot do sanity checks
		if highestMigratedAtIndex > 0 && highestMigratedAtIndex != state.LatestMigratedAtIndex {
			return fmt.Errorf("state receipt does not match highest receipt in database: state: %d, database: %d",
				state.LatestMigratedAtIndex, highestMigratedAtIndex)
		}
	}

	s.state = state
	return nil
}

// OnServiceErrorFunc is a function which is called when the service encounters an
// error which prevents it from functioning properly.
// Returning false from the error handler tells the service to terminate.
type OnServiceErrorFunc func(err error) (terminate bool)

// Start stats the MigratorService s, it stops when shutdownSignal is closed.
func (s *MigratorService) Start(shutdownSignal <-chan struct{}, onError OnServiceErrorFunc) {
	var startIndex uint32
	for {
		msIndex, migratedFunds, err := s.nextMigrations(startIndex)
		if err != nil {
			if onError != nil && !onError(err) {
				close(s.migrations)
				return
			}
			continue
		}

		s.Events.MigratedFundsFetched.Trigger(migratedFunds)

		// always continue with the next index
		startIndex = msIndex + 1

		for {
			batch := migratedFunds
			lastBatch := true
			if len(batch) > s.receiptMaxEntries {
				batch = batch[:s.receiptMaxEntries]
				lastBatch = false
			}
			select {
			case s.migrations <- &migrationResult{msIndex, lastBatch, batch}:
			case <-shutdownSignal:
				close(s.migrations)
				return
			}
			migratedFunds = migratedFunds[len(batch):]
			if len(migratedFunds) == 0 {
				break
			}
		}
	}
}

// stateMigrations queries the next existing migrations after the current state.
// It returns an empty slice, if the state corresponded to the last migration index of that milestone.
// It returns an error if the current state contains an included migration index that is too large.
func (s *MigratorService) stateMigrations() (uint32, []*iotago.MigratedFundsEntry, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	migratedFunds, err := s.queryer.QueryMigratedFunds(s.state.LatestMigratedAtIndex)
	if err != nil {
		return 0, nil, err
	}
	l := uint32(len(migratedFunds))
	if l >= s.state.LatestIncludedIndex {
		return s.state.LatestMigratedAtIndex, migratedFunds[s.state.LatestIncludedIndex:], nil
	}
	return 0, nil, common.CriticalError(fmt.Errorf("%w: state at index %d but only %d migrations", ErrInvalidState, s.state.LatestIncludedIndex, l))
}

// nextMigrations queries the next existing migrations starting from milestone index startIndex.
// If startIndex is 0 the indices from state are used.
func (s *MigratorService) nextMigrations(startIndex uint32) (uint32, []*iotago.MigratedFundsEntry, error) {
	if startIndex == 0 {
		// for bootstrapping query the migrations corresponding to the state
		msIndex, migratedFunds, err := s.stateMigrations()
		if err != nil {
			return 0, nil, fmt.Errorf("failed to query migrations corresponding to initial state: %w", err)
		}
		// return remaining migrations
		if len(migratedFunds) > 0 {
			return msIndex, migratedFunds, nil
		}
		// otherwise query the next available migrations
		startIndex = msIndex + 1
	}
	return s.queryer.QueryNextMigratedFunds(startIndex)
}

func (s *MigratorService) updateState(result *migrationResult) {
	if result.stopIndex < s.state.LatestMigratedAtIndex {
		panic("invalid stop index")
	}
	// the result increases the latest milestone index
	if result.stopIndex != s.state.LatestMigratedAtIndex {
		s.state.LatestMigratedAtIndex = result.stopIndex
		s.state.LatestIncludedIndex = 0
	}
	s.state.LatestIncludedIndex += uint32(len(result.migratedFunds))
}

func createReceipt(migratedAt uint32, final bool, funds []*iotago.MigratedFundsEntry) *iotago.Receipt {
	// never create an empty receipt
	if len(funds) == 0 {
		return nil
	}
	receipt := &iotago.Receipt{
		MigratedAt: migratedAt,
		Final:      final,
		Funds:      make([]iotago.Serializable, len(funds)),
	}
	for i := range funds {
		receipt.Funds[i] = funds[i]
	}
	return receipt
}
