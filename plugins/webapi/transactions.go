package webapi

import (
	"fmt"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/iota.go/address"
	"github.com/iotaledger/iota.go/bundle"
	"github.com/iotaledger/iota.go/guards"
	"github.com/iotaledger/iota.go/math"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hornet/pkg/config"
	"github.com/iotaledger/hornet/pkg/model/hornet"
	"github.com/iotaledger/hornet/pkg/model/tangle"
	"github.com/iotaledger/hornet/plugins/gossip"
)

// checks whether the given trytes make up a syntactically and semantically valid migration bundle.
// for each input transaction it is also checked, whether the migration
// bundle is spending the entirety of the funds residing on the given address.
func isMigrationBundle(trytes []trinary.Trytes) error {
	// the trytes to broadcast must make up a complete migration bundle
	bndl, err := transaction.AsTransactionObjects(trytes, nil)
	if err != nil {
		return err
	}

	if err := bundle.ValidBundle(bndl, true); err != nil {
		return err
	}

	// check also that the input transactions are spending the entirety of
	// the funds residing on the given address:
	// obviously this is racy in re to what milestone is currently applied
	// but is still in place as a rudimentary safe-guard.
	for txIndex, tx := range bndl {
		if tx.Value < 0 {
			balance, _, err := tangle.GetBalanceForAddressWithoutLocking(hornet.HashFromAddressTrytes(tx.Address))
			if err != nil {
				return err
			}
			if balance != math.AbsInt64(tx.Value) {
				return fmt.Errorf("input transaction %d does not spend entirety of funds residing on its address", txIndex)
			}
		}
	}

	return nil
}

func (s *WebAPIServer) rpcBroadcastTransactions(c echo.Context) (interface{}, error) {
	request := &BroadcastTransactions{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid request, error: %s", err)
	}

	if len(request.Trytes) == 0 {
		return nil, errors.WithMessage(echo.ErrBadRequest, "No trytes provided")
	}

	for _, trytes := range request.Trytes {
		if err := trinary.ValidTrytes(trytes); err != nil {
			return nil, errors.WithMessage(echo.ErrBadRequest, err.Error())
		}
	}

	if !config.NodeConfig.GetBool(config.CfgWebAPIDisableMigrationBundleCheckOnBroadcast) {
		// we only allow the broadcasting of migration bundles
		if err := isMigrationBundle(request.Trytes); err != nil {
			return nil, errors.WithMessage(echo.ErrBadRequest, err.Error())
		}
	}

	for _, trytes := range request.Trytes {
		if err := gossip.Processor().ValidateTransactionTrytesAndEmit(trytes); err != nil {
			return nil, errors.WithMessage(echo.ErrBadRequest, err.Error())
		}
	}

	return &BradcastTransactionsResponse{}, nil
}

func (s *WebAPIServer) findTransactions(maxResults int, valueOnly bool, queryBundleHashes, queryApproveeHashes, queryAddressHashes, queryTagHashes map[string]struct{}) []string {

	results := make(map[string]struct{})
	searchedBefore := false

	// check if bundle hash search criteria was given
	if len(queryBundleHashes) > 0 {
		// search txs by bundle hash
		for bundleHash := range queryBundleHashes {
			for _, r := range tangle.GetBundleTransactionHashes(hornet.Hash(bundleHash), true, maxResults-len(results)) {
				results[string(r)] = struct{}{}
			}
		}
		searchedBefore = true
	}

	// check if approvee search criteria was given
	if len(queryApproveeHashes) > 0 {
		if !searchedBefore {
			// search txs by approvees
			for approveeHash := range queryApproveeHashes {
				for _, r := range tangle.GetApproverHashes(hornet.Hash(approveeHash), maxResults-len(results)) {
					results[string(r)] = struct{}{}
				}
			}
			searchedBefore = true
		} else {
			// check if results match at least one of the approvee search criteria
			for txHash := range results {
				contains := false
				for approveeHash := range queryApproveeHashes {
					if tangle.ContainsApprover(hornet.Hash(approveeHash), hornet.Hash(txHash)) {
						contains = true

						break
					}
				}
				if !contains {
					delete(results, txHash)
				}
			}
		}
	}

	// check if address search criteria was given
	if len(queryAddressHashes) > 0 {
		if !searchedBefore {
			// search txs by address
			for addressHash := range queryAddressHashes {
				for _, r := range tangle.GetTransactionHashesForAddress(hornet.Hash(addressHash), valueOnly, true, maxResults-len(results)) {
					results[string(r)] = struct{}{}
				}
			}
			searchedBefore = true
		} else {
			// check if results match at least one of the address search criteria
			for txHash := range results {
				contains := false
				for addressHash := range queryAddressHashes {
					if tangle.ContainsAddress(hornet.Hash(addressHash), hornet.Hash(txHash), valueOnly) {
						contains = true

						break
					}
				}
				if !contains {
					delete(results, txHash)
				}
			}
		}
	}

	// check if tag search criteria was given
	if len(queryTagHashes) > 0 {
		if !searchedBefore {
			// search txs by tags
			for tagHash := range queryTagHashes {
				for _, r := range tangle.GetTagHashes(hornet.Hash(tagHash), true, maxResults-len(results)) {
					results[string(r)] = struct{}{}
				}
			}
		} else {
			// check if results match at least one of the tag search criteria
			for txHash := range results {
				contains := false
				for tagHash := range queryTagHashes {
					if tangle.ContainsTag(hornet.Hash(tagHash), hornet.Hash(txHash)) {
						contains = true

						break
					}
				}
				if !contains {
					delete(results, txHash)
				}
			}
		}
	}

	// convert to slice
	txHashes := make([]string, 0, len(results))
	for r := range results {
		txHashes = append(txHashes, hornet.Hash(r).Trytes())
	}

	return txHashes
}

func (s *WebAPIServer) rpcFindTransactions(c echo.Context) (interface{}, error) {
	request := &FindTransactions{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid request, error: %s", err)
	}

	maxResults := s.limitsMaxResults
	if (request.MaxResults > 0) && (request.MaxResults < maxResults) {
		maxResults = request.MaxResults
	}

	if len(request.Bundles) == 0 && len(request.Addresses) == 0 && len(request.Approvees) == 0 && len(request.Tags) == 0 {
		return nil, errors.WithMessage(ErrInvalidParameter, "no search criteria was given")
	}

	queryBundleHashes := make(map[string]struct{})
	queryApproveeHashes := make(map[string]struct{})
	queryAddressHashes := make(map[string]struct{})
	queryTagHashes := make(map[string]struct{})

	// check all queries first
	for _, bundleTrytes := range request.Bundles {
		if !guards.IsTransactionHash(bundleTrytes) {
			return nil, errors.WithMessagef(ErrInvalidParameter, "invalid bundle hash provided: %s", bundleTrytes)
		}
		queryBundleHashes[string(hornet.HashFromHashTrytes(bundleTrytes))] = struct{}{}
	}

	for _, approveeTrytes := range request.Approvees {
		if !guards.IsTransactionHash(approveeTrytes) {
			return nil, errors.WithMessagef(ErrInvalidParameter, "invalid aprovee hash provided: %s", approveeTrytes)
		}
		queryApproveeHashes[string(hornet.HashFromHashTrytes(approveeTrytes))] = struct{}{}
	}

	for _, addressTrytes := range request.Addresses {
		if err := address.ValidAddress(addressTrytes); err != nil {
			return nil, errors.WithMessagef(ErrInvalidParameter, "invalid address hash provided: %s", addressTrytes)
		}
		queryAddressHashes[string(hornet.HashFromAddressTrytes(addressTrytes))] = struct{}{}
	}

	for _, tagTrytes := range request.Tags {
		if err := trinary.ValidTrytes(tagTrytes); err != nil {
			return nil, errors.WithMessagef(ErrInvalidParameter, "invalid tag trytes provided: %s", tagTrytes)
		}
		if len(tagTrytes) > 27 {
			return nil, errors.WithMessagef(ErrInvalidParameter, "invalid tag length: %s", tagTrytes)
		}
		if len(tagTrytes) < 27 {
			tagTrytes = trinary.MustPad(tagTrytes, 27)
		}
		queryTagHashes[string(hornet.HashFromTagTrytes(tagTrytes))] = struct{}{}
	}

	if !tangle.WaitForNodeSynced(waitForNodeSyncedTimeout) {
		return nil, errors.WithMessage(echo.ErrServiceUnavailable, tangle.ErrNodeNotSynced.Error())
	}

	tangle.ReadLockLedger()
	defer tangle.ReadUnlockLedger()

	txHashes := s.findTransactions(maxResults, request.ValueOnly, queryBundleHashes, queryApproveeHashes, queryAddressHashes, queryTagHashes)

	return &FindTransactionsResponse{
		Hashes: txHashes,
	}, nil
}

// redirect to broadcastTransactions
func (s *WebAPIServer) rpcStoreTransactions(c echo.Context) (interface{}, error) {
	return s.rpcBroadcastTransactions(c)
}

func (s *WebAPIServer) transactions(c echo.Context) (interface{}, error) {
	valueOnly := false
	for query := range c.QueryParams() {
		if strings.ToLower(query) == "valueonly" {
			valueOnly = true

			break
		}
	}

	maxResults, err := parseMaxResultsQueryParam(c, s.limitsMaxResults)
	if err != nil {
		return nil, err
	}

	requestBundleHash, err := parseBundleQueryParam(c)
	if err != nil {
		return nil, err
	}
	requestApproveeHash, err := parseApproveeQueryParam(c)
	if err != nil {
		return nil, err
	}
	requestAddressHash, err := parseAddressQueryParam(c)
	if err != nil {
		return nil, err
	}
	requestTagHash, err := parseTagQueryParam(c)
	if err != nil {
		return nil, err
	}

	if requestBundleHash == nil && requestApproveeHash == nil && requestAddressHash == nil && requestTagHash == nil {
		return nil, errors.WithMessage(ErrInvalidParameter, "no search criteria was given")
	}

	queryBundleHashes := make(map[string]struct{})
	queryApproveeHashes := make(map[string]struct{})
	queryAddressHashes := make(map[string]struct{})
	queryTagHashes := make(map[string]struct{})

	if requestBundleHash != nil {
		queryBundleHashes[string(requestBundleHash)] = struct{}{}
	}
	if requestApproveeHash != nil {
		queryApproveeHashes[string(requestApproveeHash)] = struct{}{}
	}
	if requestAddressHash != nil {
		queryAddressHashes[string(requestAddressHash)] = struct{}{}
	}
	if requestTagHash != nil {
		queryTagHashes[string(requestTagHash)] = struct{}{}
	}

	if !tangle.WaitForNodeSynced(waitForNodeSyncedTimeout) {
		return nil, errors.WithMessage(echo.ErrServiceUnavailable, tangle.ErrNodeNotSynced.Error())
	}

	tangle.ReadLockLedger()
	defer tangle.ReadUnlockLedger()

	txHashes := s.findTransactions(maxResults, valueOnly, queryBundleHashes, queryApproveeHashes, queryAddressHashes, queryTagHashes)

	return &transactionsResponse{
		Bundle: func() string {
			if requestBundleHash != nil {
				return requestBundleHash.Trytes()
			}

			return ""
		}(),
		Approvee: func() string {
			if requestApproveeHash != nil {
				return requestApproveeHash.Trytes()
			}

			return ""
		}(),
		Address: func() string {
			if requestAddressHash != nil {
				return requestAddressHash.Trytes()
			}

			return ""
		}(),
		Tag: func() string {
			if requestTagHash != nil {
				return requestTagHash.Trytes()
			}

			return ""
		}(),
		TransactionHashes: txHashes,
		LedgerIndex:       tangle.GetSolidMilestoneIndex(),
	}, nil
}
