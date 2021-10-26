package referendum

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/syncutils"
	iotago "github.com/iotaledger/iota.go/v2"
)

// Events are the events issued by the ReferendumManager.
type Events struct {
	// SoftError is triggered when a soft error is encountered.
	SoftError *events.Event
}

var (
	ErrReferendumAlreadyStarted = errors.New("the given referendum already started")
	ErrReferendumAlreadyEnded   = errors.New("the given referendum already ended")
)

// ReferendumManager is used to track the outcome of referendums in the tangle.
type ReferendumManager struct {
	syncutils.Mutex

	// used to access the node storage.
	storage *storage.Storage
	// holds the ReferendumManager options.
	opts *Options

	//TODO: add health check when the db split is merged
	referendumStore kvstore.KVStore

	// events of the ReferendumManager.
	Events *Events
}

// the default options applied to the ReferendumManager.
var defaultOptions = []Option{
	WithIndexationMessage("IOTAVOTE"),
}

// Options define options for the ReferendumManager.
type Options struct {
	logger *logger.Logger

	indexationMessage []byte
}

// applies the given Option.
func (so *Options) apply(opts ...Option) {
	for _, opt := range opts {
		opt(so)
	}
}

// WithLogger enables logging within the faucet.
func WithLogger(logger *logger.Logger) Option {
	return func(opts *Options) {
		opts.logger = logger
	}
}

// WithIndexationMessage defines the ReferendumManager indexation payload to track.
func WithIndexationMessage(indexationMessage string) Option {
	return func(opts *Options) {
		opts.indexationMessage = []byte(indexationMessage)
	}
}

// Option is a function setting a faucet option.
type Option func(opts *Options)

// NewManager creates a new ReferendumManager instance.
func NewManager(
	storage *storage.Storage,
	referendumStore kvstore.KVStore,
	opts ...Option) *ReferendumManager {

	options := &Options{}
	options.apply(defaultOptions...)
	options.apply(opts...)

	manager := &ReferendumManager{
		storage:         storage,
		referendumStore: referendumStore,
		opts:            options,

		Events: &Events{
			SoftError: events.NewEvent(events.ErrorCaller),
		},
	}
	manager.init()

	return manager
}

func (rm *ReferendumManager) init() {
}

func (rm *ReferendumManager) CloseDatabase() error {
	var flushAndCloseError error
	if err := rm.referendumStore.Flush(); err != nil {
		flushAndCloseError = err
	}
	if err := rm.referendumStore.Close(); err != nil {
		flushAndCloseError = err
	}
	return flushAndCloseError
}

func (rm *ReferendumManager) Referendums() ([]*Referendum, error) {
	return []*Referendum{}, nil
}

func (rm *ReferendumManager) IsAnyReferendumAcceptingVotes(index milestone.Index) bool {

	referendums, err := rm.ReferendumsAcceptingVotes(index)
	if err != nil {
		return false
	}
	return len(referendums) > 0
}

func (rm *ReferendumManager) ReferendumsAcceptingVotes(index milestone.Index) ([]*Referendum, error) {

	referendums, err := rm.Referendums()
	if err != nil {
		return nil, err
	}

	var filtered []*Referendum
	for _, referendum := range referendums {
		if referendumIsAcceptingVotes(referendum, index) {
			filtered = append(filtered, referendum)
		}
	}

	return filtered, nil
}

func (rm *ReferendumManager) ReferendumsCountingVotes(index milestone.Index) ([]*Referendum, error) {
	referendums, err := rm.Referendums()
	if err != nil {
		return nil, err
	}

	var filtered []*Referendum
	for _, referendum := range referendums {
		if referendumIsCountingVotes(referendum, index) {
			filtered = append(filtered, referendum)
		}
	}

	return filtered, nil
}

// StoreReferendum accepts a new Referendum the manager should track.
// The current confirmed milestone index needs to be provided, so that the manager can check if the referendum can be added.
func (rm *ReferendumManager) StoreReferendum(referendum *Referendum, confirmedMilestoneIndex milestone.Index) (ReferendumID, error) {

	if confirmedMilestoneIndex >= referendum.MilestoneEnd {
		return NullReferendumID, ErrReferendumAlreadyEnded
	}

	if confirmedMilestoneIndex >= referendum.MilestoneStart {
		return NullReferendumID, ErrReferendumAlreadyStarted
	}

	return referendum.ID()
}

func (rm *ReferendumManager) Referendum(referendumID ReferendumID) (*Referendum, error) {
	return &Referendum{}, nil
}

func (rm *ReferendumManager) DeleteReferendum(referendumID ReferendumID) error {
	return nil
}

// logSoftError logs a soft error and triggers the event.
func (rm *ReferendumManager) logSoftError(err error) {
	if rm.opts.logger != nil {
		rm.opts.logger.Warn(err)
	}
	rm.Events.SoftError.Trigger(err)
}

func referendumIsAcceptingVotes(referendum *Referendum, index milestone.Index) bool {
	return index >= referendum.MilestoneStart && index <= referendum.MilestoneEnd
}

func referendumIsCountingVotes(referendum *Referendum, index milestone.Index) bool {
	return index >= referendum.MilestoneStartHolding && index <= referendum.MilestoneEnd
}

// ApplyNewUTXO checks if the new UTXO is part of a voting transaction.
// The following rules must be satisfied:
// 	- Must be a value transaction
// 	- Inputs must all come from the same address. Multiple inputs are allowed.
// 	- Has a singular output going to the same address as all input addresses.
// 	- Output Type 0 (SigLockedSingleOutput) and Type 1 (SigLockedDustAllowanceOutput) are both valid for this.
// 	- The Indexation must match the configured Indexation.
//  - The vote data must be parseable.
func (rm *ReferendumManager) ApplyNewUTXO(index milestone.Index, newOutput *utxo.Output) error {

	// No active referendum running, so no work to be done
	if !rm.IsAnyReferendumAcceptingVotes(index) {
		return nil
	}

	messageID := newOutput.MessageID()

	cachedMsg := rm.storage.CachedMessageOrNil(messageID)
	if cachedMsg == nil {
		// if the message was included, there must be a message
		return fmt.Errorf("message not found: %s", messageID.ToHex())
	}
	defer cachedMsg.Release(true)

	msg := cachedMsg.Message()

	transaction := msg.Transaction()
	if transaction == nil {
		// if the message was included, there must be a transaction payload
		return fmt.Errorf("no transaction payload found: MsgID: %s", messageID.ToHex())
	}

	txEssence := msg.TransactionEssence()
	if txEssence == nil {
		// if the message was included, there must be a transaction payload essence
		return fmt.Errorf("no transaction transactionEssence found: MsgID: %s", messageID.ToHex())
	}

	txEssenceIndexation := msg.TransactionEssenceIndexation()
	if txEssenceIndexation == nil {
		// no need to check if there is not indexation payload
		return nil
	}

	// the index of the transaction payload must match our configured indexation
	if !bytes.Equal(txEssenceIndexation.Index, rm.opts.indexationMessage) {
		return nil
	}

	// collect inputs
	inputOutputs := utxo.Outputs{}
	for _, input := range msg.TransactionEssenceUTXOInputs() {
		output, err := rm.storage.UTXOManager().ReadOutputByOutputIDWithoutLocking(input)
		if err != nil {
			return err
		}
		inputOutputs = append(inputOutputs, output)
	}

	// collect outputs
	depositOutputs := utxo.Outputs{}
	for i := 0; i < len(txEssence.Outputs); i++ {
		output, err := utxo.NewOutput(messageID, transaction, uint16(i))
		if err != nil {
			return err
		}
		depositOutputs = append(depositOutputs, output)
	}

	// only a single output is allowed
	if len(depositOutputs) != 1 {
		return nil
	}

	// only OutputSigLockedSingleOutput and OutputSigLockedDustAllowanceOutput are allowed as output type
	switch depositOutputs[0].OutputType() {
	case iotago.OutputSigLockedDustAllowanceOutput:
	case iotago.OutputSigLockedSingleOutput:
	default:
		return nil
	}

	outputAddress, err := depositOutputs[0].Address().Serialize(iotago.DeSeriModeNoValidation)
	if err != nil {
		return nil
	}

	// check if all inputs come from the same address as the output
	for _, input := range inputOutputs {
		inputAddress, err := input.Address().Serialize(iotago.DeSeriModeNoValidation)
		if err != nil {
			return nil
		}

		if !bytes.Equal(outputAddress, inputAddress) {
			// input address does not match the output address =>  not a voting transaction
			return nil
		}
	}

	votes, err := votesFromIndexation(txEssenceIndexation)
	if err != nil {
		return err
	}

	validVotes := rm.validVotes(index, votes)

	if len(validVotes) == 0 {
		// No votes for anything we are tracking
		return nil
	}

	mutations := rm.referendumStore.Batched()

	//Store the message holding the vote
	if err := rm.storeMessage(msg, mutations); err != nil {
		mutations.Cancel()
		return err
	}

	// Store the vote started at this milestone
	if err := rm.startVoteAtMilestone(depositOutputs[0], index, mutations); err != nil {
		mutations.Cancel()
		return err
	}

	// Count the new votes by increasing the current vote balance
	for _, vote := range validVotes {
		if err := rm.countCurrentVote(depositOutputs[0], vote, true, mutations); err != nil {
			mutations.Cancel()
			return err
		}
	}

	return mutations.Commit()
}

func (rm *ReferendumManager) ApplySpentUTXO(index milestone.Index, spent *utxo.Spent) error {

	// No active referendum running, so no work to be done
	if !rm.IsAnyReferendumAcceptingVotes(index) {
		return nil
	}

	// Check if we tracked the vote initially, e.g. saved the Message that created this UTXO
	msg, err := rm.messageForMessageID(spent.MessageID())
	if err != nil {
		return err
	}

	if msg == nil {
		// This UTXO had no valid vote, so we did not store the message for it
		return nil
	}

	txEssenceIndexation := msg.TransactionEssenceIndexation()
	if txEssenceIndexation == nil {
		// We tracked this vote before, and now we don't have its indexation, so something happened
		return ErrInvalidPreviouslyTrackedVote
	}

	votes, err := votesFromIndexation(txEssenceIndexation)
	if err != nil {
		return err
	}

	validVotes := rm.validVotes(index, votes)

	if len(validVotes) == 0 {
		// We were previously tracking this vote, but now there are no valid votes, so something happened
		return ErrInvalidPreviouslyTrackedVote
	}

	mutations := rm.referendumStore.Batched()

	// Store the vote ended at this milestone
	if err := rm.endVoteAtMilestone(spent.Output(), index, mutations); err != nil {
		mutations.Cancel()
		return err
	}

	// Count the spent votes by decreasing the current vote balance
	for _, vote := range validVotes {
		if err := rm.countCurrentVote(spent.Output(), vote, false, mutations); err != nil {
			mutations.Cancel()
			return err
		}
	}

	return mutations.Commit()
}

// ApplyNewConfirmedMilestoneIndex iterates over each counting referendum and applies the current vote for each question to the total vote
func (rm *ReferendumManager) ApplyNewConfirmedMilestoneIndex(index milestone.Index) error {

	countingReferendums, err := rm.ReferendumsCountingVotes(index)
	if err != nil {
		return err
	}

	// No counting referendum, so no work to be done
	if len(countingReferendums) == 0 {
		return nil
	}

	mutations := rm.referendumStore.Batched()

	// Iterate over all known referendums that are currently counting
	for _, referendum := range countingReferendums {

		referendumID, err := referendum.ID()
		if err != nil {
			mutations.Cancel()
			return err
		}

		// For each referendum, iterate over all questions
		for idx, value := range referendum.Questions {
			questionIndex := uint8(idx)
			question := value.(*Question) // force cast here since we are sure the stored Referendum is valid

			// For each question, iterate over all answers. Include 0 here, since that is valid, i.e. answer skipped by voter
			for idx := 0; idx <= len(question.Answers); idx++ {
				answerIndex := uint8(idx)

				totalBalance, err := rm.TotalBalanceForReferendum(referendumID, questionIndex, answerIndex)
				if err != nil {
					mutations.Cancel()
					return err
				}

				currentBalance, err := rm.CurrentBalanceForReferendum(referendumID, questionIndex, answerIndex)
				if err != nil {
					mutations.Cancel()
					return err
				}

				// Add current vote balance to total vote balance for each answer
				newTotal := totalBalance + currentBalance

				if err := setTotalBalanceForReferendum(referendumID, questionIndex, answerIndex, newTotal, mutations); err != nil {
					mutations.Cancel()
					return err
				}
			}
		}
	}

	return mutations.Commit()
}

func (rm *ReferendumManager) validVotes(index milestone.Index, votes []*Vote) []*Vote {

	var validVotes []*Vote
	for _, vote := range votes {

		// Check that we have the referendum vor the given vote
		referendum, err := rm.Referendum(vote.ReferendumID)
		if err != nil {
			continue
		}

		// Check that the referendum is accepting votes
		if !referendumIsAcceptingVotes(referendum, index) {
			continue
		}

		// Check that the amount of answers equals the questions in the referendum
		if len(vote.Answers) != len(referendum.Questions) {
			continue
		}

		//TODO: validate answers? We would create a current vote for invalid answers, but only count valid answers and skipped (index == 0) anyway

		validVotes = append(validVotes, vote)
	}

	return validVotes
}

func votesFromIndexation(indexation *iotago.Indexation) ([]*Vote, error) {

	// try to parse the votes payload
	parsedVotes := &Votes{}
	if _, err := parsedVotes.Deserialize(indexation.Data, iotago.DeSeriModePerformValidation); err != nil {
		// votes payload can't be parsed => ignore votes
		return nil, fmt.Errorf("no valid votes payload")
	}

	var votes []*Vote
	for _, vote := range parsedVotes.Votes {
		votes = append(votes, vote.(*Vote))
	}

	return votes, nil
}
