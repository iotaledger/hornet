package faucet

import (
	"bytes"
	"fmt"
	"runtime"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/restapi"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/syncutils"
	iotago "github.com/iotaledger/iota.go/v2"
)

// SendMessageFunc is a function which sends a message to the network.
type SendMessageFunc = func(msg *storage.Message) error

// TipselFunc selects tips for the faucet.
type TipselFunc = func() (tips hornet.MessageIDs, err error)

var (
	// ErrNoTipsGiven is returned when no tips were given to issue a message.
	ErrNoTipsGiven = errors.New("no tips given")
)

// Events are the events issued by the faucet.
type Events struct {
	// Fired when a faucet message is issued.
	IssuedMessage *events.Event
	// SoftError is triggered when a soft error is encountered.
	SoftError *events.Event
}

// queueItem is an item for the faucet requests queue.
type queueItem struct {
	Bech32         string
	Amount         uint64
	Ed25519Address *iotago.Ed25519Address
}

// FaucetInfoResponse defines the response of a GET RouteFaucetInfo REST API call.
type FaucetInfoResponse struct {
	// The bech32 address of the faucet.
	Address string `json:"address"`
	// The remaining balance of faucet.
	Balance uint64 `json:"balance"`
}

// FaucetEnqueueResponse defines the response of a POST RouteFaucetEnqueue REST API call.
type FaucetEnqueueResponse struct {
	// The bech32 address.
	Address string `json:"address"`
	// The number of waiting requests in the queue.
	WaitingRequests int `json:"waitingRequests"`
}

// Faucet is used to issue transaction to users that requested funds via a REST endpoint.
type Faucet struct {
	syncutils.Mutex

	// used to access the node storage.
	storage *storage.Storage
	// id of the network the faucet is running in.
	networkID uint64
	// belowMaxDepth is the maximum allowed delta
	// value between OCRI of a given message in relation to the current CMI before it gets lazy.
	belowMaxDepth milestone.Index
	// used to get the outputs.
	utxoManager *utxo.Manager
	// the address of the faucet.
	address *iotago.Ed25519Address
	// used to sign the faucet transactions.
	addressSigner iotago.AddressSigner
	// used to get valid tips for new faucet messages.
	tipselFunc TipselFunc
	// used to do the PoW for the faucet messages.
	powHandler *pow.Handler
	// the function used to send a message.
	sendMessageFunc SendMessageFunc
	// holds the faucet options.
	opts *Options

	// events of the faucet.
	Events *Events

	// the message ID of the last sent faucet message.
	lastMessageID hornet.MessageID
	// map with all queued requests per address.
	queueMap map[string]*queueItem
	// queue of new requests.
	queue chan *queueItem
}

// the default options applied to the faucet.
var defaultOptions = []Option{
	WithHRPNetworkPrefix(iotago.PrefixTestnet),
	WithAmount(10000000),            // 10 Mi
	WithSmallAmount(1000000),        // 1 Mi
	WithMaxAddressBalance(20000000), // 20 Mi
	WithMaxOutputCount(iotago.MaxOutputsCount),
	WithIndexationMessage("HORNET FAUCET"),
	WithBatchTimeout(2 * time.Second),
	WithPowWorkerCount(0),
}

// Options define options for the faucet.
type Options struct {
	logger *logger.Logger

	hrpNetworkPrefix  iotago.NetworkPrefix
	amount            uint64
	smallAmount       uint64
	maxAddressBalance uint64
	maxOutputCount    int
	indexationMessage []byte
	batchTimeout      time.Duration
	powWorkerCount    int
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

// WithHRPNetworkPrefix sets the bech32 HRP network prefix.
func WithHRPNetworkPrefix(networkPrefix iotago.NetworkPrefix) Option {
	return func(opts *Options) {
		opts.hrpNetworkPrefix = networkPrefix
	}
}

// WithAmount defines the amount of funds the requester receives.
func WithAmount(amount uint64) Option {
	return func(opts *Options) {
		opts.amount = amount
	}
}

// WithSmallAmount defines the amount of funds the requester receives
// if the target address has more funds than the faucet amount and less than maximum.
func WithSmallAmount(smallAmount uint64) Option {
	return func(opts *Options) {
		opts.smallAmount = smallAmount
	}
}

// WithMaxAddressBalance defines the maximum allowed amount of funds on the target address.
// If there are more funds already, the faucet request is rejected.
func WithMaxAddressBalance(maxAddressBalance uint64) Option {
	return func(opts *Options) {
		opts.maxAddressBalance = maxAddressBalance
	}
}

// WithMaxOutputCount defines the maximum output count per faucet message.
func WithMaxOutputCount(maxOutputCount int) Option {
	return func(opts *Options) {
		if maxOutputCount > iotago.MaxOutputsCount {
			maxOutputCount = iotago.MaxOutputsCount
		}
		opts.maxOutputCount = maxOutputCount
	}
}

// WithIndexationMessage defines the faucet transaction indexation payload.
func WithIndexationMessage(indexationMessage string) Option {
	return func(opts *Options) {
		opts.indexationMessage = []byte(indexationMessage)
	}
}

// WithBatchTimeout sets the maximum duration for collecting faucet batches.
func WithBatchTimeout(timeout time.Duration) Option {
	return func(opts *Options) {
		opts.batchTimeout = timeout
	}
}

// WithPowWorkerCount defines the amount of workers used for calculating PoW when issuing faucet messages.
func WithPowWorkerCount(powWorkerCount int) Option {

	if powWorkerCount == 0 {
		powWorkerCount = runtime.NumCPU() - 1
	}

	if powWorkerCount < 1 {
		powWorkerCount = 1
	}

	return func(opts *Options) {
		opts.powWorkerCount = powWorkerCount
	}
}

// Option is a function setting a faucet option.
type Option func(opts *Options)

// New creates a new faucet instance.
func New(
	storage *storage.Storage,
	networkID uint64,
	belowMaxDepth int,
	utxoManager *utxo.Manager,
	address *iotago.Ed25519Address,
	addressSigner iotago.AddressSigner,
	tipselFunc TipselFunc,
	powHandler *pow.Handler,
	sendMessageFunc SendMessageFunc,
	opts ...Option) *Faucet {

	options := &Options{}
	options.apply(defaultOptions...)
	options.apply(opts...)

	faucet := &Faucet{
		storage:         storage,
		networkID:       networkID,
		belowMaxDepth:   milestone.Index(belowMaxDepth),
		utxoManager:     utxoManager,
		address:         address,
		addressSigner:   addressSigner,
		tipselFunc:      tipselFunc,
		powHandler:      powHandler,
		sendMessageFunc: sendMessageFunc,
		opts:            options,

		Events: &Events{
			IssuedMessage: events.NewEvent(events.VoidCaller),
			SoftError:     events.NewEvent(events.ErrorCaller),
		},
	}
	faucet.init()

	return faucet
}

func (f *Faucet) init() {
	f.queue = make(chan *queueItem, 5000)
	f.queueMap = make(map[string]*queueItem)
	f.lastMessageID = hornet.NullMessageID()
}

// NetworkPrefix returns the used network prefix.
func (f *Faucet) NetworkPrefix() iotago.NetworkPrefix {
	return f.opts.hrpNetworkPrefix
}

// Info returns the used faucet address and remaining balance.
func (f *Faucet) Info() (*FaucetInfoResponse, error) {
	balance, _, err := f.utxoManager.AddressBalanceWithoutLocking(f.address)
	if err != nil {
		return nil, err
	}

	return &FaucetInfoResponse{
		Address: f.address.Bech32(f.opts.hrpNetworkPrefix),
		Balance: balance,
	}, nil
}

// Enqueue adds a new faucet request to the queue.
func (f *Faucet) Enqueue(bech32 string, ed25519Addr *iotago.Ed25519Address) (*FaucetEnqueueResponse, error) {
	f.Lock()
	defer f.Unlock()

	if _, exists := f.queueMap[bech32]; exists {
		return nil, errors.WithMessage(restapi.ErrInvalidParameter, "Address is already in the queue.")
	}

	amount := f.opts.amount
	balance, _, err := f.utxoManager.AddressBalanceWithoutLocking(ed25519Addr)
	if err == nil && balance >= f.opts.amount {
		amount = f.opts.smallAmount

		if balance >= f.opts.maxAddressBalance {
			return nil, errors.WithMessage(restapi.ErrInvalidParameter, "You already have enough coins on your address.")
		}
	}

	request := &queueItem{
		Bech32:         bech32,
		Amount:         amount,
		Ed25519Address: ed25519Addr,
	}

	select {
	case f.queue <- request:
		f.queueMap[bech32] = request
		return &FaucetEnqueueResponse{
			Address:         bech32,
			WaitingRequests: len(f.queueMap),
		}, nil

	default:
		// queue is full
		return nil, errors.WithMessage(echo.ErrInternalServerError, "Faucet queue is full. Please try again later!")
	}
}

// clearRequests clears the old requests from the map.
// this is necessary to be able to send new requests to the same addresses.
func (f *Faucet) clearRequests(batchedRequests []*queueItem) {
	f.Lock()
	defer f.Unlock()

	for _, request := range batchedRequests {
		delete(f.queueMap, request.Bech32)
	}
}

// createMessage creates a new message and references the last faucet message (also reattaches if below max depth).
func (f *Faucet) createMessage(txPayload iotago.Serializable, shutdownSignal <-chan struct{}) (*storage.Message, error) {

	tips, err := f.tipselFunc()
	if err != nil {
		return nil, err
	}

	reattachMessage := func(messageID hornet.MessageID) (*storage.Message, error) {
		cachedMsg := f.storage.CachedMessageOrNil(f.lastMessageID)
		if cachedMsg == nil {
			// message unknown
			return nil, fmt.Errorf("message not found: %s", messageID.ToHex())
		}
		defer cachedMsg.Release(true)

		tips, err := f.tipselFunc()
		if err != nil {
			return nil, err
		}

		iotaMsg := &iotago.Message{
			NetworkID: f.networkID,
			Parents:   tips.ToSliceOfArrays(),
			Payload:   cachedMsg.Message().Message().Payload,
		}

		if err := f.powHandler.DoPoW(iotaMsg, shutdownSignal, 1); err != nil {
			return nil, err
		}

		msg, err := storage.NewMessage(iotaMsg, iotago.DeSeriModePerformValidation)
		if err != nil {
			return nil, err
		}

		if err := f.sendMessageFunc(msg); err != nil {
			return nil, err
		}

		return msg, nil
	}

	// check if the last faucet message was already confirmed.
	// if not, check if it is already below max depth and reattach in case.
	// we need to check for the last faucet message, because we reference the last message as a tip
	// to be sure the tangle consumes our UTXOs in the correct order.
	if err = func() error {
		if bytes.Equal(f.lastMessageID, hornet.NullMessageID()) {
			// do not reference NullMessage
			return nil
		}

		cachedMsgMeta := f.storage.CachedMessageMetadataOrNil(f.lastMessageID)
		if cachedMsgMeta == nil {
			// message unknown
			return nil
		}
		defer cachedMsgMeta.Release(true)

		if cachedMsgMeta.Metadata().IsReferenced() {
			// message is already confirmed, no need to reference
			return nil
		}

		_, ocri := dag.ConeRootIndexes(f.storage, cachedMsgMeta.Retain(), f.storage.ConfirmedMilestoneIndex()) // meta +
		if (f.storage.LatestMilestoneIndex() - ocri) > f.belowMaxDepth {
			// the last faucet message is not confirmed yet, but it is already below max depth
			// we need to reattach it
			msg, err := reattachMessage(f.lastMessageID)
			if err != nil {
				return common.CriticalError(fmt.Errorf("faucet message was below max depth and couldn't be reattached: %w", err))
			}

			// update the lastMessasgeID because we reattached the message
			f.lastMessageID = msg.MessageID()
		}

		tips[0] = f.lastMessageID
		tips = tips.RemoveDupsAndSortByLexicalOrder()

		return nil
	}(); err != nil {
		return nil, err
	}

	// create the message
	iotaMsg := &iotago.Message{
		NetworkID: f.networkID,
		Parents:   tips.ToSliceOfArrays(),
		Payload:   txPayload,
	}

	if err := f.powHandler.DoPoW(iotaMsg, shutdownSignal, 1); err != nil {
		return nil, err
	}

	msg, err := storage.NewMessage(iotaMsg, iotago.DeSeriModePerformValidation)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

// buildTransactionPayload creates a signed transaction payload with all UTXO and batched requests.
func (f *Faucet) buildTransactionPayload(unspentOutputs []*utxo.Output, batchedRequests []*queueItem) (*iotago.Transaction, *iotago.UTXOInput, uint64, error) {

	txBuilder := iotago.NewTransactionBuilder()
	txBuilder.AddIndexationPayload(&iotago.Indexation{Index: f.opts.indexationMessage, Data: nil})

	outputCount := 0
	var remainderAmount int64 = 0

	// collect all unspent output of the faucet address
	for _, unspentOutput := range unspentOutputs {
		outputCount++
		remainderAmount += int64(unspentOutput.Amount())
		txBuilder.AddInput(&iotago.ToBeSignedUTXOInput{Address: f.address, Input: unspentOutput.UTXOInput()})
	}

	// add all requests as outputs
	for _, req := range batchedRequests {
		outputCount++

		if outputCount >= f.opts.maxOutputCount {
			// do not collect further requests
			// the last slot is for the remainder
			break
		}

		if remainderAmount == 0 {
			// do not collect further requests
			break
		}

		amount := req.Amount
		if remainderAmount < int64(amount) {
			// not enough funds left
			amount = uint64(remainderAmount)
		}
		remainderAmount -= int64(amount)

		txBuilder.AddOutput(&iotago.SigLockedSingleOutput{Address: req.Ed25519Address, Amount: uint64(amount)})
	}

	if remainderAmount > 0 {
		txBuilder.AddOutput(&iotago.SigLockedSingleOutput{Address: f.address, Amount: uint64(remainderAmount)})
	}

	txPayload, err := txBuilder.Build(f.addressSigner)
	if err != nil {
		return nil, nil, 0, err
	}

	if remainderAmount == 0 {
		// no remainder available
		return txPayload, nil, 0, nil
	}

	transactionID, err := txPayload.ID()
	if err != nil {
		return nil, nil, 0, fmt.Errorf("can't compute the transaction ID, error: %w", err)
	}

	remainderOutput := &iotago.UTXOInput{}
	copy(remainderOutput.TransactionID[:], transactionID[:iotago.TransactionIDLength])

	// search remainder address in the outputs
	found := false
	var outputIndex uint16 = 0
	for _, output := range txPayload.Essence.(*iotago.TransactionEssence).Outputs {
		sigLock := output.(*iotago.SigLockedSingleOutput)
		ed25519Addr := sigLock.Address.(*iotago.Ed25519Address)

		if bytes.Equal(ed25519Addr[:], f.address[:]) {
			// found the remainder address in the outputs
			found = true
			remainderOutput.TransactionOutputIndex = outputIndex
			break
		}
		outputIndex++
	}

	if !found {
		return nil, nil, 0, errors.New("can't find the faucet remainder output")
	}

	return txPayload, remainderOutput, uint64(remainderAmount), nil
}

// sendFaucetMessage creates a faucet transaction payload and remembers the last sent messageID.
func (f *Faucet) sendFaucetMessage(unspentOutputs []*utxo.Output, batchedRequests []*queueItem, shutdownSignal <-chan struct{}) (*utxo.Output, error) {

	txPayload, remainderIotaGoOutput, remainderAmount, err := f.buildTransactionPayload(unspentOutputs, batchedRequests)
	if err != nil {
		return nil, fmt.Errorf("build transaction payload failed, error: %w", err)
	}

	msg, err := f.createMessage(txPayload, shutdownSignal)
	if err != nil {
		return nil, fmt.Errorf("build faucet message failed, error: %w", err)
	}

	if err := f.sendMessageFunc(msg); err != nil {
		return nil, fmt.Errorf("send faucet message failed, error: %w", err)
	}

	f.lastMessageID = msg.MessageID()
	remainderIotaGoOutputID := remainderIotaGoOutput.ID()
	remainderOutput := utxo.CreateOutput(&remainderIotaGoOutputID, msg.MessageID(), iotago.OutputSigLockedSingleOutput, f.address, uint64(remainderAmount))

	return remainderOutput, nil
}

// logSoftError logs a soft error and triggers the event.
func (f *Faucet) logSoftError(err error) {
	if f.opts.logger != nil {
		f.opts.logger.Warn(err)
	}
	f.Events.SoftError.Trigger(err)
}

// RunFaucetLoop collects unspent outputs on the faucet address and batches the requests from the queue.
func (f *Faucet) RunFaucetLoop(shutdownSignal <-chan struct{}) error {

	var lastRemainderOutput *utxo.Output

	for {
		select {
		case <-shutdownSignal:
			// faucet was stopped
			return nil

		default:

			// only collect unspent outputs if the lastRemainderOutput is not pending
			shouldCollectUnspentOutputs := func() bool {
				if lastRemainderOutput == nil {
					return true
				}

				if _, err := f.utxoManager.ReadOutputByOutputIDWithoutLocking(lastRemainderOutput.OutputID()); err != nil {
					return false
				}

				return true
			}

			var err error
			unspentOutputs := []*utxo.Output{}
			if shouldCollectUnspentOutputs() {
				unspentOutputs, err = f.utxoManager.UnspentOutputs(utxo.FilterAddress(f.address), utxo.ReadLockLedger(false), utxo.MaxResultCount(f.opts.maxOutputCount-2), utxo.FilterOutputType(iotago.OutputSigLockedSingleOutput))
				if err != nil {
					return fmt.Errorf("reading unspent outputs failed: %s, error: %w", f.address.Bech32(f.opts.hrpNetworkPrefix), err)
				}
			} else {
				unspentOutputs = append(unspentOutputs, lastRemainderOutput)
			}

			var amount uint64 = 0
			found := false
			for _, unspentOutput := range unspentOutputs {
				amount += unspentOutput.Amount()
				if lastRemainderOutput != nil && bytes.Equal(unspentOutput.OutputID()[:], lastRemainderOutput.OutputID()[:]) {
					found = true
				}
			}

			if lastRemainderOutput != nil && !found {
				unspentOutputs = append(unspentOutputs, lastRemainderOutput)
				amount += lastRemainderOutput.Amount()
			}

			collectedRequestsCounter := len(unspentOutputs)
			batchWriterTimeoutChan := time.After(f.opts.batchTimeout)
			batchedRequests := []*queueItem{}

		CollectValues:
			for collectedRequestsCounter < f.opts.maxOutputCount-1 && amount > f.opts.amount {
				select {
				case <-shutdownSignal:
					// faucet was stopped
					return nil

				case <-batchWriterTimeoutChan:
					// timeout was reached => stop collecting requests
					break CollectValues

				case request := <-f.queue:
					batchedRequests = append(batchedRequests, request)
					collectedRequestsCounter++
					amount -= request.Amount
				}
			}

			f.clearRequests(batchedRequests)

			if len(unspentOutputs) < 2 && len(batchedRequests) == 0 {
				// no need to sweep or send funds
				continue
			}

			remainderOutput, err := f.sendFaucetMessage(unspentOutputs, batchedRequests, shutdownSignal)
			if err != nil {
				if common.IsCriticalError(err) != nil {
					// error is a critical error
					// => stop the faucet
					return err
				}

				f.logSoftError(err)
				continue
			}

			lastRemainderOutput = remainderOutput
		}
	}
}
