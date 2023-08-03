package webapi

import (
	"fmt"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hornet/pkg/model/hornet"
	"github.com/iotaledger/hornet/pkg/model/milestone"
	"github.com/iotaledger/hornet/pkg/model/tangle"
)

// Container holds an object.
type Container interface {
	Item() Container
}

type newTxWithValueFunc[T Container] func(txHash trinary.Hash, address trinary.Hash, index uint64, value int64) T
type newTxHashWithValueFunc[H Container] func(txHash trinary.Hash, tailTxHash trinary.Hash, bundleHash trinary.Hash, address trinary.Hash, value int64) H
type newBundleWithValueFunc[B Container, T Container] func(bundleHash trinary.Hash, tailTxHash trinary.Hash, transactions []T, lastIndex uint64) B

//nolint:nonamedreturns
func getMilestoneStateDiff[T Container, H Container, B Container](milestoneIndex milestone.Index, newTxWithValue newTxWithValueFunc[T], newTxHashWithValue newTxHashWithValueFunc[H], newBundleWithValue newBundleWithValueFunc[B, T]) (confirmedTxWithValue []H, confirmedBundlesWithValue []B, totalLedgerChanges map[string]int64, err error) {

	cachedMsBndl := tangle.GetMilestoneOrNil(milestoneIndex)
	if cachedMsBndl == nil {
		return nil, nil, nil, fmt.Errorf("milestone not found: %d", milestoneIndex)
	}
	defer cachedMsBndl.Release(true)

	msBndl := cachedMsBndl.GetBundle()

	txsToConfirm := make(map[string]struct{})
	txsToTraverse := make(map[string]struct{})
	totalLedgerChanges = make(map[string]int64)

	txsToTraverse[string(msBndl.GetTailHash())] = struct{}{}

	// Collect all tx to check by traversing the tangle
	// Loop as long as new transactions are added in every loop cycle
	for len(txsToTraverse) != 0 {

		for txHash := range txsToTraverse {
			delete(txsToTraverse, txHash)

			if _, checked := txsToConfirm[txHash]; checked {
				// Tx was already checked => ignore
				continue
			}

			if tangle.SolidEntryPointsContain(hornet.Hash(txHash)) {
				// Ignore solid entry points (snapshot milestone included)
				continue
			}

			cachedTxMeta := tangle.GetCachedTxMetadataOrNil(hornet.Hash(txHash))
			if cachedTxMeta == nil {
				return nil, nil, nil, fmt.Errorf("getMilestoneStateDiff: transaction not found: %v", hornet.Hash(txHash).Trytes())
			}
			txMeta := cachedTxMeta.GetMetadata()
			cachedTxMeta.Release(true)

			confirmed, at := txMeta.GetConfirmed()
			if confirmed {
				if at != milestoneIndex {
					// ignore all tx that were confirmed by another milestone
					continue
				}
			} else {
				return nil, nil, nil, fmt.Errorf("getMilestoneStateDiff: transaction not confirmed yet: %v", hornet.Hash(txHash).Trytes())
			}

			// Mark the approvees to be traversed
			txsToTraverse[string(txMeta.GetTrunkHash())] = struct{}{}
			txsToTraverse[string(txMeta.GetBranchHash())] = struct{}{}

			if !txMeta.IsTail() {
				continue
			}

			cachedBndl := tangle.GetCachedBundleOrNil(hornet.Hash(txHash))
			if cachedBndl == nil {
				txBundle := txMeta.GetBundleHash()

				return nil, nil, nil, fmt.Errorf("getMilestoneStateDiff: Tx: %v, bundle not found: %v", hornet.Hash(txHash).Trytes(), txBundle.Trytes())
			}
			bndl := cachedBndl.GetBundle()
			cachedBndl.Release(true)

			if !bndl.IsValid() {
				txBundle := txMeta.GetBundleHash()

				return nil, nil, nil, fmt.Errorf("getMilestoneStateDiff: Tx: %v, bundle not valid: %v", hornet.Hash(txHash).Trytes(), txBundle.Trytes())
			}

			if !bndl.IsValueSpam() {
				ledgerChanges := bndl.GetLedgerChanges()

				var txsWithValue []T

				cachedTxs := bndl.GetTransactions()
				for _, cachedTx := range cachedTxs {
					hornetTx := cachedTx.GetTransaction()
					// hornetTx is being retained during the loop, so safe to use the pointer here
					if hornetTx.Tx.Value != 0 {
						confirmedTxWithValue = append(confirmedTxWithValue, newTxHashWithValue(hornetTx.Tx.Hash, bndl.GetTailHash().Trytes(), hornetTx.Tx.Bundle, hornetTx.Tx.Address, hornetTx.Tx.Value))
					}
					txsWithValue = append(txsWithValue, newTxWithValue(hornetTx.Tx.Hash, hornetTx.Tx.Address, hornetTx.Tx.CurrentIndex, hornetTx.Tx.Value))
				}
				for address, change := range ledgerChanges {
					totalLedgerChanges[address] += change
				}

				cachedBundleHeadTx := bndl.GetHead()
				bndlHeadTx := cachedBundleHeadTx.GetTransaction()
				cachedBundleHeadTx.Release(true)

				confirmedBundlesWithValue = append(confirmedBundlesWithValue, newBundleWithValue(txMeta.GetBundleHash().Trytes(), bndl.GetTailHash().Trytes(), txsWithValue, bndlHeadTx.Tx.CurrentIndex))
			}

			// we only add the tail transaction to the txsToConfirm set, in order to not
			// accidentally skip cones, in case the other transactions (non-tail) of the bundle do not
			// reference the same trunk transaction (as seen from the PoV of the bundle).
			// if we wouldn't do it like this, we have a high chance of computing an
			// inconsistent ledger state.
			txsToConfirm[txHash] = struct{}{}
		}
	}

	return confirmedTxWithValue, confirmedBundlesWithValue, totalLedgerChanges, nil
}

func (s *WebAPIServer) rpcGetLedgerState(c echo.Context) (interface{}, error) {
	request := &GetLedgerState{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid request, error: %s", err)
	}

	balances, index, err := tangle.GetLedgerStateForMilestone(c.Request().Context(), request.TargetIndex)
	if err != nil {
		return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
	}

	balancesTrytes := make(map[trinary.Trytes]uint64)
	for address, balance := range balances {
		balancesTrytes[hornet.Hash(address).Trytes()] = balance
	}

	return &GetLedgerStateResponse{
		Balances:       balancesTrytes,
		MilestoneIndex: index,
	}, nil
}

func (s *WebAPIServer) rpcGetLedgerDiff(c echo.Context) (interface{}, error) {
	request := &GetLedgerDiff{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid request, error: %s", err)
	}

	smi := tangle.GetSolidMilestoneIndex()
	requestedIndex := request.MilestoneIndex
	if requestedIndex > smi {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid milestone index: %d, lsmi is %d", requestedIndex, smi)
	}

	diff, err := tangle.GetLedgerDiffForMilestone(c.Request().Context(), requestedIndex)
	if err != nil {
		return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
	}

	diffTrytes := make(map[trinary.Trytes]int64)
	for address, balance := range diff {
		diffTrytes[hornet.Hash(address).Trytes()] = balance
	}

	return &GetLedgerDiffResponse{
		Diff:           diffTrytes,
		MilestoneIndex: request.MilestoneIndex,
	}, nil
}

func (s *WebAPIServer) rpcGetLedgerDiffExt(c echo.Context) (interface{}, error) {
	request := &GetLedgerDiffExt{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid request, error: %s", err)
	}

	smi := tangle.GetSolidMilestoneIndex()
	requestedIndex := request.MilestoneIndex
	if requestedIndex > smi {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid milestone index: %d, lsmi is %d", requestedIndex, smi)
	}

	newTxWithValue := func(txHash trinary.Hash, address trinary.Hash, index uint64, value int64) *TxWithValue {
		return &TxWithValue{
			TxHash:  txHash,
			Address: address,
			Index:   index,
			Value:   value,
		}
	}

	newTxHashWithValue := func(txHash trinary.Hash, tailTxHash trinary.Hash, bundleHash trinary.Hash, address trinary.Hash, value int64) *TxHashWithValue {
		return &TxHashWithValue{
			TxHash:     txHash,
			TailTxHash: tailTxHash,
			BundleHash: bundleHash,
			Address:    address,
			Value:      value,
		}
	}

	newBundleWithValue := func(bundleHash trinary.Hash, tailTxHash trinary.Hash, transactions []*TxWithValue, lastIndex uint64) *BundleWithValue {
		return &BundleWithValue{
			BundleHash: bundleHash,
			TailTxHash: tailTxHash,
			Txs:        transactions,
			LastIndex:  lastIndex,
		}
	}

	confirmedTxWithValue, confirmedBundlesWithValue, ledgerChanges, err := getMilestoneStateDiff(requestedIndex, newTxWithValue, newTxHashWithValue, newBundleWithValue)
	if err != nil {
		return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
	}

	ledgerChangesTrytes := make(map[trinary.Trytes]int64)
	for address, balance := range ledgerChanges {
		ledgerChangesTrytes[hornet.Hash(address).Trytes()] = balance
	}

	result := &GetLedgerDiffExtResponse{}
	result.ConfirmedTxWithValue = confirmedTxWithValue
	result.ConfirmedBundlesWithValue = confirmedBundlesWithValue
	result.Diff = ledgerChangesTrytes
	result.MilestoneIndex = request.MilestoneIndex

	return result, nil
}

func (s *WebAPIServer) ledgerState(c echo.Context, targetIndex milestone.Index) (interface{}, error) {
	balances, index, err := tangle.GetLedgerStateForMilestone(c.Request().Context(), targetIndex)
	if err != nil {
		return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
	}

	addressesWithBalances := make(map[trinary.Trytes]string)
	for address, balance := range balances {
		addressesWithBalances[hornet.Hash(address).Trytes()] = strconv.FormatUint(balance, 10)
	}

	return &ledgerStateResponse{
		Balances:    addressesWithBalances,
		LedgerIndex: index,
	}, nil
}

func (s *WebAPIServer) ledgerStateByLatestSolidIndex(c echo.Context) (interface{}, error) {
	return s.ledgerState(c, 0)
}

func (s *WebAPIServer) ledgerStateByIndex(c echo.Context) (interface{}, error) {
	msIndex, err := ParseMilestoneIndexParam(c, ParameterMilestoneIndex)
	if err != nil {
		return nil, err
	}

	return s.ledgerState(c, milestone.Index(msIndex))
}

func (s *WebAPIServer) ledgerDiff(c echo.Context) (interface{}, error) {
	msIndexIotaGo, err := ParseMilestoneIndexParam(c, ParameterMilestoneIndex)
	if err != nil {
		return nil, err
	}
	msIndex := milestone.Index(msIndexIotaGo)

	smi := tangle.GetSolidMilestoneIndex()
	if msIndex > smi {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid milestone index: %d, lsmi is %d", msIndex, smi)
	}

	diff, err := tangle.GetLedgerDiffForMilestone(c.Request().Context(), msIndex)
	if err != nil {
		return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
	}

	addressesWithDiffs := make(map[trinary.Trytes]string)
	for address, balance := range diff {
		addressesWithDiffs[hornet.Hash(address).Trytes()] = strconv.FormatInt(balance, 10)
	}

	return &ledgerDiffResponse{
		AddressDiffs: addressesWithDiffs,
		LedgerIndex:  msIndex,
	}, nil
}

func (s *WebAPIServer) ledgerDiffExtended(c echo.Context) (interface{}, error) {
	msIndexIotaGo, err := ParseMilestoneIndexParam(c, ParameterMilestoneIndex)
	if err != nil {
		return nil, err
	}
	msIndex := milestone.Index(msIndexIotaGo)

	smi := tangle.GetSolidMilestoneIndex()
	if msIndex > smi {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid milestone index: %d, lsmi is %d", msIndex, smi)
	}

	newTxWithValue := func(txHash trinary.Hash, address trinary.Hash, index uint64, value int64) *txWithValue {
		return &txWithValue{
			TxHash:  txHash,
			Address: address,
			Index:   uint32(index),
			Value:   strconv.FormatInt(value, 10),
		}
	}

	newTxHashWithValue := func(txHash trinary.Hash, tailTxHash trinary.Hash, bundleHash trinary.Hash, address trinary.Hash, value int64) *txHashWithValue {
		return &txHashWithValue{
			TxHash:     txHash,
			TailTxHash: tailTxHash,
			Bundle:     bundleHash,
			Address:    address,
			Value:      strconv.FormatInt(value, 10),
		}
	}

	newBundleWithValue := func(bundleHash trinary.Hash, tailTxHash trinary.Hash, transactions []*txWithValue, lastIndex uint64) *bundleWithValue {
		return &bundleWithValue{
			Bundle:     bundleHash,
			TailTxHash: tailTxHash,
			Txs:        transactions,
			LastIndex:  uint32(lastIndex),
		}
	}

	confirmedTxWithValue, confirmedBundlesWithValue, ledgerChanges, err := getMilestoneStateDiff(msIndex, newTxWithValue, newTxHashWithValue, newBundleWithValue)
	if err != nil {
		return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
	}

	addressesWithDiffs := make(map[trinary.Trytes]string)
	for address, balance := range ledgerChanges {
		addressesWithDiffs[hornet.Hash(address).Trytes()] = strconv.FormatInt(balance, 10)
	}

	return ledgerDiffExtendedResponse{
		ConfirmedTxWithValue:      confirmedTxWithValue,
		ConfirmedBundlesWithValue: confirmedBundlesWithValue,
		AddressDiffs:              addressesWithDiffs,
		LedgerIndex:               msIndex,
	}, nil
}
