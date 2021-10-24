package referendum

import (
	"bytes"
	"fmt"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/syncutils"
	iotago "github.com/iotaledger/iota.go/v2"
)

// Events are the events issued by the ReferendumManager.
type Events struct {
	// SoftError is triggered when a soft error is encountered.
	SoftError *events.Event
}

// ReferendumManager is used to track the outcome of referendas in the tangle.
type ReferendumManager struct {
	syncutils.Mutex

	// used to access the node storage.
	storage *storage.Storage
	// holds the ReferendumManager options.
	opts *Options

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
	opts ...Option) *ReferendumManager {

	options := &Options{}
	options.apply(defaultOptions...)
	options.apply(opts...)

	manager := &ReferendumManager{
		storage: storage,
		opts:    options,

		Events: &Events{
			SoftError: events.NewEvent(events.ErrorCaller),
		},
	}
	manager.init()

	return manager
}

func (rm *ReferendumManager) init() {
}

func (rm *ReferendumManager) Referenda() (*ReferendaResponse, error) {
	return &ReferendaResponse{}, nil
}

func (rm *ReferendumManager) CreateReferendum() (*CreateReferendumResponse, error) {
	return &CreateReferendumResponse{}, nil
}

func (rm *ReferendumManager) Referendum(referendumID hornet.MessageID) (*ReferendumResponse, error) {
	return &ReferendumResponse{}, nil
}

func (rm *ReferendumManager) DeleteReferendum(referendumID hornet.MessageID) error {
	return nil
}

func (rm *ReferendumManager) ReferendumQuestions(referendumID hornet.MessageID) (*ReferendumQuestionsResponse, error) {
	return &ReferendumQuestionsResponse{}, nil
}

func (rm *ReferendumManager) ReferendumQuestion(referendumID hornet.MessageID, questionIndex int) (*ReferendumQuestionResponse, error) {
	return &ReferendumQuestionResponse{}, nil
}

func (rm *ReferendumManager) ReferendumStatus(referendumID hornet.MessageID) (*ReferendumStatusResponse, error) {
	return &ReferendumStatusResponse{}, nil
}

func (rm *ReferendumManager) ReferendumQuestionStatus(referendumID hornet.MessageID, questionIndex int) (*ReferendumQuestionStatusResponse, error) {
	return &ReferendumQuestionStatusResponse{}, nil
}

// logSoftError logs a soft error and triggers the event.
func (rm *ReferendumManager) logSoftError(err error) {
	if rm.opts.logger != nil {
		rm.opts.logger.Warn(err)
	}
	rm.Events.SoftError.Trigger(err)
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

	// try to parse the vote payload
	v := &Vote{}
	if _, err := v.Deserialize(txEssenceIndexation.Data, iotago.DeSeriModePerformValidation); err != nil {
		// vote payload can't be parsed => ignore vote
		return nil
	}

	// TODO:
	// do something with the vote :)

	return nil
}

func (rm *ReferendumManager) ApplySpentUTXO(index milestone.Index, spent *utxo.Spent) error {

	// TODO:
	// check if we were tracking that UTXO => do something

	return nil
}

func (rm *ReferendumManager) ApplyNewConfirmedMilestoneIndex(index milestone.Index) error {

	// TODO:
	// Do all the fancy vote balance calculations

	return nil
}
