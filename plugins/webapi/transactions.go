package webapi

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"

	"github.com/iotaledger/iota.go/address"
	"github.com/iotaledger/iota.go/guards"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/plugins/gossip"
)

func init() {
	addEndpoint("broadcastTransactions", broadcastTransactions, implementedAPIcalls)
	addEndpoint("findTransactions", findTransactions, implementedAPIcalls)
	addEndpoint("storeTransactions", storeTransactions, implementedAPIcalls)
}

func broadcastTransactions(i interface{}, c *gin.Context, _ <-chan struct{}) {
	e := ErrorReturn{}
	query := &BroadcastTransactions{}

	if err := mapstructure.Decode(i, query); err != nil {
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	if len(query.Trytes) == 0 {
		e.Error = "No trytes provided"
		c.JSON(http.StatusBadRequest, e)
		return
	}

	for _, trytes := range query.Trytes {
		if err := trinary.ValidTrytes(trytes); err != nil {
			e.Error = err.Error()
			c.JSON(http.StatusBadRequest, e)
			return
		}
	}

	for _, trytes := range query.Trytes {
		if err := gossip.Processor().ValidateTransactionTrytesAndEmit(trytes); err != nil {
			e.Error = err.Error()
			c.JSON(http.StatusBadRequest, e)
			return
		}
	}
	c.JSON(http.StatusOK, BradcastTransactionsReturn{})
}

func findTransactions(i interface{}, c *gin.Context, _ <-chan struct{}) {
	e := ErrorReturn{}
	query := &FindTransactions{}

	if err := mapstructure.Decode(i, query); err != nil {
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	maxResults := config.NodeConfig.GetInt(config.CfgWebAPILimitsMaxFindTransactions)
	if (query.MaxResults != 0) && (query.MaxResults < maxResults) {
		maxResults = query.MaxResults
	}

	// should return an error in a sane API but unfortunately we need to keep backwards compatibility
	if len(query.Bundles) == 0 && len(query.Addresses) == 0 && len(query.Approvees) == 0 && len(query.Tags) == 0 {
		c.JSON(http.StatusOK, FindTransactionsReturn{Hashes: []string{}})
		return
	}

	if (len(query.Bundles) + len(query.Addresses) + len(query.Approvees) + len(query.Tags)) > maxResults {
		e.Error = "too many bundle, address, approvee or tag hashes. max. allowed: " + strconv.Itoa(maxResults)
		c.JSON(http.StatusBadRequest, e)
		return
	}

	queryBundleHashes := make(map[string]struct{})
	queryApproveeHashes := make(map[string]struct{})
	queryAddressHashes := make(map[string]struct{})
	queryTagHashes := make(map[string]struct{})

	// check all queries first
	for _, bundleTrytes := range query.Bundles {
		if err := trinary.ValidTrytes(bundleTrytes); err != nil {
			e.Error = fmt.Sprintf("bundle hash invalid: %s", bundleTrytes)
			c.JSON(http.StatusBadRequest, e)
			return
		}
		queryBundleHashes[string(hornet.HashFromHashTrytes(bundleTrytes))] = struct{}{}
	}

	for _, approveeTrytes := range query.Approvees {
		if !guards.IsTransactionHash(approveeTrytes) {
			e.Error = fmt.Sprintf("aprovee hash invalid: %s", approveeTrytes)
			c.JSON(http.StatusBadRequest, e)
			return
		}
		queryApproveeHashes[string(hornet.HashFromHashTrytes(approveeTrytes))] = struct{}{}
	}

	for _, addressTrytes := range query.Addresses {
		if err := address.ValidAddress(addressTrytes); err != nil {
			e.Error = fmt.Sprintf("address hash invalid: %s", addressTrytes)
			c.JSON(http.StatusBadRequest, e)
			return
		}
		if len(addressTrytes) == 90 {
			addressTrytes = addressTrytes[:81]
		}
		queryAddressHashes[string(hornet.HashFromHashTrytes(addressTrytes))] = struct{}{}
	}

	for _, tagTrytes := range query.Tags {
		if err := trinary.ValidTrytes(tagTrytes); err != nil {
			e.Error = fmt.Sprintf("tag invalid: %s", tagTrytes)
			c.JSON(http.StatusBadRequest, e)
			return
		}
		if len(tagTrytes) != 27 {
			e.Error = fmt.Sprintf("tag invalid length: %s", tagTrytes)
			c.JSON(http.StatusBadRequest, e)
			return
		}
		queryTagHashes[string(hornet.HashFromTagTrytes(tagTrytes))] = struct{}{}
	}

	results := make(map[string]struct{})
	searchedBefore := false

	// search txs by bundle hash
	for bundleHash := range queryBundleHashes {
		for _, r := range tangle.GetBundleTransactionHashes(hornet.Hash(bundleHash), true, maxResults-len(results)) {
			results[string(r)] = struct{}{}
		}
		searchedBefore = true
	}

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

	if !searchedBefore {
		// search txs by address
		for addressHash := range queryAddressHashes {
			for _, r := range tangle.GetTransactionHashesForAddress(hornet.Hash(addressHash), query.ValueOnly, true, maxResults-len(results)) {
				results[string(r)] = struct{}{}
			}
		}
		searchedBefore = true
	} else {
		// check if results match at least one of the address search criteria
		for txHash := range results {
			contains := false
			for addressHash := range queryAddressHashes {
				if tangle.ContainsAddress(hornet.Hash(addressHash), hornet.Hash(txHash), query.ValueOnly) {
					contains = true
					break
				}
			}
			if !contains {
				delete(results, txHash)
			}
		}
	}

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

	// convert to slice
	var j int
	txHashes := make([]string, len(results))
	for r := range results {
		txHashes[j] = hornet.Hash(r).Trytes()
		j++
	}

	c.JSON(http.StatusOK, FindTransactionsReturn{Hashes: txHashes})
}

// redirect to broadcastTransactions
func storeTransactions(i interface{}, c *gin.Context, abortSignal <-chan struct{}) {
	broadcastTransactions(i, c, abortSignal)
}
