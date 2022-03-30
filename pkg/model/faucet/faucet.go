package faucet

import (
	"bytes"
	"context"
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
	"github.com/gohornet/hornet/pkg/model/syncmanager"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/restapi"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/gohornet/hornet/pkg/whiteflag"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hive.go/syncutils"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/builder"
)

// SendMessageFunc is a function which sends a message to the network.
type SendMessageFunc = func(msg *storage.Message) error

// TipselFunc selects tips for the faucet.
type TipselFunc = func() (tips hornet.MessageIDs, err error)

var (
	// ErrNoTipsGiven is returned when no tips were given to issue a message.
	ErrNoTipsGiven = errors.New("no tips given")
	// ErrNothingToProcess is returned when there is no need to sweep or send funds.
	ErrNothingToProcess = errors.New("nothing to process")
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
	Bech32  string
	Amount  uint64
	Address iotago.Address
}

// pendingTransaction holds info about a sent transaction that is pending.
type pendingTransaction struct {
	MessageID   hornet.MessageID
	QueuedItems []*queueItem
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
	// lock used to secure the state of the faucet.
	syncutils.Mutex
	// the logger used to log events.
	*utils.WrappedLogger

	// used to access the global daemon.
	daemon daemon.Daemon
	// used to access the node storage.
	storage *storage.Storage
	// used to determine the sync status of the node.
	syncManager *syncmanager.SyncManager
	// id of the network the faucet is running in.
	networkID uint64
	// Deserialization parameters including byte costs
	deSeriParas *iotago.DeSerializationParameters
	// belowMaxDepth is the maximum allowed delta
	// value between OCRI of a given message in relation to the current CMI before it gets lazy.
	belowMaxDepth milestone.Index
	// used to get the outputs.
	utxoManager *utxo.Manager
	// the address of the faucet.
	address iotago.Address
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

	// faucetBalance is the remaining balance of the faucet if all requests would be processed.
	faucetBalance uint64
	// queue of new requests.
	queue chan *queueItem
	// map with all queued requests per address (bech32).
	queueMap map[string]*queueItem
	// flushQueue is used to signal to stop an ongoing batching of faucet requests.
	flushQueue chan struct{}
	// pendingTransactionsMap is a map of sent transactions that are pending.
	pendingTransactionsMap map[string]*pendingTransaction
	// the message ID of the last sent faucet message.
	lastMessageID hornet.MessageID
	// the latest unused UTXO output that may not be confirmed yet but can be reused in new transactions.
	// this is used to issue multiple transactions without waiting for the confirmation by milestones.
	lastRemainderOutput *utxo.Output
}

// the default options applied to the faucet.
var defaultOptions = []Option{
	WithHRPNetworkPrefix(iotago.PrefixTestnet),
	WithAmount(10000000),            // 10 Mi
	WithSmallAmount(1000000),        // 1 Mi
	WithMaxAddressBalance(20000000), // 20 Mi
	WithMaxOutputCount(iotago.MaxOutputsCount),
	WithTagMessage("HORNET FAUCET"),
	WithBatchTimeout(2 * time.Second),
	WithPowWorkerCount(0),
}

// Options define options for the faucet.
type Options struct {
	// the logger used to log events.
	logger            *logger.Logger
	hrpNetworkPrefix  iotago.NetworkPrefix
	amount            uint64
	smallAmount       uint64
	maxAddressBalance uint64
	maxOutputCount    int
	tagMessage        []byte
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
		if maxOutputCount < 2 {
			maxOutputCount = 2
		}
		opts.maxOutputCount = maxOutputCount
	}
}

// WithTagMessage defines the faucet transaction tag payload.
func WithTagMessage(tagMessage string) Option {
	return func(opts *Options) {
		opts.tagMessage = []byte(tagMessage)
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
	daemon daemon.Daemon,
	dbStorage *storage.Storage,
	syncManager *syncmanager.SyncManager,
	networkID uint64,
	deSeriParas *iotago.DeSerializationParameters,
	belowMaxDepth int,
	utxoManager *utxo.Manager,
	address iotago.Address,
	addressSigner iotago.AddressSigner,
	tipselFunc TipselFunc,
	powHandler *pow.Handler,
	sendMessageFunc SendMessageFunc,
	opts ...Option) *Faucet {

	options := &Options{}
	options.apply(defaultOptions...)
	options.apply(opts...)

	faucet := &Faucet{
		daemon:          daemon,
		storage:         dbStorage,
		syncManager:     syncManager,
		networkID:       networkID,
		deSeriParas:     deSeriParas,
		belowMaxDepth:   milestone.Index(belowMaxDepth),
		utxoManager:     utxoManager,
		address:         address,
		addressSigner:   addressSigner,
		tipselFunc:      tipselFunc,
		powHandler:      powHandler,
		sendMessageFunc: sendMessageFunc,
		opts:            options,

		Events: &Events{
			IssuedMessage: events.NewEvent(storage.MessageIDCaller),
			SoftError:     events.NewEvent(events.ErrorCaller),
		},
	}
	faucet.WrappedLogger = utils.NewWrappedLogger(options.logger)
	faucet.init()

	return faucet
}

func (f *Faucet) init() {
	f.faucetBalance = 0
	f.queue = make(chan *queueItem, 5000)
	f.queueMap = make(map[string]*queueItem)
	f.flushQueue = make(chan struct{})
	f.pendingTransactionsMap = make(map[string]*pendingTransaction)
	f.lastMessageID = nil
	f.lastRemainderOutput = nil
}

// NetworkPrefix returns the used network prefix.
func (f *Faucet) NetworkPrefix() iotago.NetworkPrefix {
	return f.opts.hrpNetworkPrefix
}

// Info returns the used faucet address and remaining balance.
func (f *Faucet) Info() (*FaucetInfoResponse, error) {
	return &FaucetInfoResponse{
		Address: f.address.Bech32(f.opts.hrpNetworkPrefix),
		Balance: f.faucetBalance,
	}, nil
}

func (f *Faucet) collectUnspentBasicOutputsWithoutConstraints(address iotago.Address) (utxo.Outputs, uint64, error) {

	outputHasSpendingConstraint := func(output *utxo.Output) bool {
		conditions := output.Output().UnlockConditions().MustSet()
		return conditions.HasStorageDepositReturnCondition() || conditions.HasExpirationCondition() || conditions.HasTimelockCondition()
	}

	outputs := utxo.Outputs{}
	var balance uint64
	consumerFunc := func(output *utxo.Output) bool {
		if output.OutputType() != iotago.OutputBasic {
			return true
		}
		ownerAddress := output.Output().UnlockConditions().MustSet().Address().Address
		if ownerAddress != nil && address.Equal(ownerAddress) && !outputHasSpendingConstraint(output) {
			outputs = append(outputs, output)
			balance += output.Deposit()
		}
		return true
	}
	if err := f.utxoManager.ForEachUnspentOutput(consumerFunc, utxo.ReadLockLedger(false)); err != nil {
		return nil, 0, err
	}
	return outputs, balance, nil
}

func (f *Faucet) computeAddressBalance(address iotago.Address) (uint64, error) {
	_, balance, err := f.collectUnspentBasicOutputsWithoutConstraints(address)
	return balance, err
}

// Enqueue adds a new faucet request to the queue.
func (f *Faucet) Enqueue(bech32Addr string) (*FaucetEnqueueResponse, error) {

	addr, err := f.parseBech32Address(bech32Addr)
	if err != nil {
		return nil, err
	}

	if !f.syncManager.IsNodeAlmostSynced() {
		return nil, errors.WithMessage(echo.ErrInternalServerError, "Faucet node is not synchronized. Please try again later!")
	}

	f.Lock()
	defer f.Unlock()

	if _, exists := f.queueMap[bech32Addr]; exists {
		return nil, errors.WithMessage(restapi.ErrInvalidParameter, "Address is already in the queue.")
	}

	amount := f.opts.amount
	balance, err := f.computeAddressBalance(addr)
	if err == nil && balance >= f.opts.amount {
		amount = f.opts.smallAmount

		if balance >= f.opts.maxAddressBalance {
			return nil, errors.WithMessage(restapi.ErrInvalidParameter, "You already have enough funds on your address.")
		}
	}

	if amount > f.faucetBalance {
		return nil, errors.WithMessage(echo.ErrInternalServerError, "Faucet does not have enough funds to process your request. Please try again later!")
	}

	request := &queueItem{
		Bech32:  bech32Addr,
		Amount:  amount,
		Address: addr,
	}

	select {
	case f.queue <- request:
		f.faucetBalance -= amount
		f.queueMap[bech32Addr] = request
		return &FaucetEnqueueResponse{
			Address:         bech32Addr,
			WaitingRequests: len(f.queueMap),
		}, nil

	default:
		// queue is full
		return nil, errors.WithMessage(echo.ErrInternalServerError, "Faucet queue is full. Please try again later!")
	}
}

// FlushRequests stops current batching of faucet requests.
func (f *Faucet) FlushRequests() {
	f.flushQueue <- struct{}{}
}

// parseBech32Address parses a bech32 address.
func (f *Faucet) parseBech32Address(bech32Addr string) (iotago.Address, error) {

	hrp, bech32Address, err := iotago.ParseBech32(bech32Addr)
	if err != nil {
		return nil, errors.WithMessage(restapi.ErrInvalidParameter, "Invalid bech32 address provided!")
	}

	if hrp != f.NetworkPrefix() {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "Invalid bech32 address provided! Address does not start with \"%s\".", f.NetworkPrefix())
	}

	return bech32Address, nil
}

// clearRequestWithoutLocking clear the old request from the map.
// this is necessary to be able to send a new request to the same address.
// write lock must be acquired outside.
func (f *Faucet) clearRequestWithoutLocking(request *queueItem) {
	delete(f.queueMap, request.Bech32)
}

// clearRequestsWithoutLocking clears the old requests from the map.
// this is necessary to be able to send new requests to the same addresses.
// write lock must be acquired outside.
func (f *Faucet) clearRequestsWithoutLocking(batchedRequests []*queueItem) {
	for _, request := range batchedRequests {
		f.clearRequestWithoutLocking(request)
	}
}

// readdRequestsWithoutLocking adds old requests back to the queue.
// write lock must be acquired outside.
func (f *Faucet) readdRequestsWithoutLocking(batchedRequests []*queueItem) {
	for _, request := range batchedRequests {
		select {
		case f.queue <- request:
		default:
			// queue full => no way to readd it, delete it from the map as well so user are able to send a new request
			f.clearRequestWithoutLocking(request)
		}
	}
}

// addPendingTransactionWithoutLocking tracks a pending transaction.
// write lock must be acquired outside.
func (f *Faucet) addPendingTransactionWithoutLocking(pending *pendingTransaction) {
	f.pendingTransactionsMap[pending.MessageID.ToMapKey()] = pending
}

// clearPendingTransactionWithoutLocking removes tracking of a pending transaction.
// write lock must be acquired outside.
func (f *Faucet) clearPendingTransactionWithoutLocking(msgID hornet.MessageID) {
	delete(f.pendingTransactionsMap, msgID.ToMapKey())
}

// createMessage creates a new message and references the last faucet message.
func (f *Faucet) createMessage(ctx context.Context, txPayload iotago.Payload, tip ...hornet.MessageID) (*storage.Message, error) {

	tips, err := f.tipselFunc()
	if err != nil {
		return nil, err
	}

	if len(tip) > 0 {
		// if a tip was passed, use that one
		if len(tips) < iotago.MaxParentsInAMessage {
			tips = append(tips, tip[0])
		} else {
			tips[0] = tip[0]
		}
		tips = tips.RemoveDupsAndSortByLexicalOrder()
	}

	// create the message
	iotaMsg := &iotago.Message{
		ProtocolVersion: iotago.ProtocolVersion,
		Parents:         tips.ToSliceOfArrays(),
		Payload:         txPayload,
	}

	if err := f.powHandler.DoPoW(ctx, iotaMsg, 1); err != nil {
		return nil, err
	}

	msg, err := storage.NewMessage(iotaMsg, serializer.DeSeriModePerformValidation, f.deSeriParas)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

// buildTransactionPayload creates a signed transaction payload with all UTXO and batched requests.
func (f *Faucet) buildTransactionPayload(unspentOutputs []*utxo.Output, batchedRequests []*queueItem) (*iotago.Transaction, *iotago.UTXOInput, uint64, error) {

	txBuilder := builder.NewTransactionBuilder(f.networkID)
	txBuilder.AddTaggedDataPayload(&iotago.TaggedData{Tag: f.opts.tagMessage, Data: nil})

	outputCount := 0
	var remainderAmount int64 = 0

	// collect all unspent output of the faucet address
	for _, unspentOutput := range unspentOutputs {
		outputCount++
		remainderAmount += int64(unspentOutput.Deposit())
		txBuilder.AddInput(&builder.ToBeSignedUTXOInput{Address: f.address, OutputID: *unspentOutput.OutputID(), Output: unspentOutput.Output()})
	}

	// add all requests as outputs
	for _, req := range batchedRequests {
		outputCount++

		if outputCount >= f.opts.maxOutputCount-1 {
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

		txBuilder.AddOutput(&iotago.BasicOutput{
			Amount: amount,
			Conditions: iotago.UnlockConditions{
				&iotago.AddressUnlockCondition{Address: req.Address},
			},
		})
	}

	if remainderAmount > 0 {
		txBuilder.AddOutput(&iotago.BasicOutput{
			Amount: uint64(remainderAmount),
			Conditions: iotago.UnlockConditions{
				&iotago.AddressUnlockCondition{Address: f.address},
			},
		})
	}

	txPayload, err := txBuilder.Build(f.deSeriParas, f.addressSigner)
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
	for _, output := range txPayload.Essence.Outputs {
		basicOutput := output.(*iotago.BasicOutput)
		conditions, err := basicOutput.UnlockConditions().Set()
		if err != nil {
			return nil, nil, 0, err
		}
		addr := conditions.Address().Address

		if f.address.Equal(addr) {
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

// sendFaucetMessage creates a faucet transaction payload and remembers the last sent messageID and the lastRemainderOutput.
func (f *Faucet) sendFaucetMessage(ctx context.Context, unspentOutputs []*utxo.Output, batchedRequests []*queueItem, tip ...hornet.MessageID) error {

	txPayload, remainderIotaGoOutput, remainderAmount, err := f.buildTransactionPayload(unspentOutputs, batchedRequests)
	if err != nil {
		return fmt.Errorf("build transaction payload failed, error: %w", err)
	}

	msg, err := f.createMessage(ctx, txPayload, tip...)
	if err != nil {
		return fmt.Errorf("build faucet message failed, error: %w", err)
	}

	if err := f.sendMessageFunc(msg); err != nil {
		return fmt.Errorf("send faucet message failed, error: %w", err)
	}

	f.Lock()
	f.lastMessageID = msg.MessageID()
	f.addPendingTransactionWithoutLocking(&pendingTransaction{MessageID: msg.MessageID(), QueuedItems: batchedRequests})
	if remainderIotaGoOutput != nil {
		remainderIotaGoOutputID := remainderIotaGoOutput.ID()
		output := &iotago.BasicOutput{
			Amount: remainderAmount,
			Conditions: iotago.UnlockConditions{
				&iotago.AddressUnlockCondition{Address: f.address},
			},
		}
		f.lastRemainderOutput = utxo.CreateOutput(&remainderIotaGoOutputID, msg.MessageID(), 0, 0, output)
	} else {
		// no funds remaining => no remainder output
		f.lastRemainderOutput = nil
	}
	f.Unlock()

	f.Events.IssuedMessage.Trigger(msg.MessageID())

	return nil
}

// logSoftError logs a soft error and triggers the event.
func (f *Faucet) logSoftError(err error) {
	f.LogWarn(err)
	f.Events.SoftError.Trigger(err)
}

// collectRequests collects faucet requests until the maximum amount or a timeout is reached.
// locking not required.
func (f *Faucet) collectRequests(ctx context.Context) ([]*queueItem, error) {

	batchedRequests := []*queueItem{}
	collectedRequestsCounter := 0

CollectValues:
	for collectedRequestsCounter < f.opts.maxOutputCount {
		select {
		case <-ctx.Done():
			// faucet was stopped
			return nil, common.ErrOperationAborted

		case <-time.After(f.opts.batchTimeout):
			// timeout was reached => stop collecting requests
			break CollectValues

		case <-f.flushQueue:
			// flush signal => stop collecting requests
			for collectedRequestsCounter < f.opts.maxOutputCount {
				// collect all pending requests
				select {
				case request := <-f.queue:
					batchedRequests = append(batchedRequests, request)
					collectedRequestsCounter++

				default:
					// no pending requests
					break CollectValues
				}
			}
			break CollectValues

		case request := <-f.queue:
			batchedRequests = append(batchedRequests, request)
			collectedRequestsCounter++
		}
	}

	return batchedRequests, nil
}

// processRequestsWithoutLocking processes all possible requests considering the maximum transaction size and the remaining funds of the faucet.
// write lock must be acquired outside.
func (f *Faucet) processRequestsWithoutLocking(collectedRequestsCounter int, amount uint64, batchedRequests []*queueItem) []*queueItem {
	processedBatchedRequests := []*queueItem{}
	unprocessedBatchedRequests := []*queueItem{}
	nodeAlmostSynced := f.syncManager.IsNodeAlmostSynced()

	for i := range batchedRequests {
		request := batchedRequests[i]

		if !nodeAlmostSynced {
			// request can't be processed because the node is not synchronized => re-add it to the queue
			unprocessedBatchedRequests = append(unprocessedBatchedRequests, request)
			continue
		}

		if collectedRequestsCounter >= f.opts.maxOutputCount-1 {
			// request can't be processed in this transaction => re-add it to the queue
			unprocessedBatchedRequests = append(unprocessedBatchedRequests, request)
			continue
		}

		if amount < request.Amount {
			// not enough funds to process this request => ignore the request
			f.clearRequestWithoutLocking(request)
			continue
		}

		// request can be processed in this transaction
		amount -= request.Amount
		collectedRequestsCounter++
		processedBatchedRequests = append(processedBatchedRequests, request)
	}

	f.readdRequestsWithoutLocking(unprocessedBatchedRequests)

	return processedBatchedRequests
}

// RunFaucetLoop collects unspent outputs on the faucet address and batches the requests from the queue.
func (f *Faucet) RunFaucetLoop(ctx context.Context, initDoneCallback func()) error {

	// set initial faucet balance
	faucetBalance, err := f.computeAddressBalance(f.address)
	if err != nil {
		return common.CriticalError(fmt.Errorf("reading faucet address balance failed: %s, error: %s", f.address.Bech32(f.opts.hrpNetworkPrefix), err))
	}
	f.faucetBalance = faucetBalance

	if initDoneCallback != nil {
		initDoneCallback()
	}

	for {
		select {
		case <-ctx.Done():
			// faucet was stopped
			return nil

		default:
			// first collect requests
			batchedRequests, err := f.collectRequests(ctx)
			if err != nil {
				if err == common.ErrOperationAborted {
					return nil
				}
				if common.IsCriticalError(err) != nil {
					// error is a critical error
					// => stop the faucet
					return err
				}
				f.logSoftError(err)
				continue
			}

			collectUnspentOutputsWithoutLocking := func() ([]*utxo.Output, uint64, error) {
				if f.lastRemainderOutput != nil {
					// the lastRemainderOutput is reused as input in the next transaction, even if it was not yet referenced by a milestone.
					// this is done to increase the throughput of the faucet in high load situations.
					// we can't collect unspent outputs, as long as the lastRemainderOutput was not confirmed,
					// since it's creating transaction could also have consumed the same UTXOs.
					return []*utxo.Output{f.lastRemainderOutput}, f.lastRemainderOutput.Deposit(), nil
				}
				return f.collectUnspentBasicOutputsWithoutConstraints(f.address)
			}

			processRequests := func() ([]*utxo.Output, []*queueItem, hornet.MessageIDs, error) {
				// first we need to read lock the ledger, to be sure that there is no confirmation ongoing
				f.utxoManager.ReadLockLedger()
				defer f.utxoManager.ReadUnlockLedger()

				// there must be a lock between collectUnspentOutputsWithoutLocking and "tipselection", otherwise the chaining may fail
				f.Lock()
				defer f.Unlock()

				unspentOutputs, amount, err := collectUnspentOutputsWithoutLocking()
				if err != nil {
					return nil, nil, nil, err
				}

				if len(unspentOutputs) < 2 && len(batchedRequests) == 0 {
					// no need to sweep or send funds
					return nil, nil, nil, ErrNothingToProcess
				}

				// if a lastMessageID exists, we need to reference it to chain the transactions in the correct order for whiteflag.
				// lastMessageID is reset by ApplyConfirmation in case the last faucet message is not confirmed and below max depth.
				var tips hornet.MessageIDs
				if f.lastMessageID != nil {
					tip := make(hornet.MessageID, len(f.lastMessageID))
					copy(tip, f.lastMessageID)
					tips = append(tips, tip)
				}

				processableRequests := f.processRequestsWithoutLocking(len(unspentOutputs), amount, batchedRequests)

				return unspentOutputs, processableRequests, tips, nil
			}

			unspentOutputs, processableRequests, tips, err := processRequests()
			if err != nil {
				if err != ErrNothingToProcess {
					if common.IsCriticalError(err) != nil {
						// error is a critical error
						// => stop the faucet
						return err
					}
					f.logSoftError(err)
				}
				continue
			}

			if err := f.sendFaucetMessage(ctx, unspentOutputs, processableRequests, tips...); err != nil {
				if common.IsCriticalError(err) != nil {
					// error is a critical error
					// => stop the faucet
					return err
				}
				f.readdRequestsWithoutLocking(processableRequests)
				f.logSoftError(err)
				continue
			}
		}
	}
}

// ApplyConfirmation applies new milestone confirmations to the faucet.
// Pending transactions are checked for their current state and either removed, readded, or left pending.
// If a conflict is found, all remaining pending transactions are readded to the queue.
// no need to ReadLockLedger, because this function should be called from milestone confirmation event anyway.
func (f *Faucet) ApplyConfirmation(confirmation *whiteflag.Confirmation) error {
	if confirmation == nil {
		return nil
	}

	f.Lock()
	defer f.Unlock()

	conflicting := false
	cmi := confirmation.MilestoneIndex

	// check pending transactions for confirmation
	for _, msgID := range confirmation.Mutations.MessagesIncludedWithTransactions {
		if pendingTx, pending := f.pendingTransactionsMap[msgID.ToMapKey()]; pending {
			// transaction was confirmed => delete the requests and the pending transaction
			f.clearRequestsWithoutLocking(pendingTx.QueuedItems)
			f.clearPendingTransactionWithoutLocking(msgID)

			if f.lastMessageID != nil && bytes.Equal(f.lastMessageID[:], msgID[:]) {
				// the latest message got confirmed, reset the lastMessageID
				f.lastMessageID = nil
			}

			if f.lastRemainderOutput != nil && bytes.Equal(f.lastRemainderOutput.MessageID()[:], msgID[:]) {
				// the latest transaction got confirmed, reset the lastRemainderOutput
				f.lastRemainderOutput = nil
			}
		}
	}

	// check pending transactions for conflicts
	for _, conflict := range confirmation.Mutations.MessagesExcludedWithConflictingTransactions {
		if pendingTx, pending := f.pendingTransactionsMap[conflict.MessageID.ToMapKey()]; pending {
			// transaction was conflicting => readd the items to the queue and delete the pending transaction
			conflicting = true
			f.readdRequestsWithoutLocking(pendingTx.QueuedItems)
			f.clearPendingTransactionWithoutLocking(conflict.MessageID)
		}
	}

	checkPendingMessageMetadata := func(pendingTx *pendingTransaction) {
		msgID := pendingTx.MessageID

		cachedMsgMeta := f.storage.CachedMessageMetadataOrNil(msgID) // meta +1
		if cachedMsgMeta == nil {
			// message unknown => delete the requests and the pending transaction
			conflicting = true
			f.clearRequestsWithoutLocking(pendingTx.QueuedItems)
			f.clearPendingTransactionWithoutLocking(msgID)
			return
		}
		defer cachedMsgMeta.Release(true) // meta -1

		metadata := cachedMsgMeta.Metadata()
		if metadata.IsReferenced() {
			if metadata.IsConflictingTx() {
				// transaction was conflicting => readd the items to the queue and delete the pending transaction
				conflicting = true
				f.readdRequestsWithoutLocking(pendingTx.QueuedItems)
				f.clearPendingTransactionWithoutLocking(msgID)
				return
			}

			// transaction was confirmed => delete the requests and the pending transaction
			f.clearRequestsWithoutLocking(pendingTx.QueuedItems)
			f.clearPendingTransactionWithoutLocking(msgID)
			return
		}

		// check if message is "below max depth"
		_, ocri, err := dag.ConeRootIndexes(f.daemon.ContextStopped(), f.storage, cachedMsgMeta.Retain(), cmi) // meta pass +1
		if err != nil {
			// an error occurred => readd the items to the queue and delete the pending transaction
			conflicting = true
			f.readdRequestsWithoutLocking(pendingTx.QueuedItems)
			f.clearPendingTransactionWithoutLocking(msgID)
			return
		}

		if (cmi - ocri) > milestone.Index(f.belowMaxDepth) {
			// below max depth => readd the items to the queue and delete the pending transaction
			conflicting = true
			f.readdRequestsWithoutLocking(pendingTx.QueuedItems)
			f.clearPendingTransactionWithoutLocking(msgID)
		}
	}

	// check all remaining pending transactions
	for _, pendingTx := range f.pendingTransactionsMap {
		checkPendingMessageMetadata(pendingTx)
	}

	if conflicting {
		// there was a conflict in the chain
		// => reset the lastMessageID and lastRemainderOutput to collect outputs and reissue all pending transactions
		f.lastMessageID = nil
		f.lastRemainderOutput = nil

		for _, pendingTx := range f.pendingTransactionsMap {
			f.readdRequestsWithoutLocking(pendingTx.QueuedItems)
			f.clearPendingTransactionWithoutLocking(pendingTx.MessageID)
		}
	}

	// calculate total balance of all pending requests
	var pendingRequestsBalance uint64 = 0
	for _, pendingRequest := range f.queueMap {
		pendingRequestsBalance += pendingRequest.Amount
	}

	// recalculate the current faucet balance
	// no need to lock since we are in the milestone confirmation anyway
	faucetBalance, err := f.computeAddressBalance(f.address)
	if err != nil {
		return common.CriticalError(fmt.Errorf("reading faucet address balance failed: %s, error: %s", f.address.Bech32(f.opts.hrpNetworkPrefix), err))
	}

	if faucetBalance < pendingRequestsBalance {
		f.faucetBalance = 0
		return nil
	}

	f.faucetBalance = faucetBalance - pendingRequestsBalance
	return nil
}
