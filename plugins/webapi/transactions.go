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

	if (len(query.Bundles) + len(query.Addresses) + len(query.Approvees) + len(query.Tags)) > maxResults {
		e.Error = "Too many bundle, address, approvee or tag hashes. Max. allowed: " + strconv.Itoa(maxResults)
		c.JSON(http.StatusBadRequest, e)
		return
	}

	results := make(map[string]struct{})

	// should return an error in a sane API but unfortunately we need to keep backwards compatibility
	if len(query.Bundles) == 0 && len(query.Addresses) == 0 && len(query.Approvees) == 0 && len(query.Tags) == 0 {
		c.JSON(http.StatusOK, FindTransactionsReturn{Hashes: []string{}})
		return
	}

	// search txs by bundle hash
	resultsByBundleHash := make(map[string]struct{})
	for _, bdl := range query.Bundles {
		if err := trinary.ValidTrytes(bdl); err != nil {
			e.Error = fmt.Sprintf("Bundle hash invalid: %s", bdl)
			c.JSON(http.StatusBadRequest, e)
			return
		}

		for _, r := range tangle.GetBundleTransactionHashes(hornet.HashFromHashTrytes(bdl), true, maxResults-len(results)).Trytes() {
			resultsByBundleHash[r] = struct{}{}
			results[r] = struct{}{}
		}
	}

	// search txs by address
	resultsByAddress := make(map[string]struct{})
	for _, addr := range query.Addresses {
		if err := address.ValidAddress(addr); err != nil {
			e.Error = fmt.Sprintf("address hash invalid: %s", addr)
			c.JSON(http.StatusBadRequest, e)
			return
		}

		if len(addr) == 90 {
			addr = addr[:81]
		}

		for _, r := range tangle.GetTransactionHashesForAddress(hornet.HashFromAddressTrytes(addr), query.ValueOnly, true, maxResults-len(results)).Trytes() {
			resultsByAddress[r] = struct{}{}
			results[r] = struct{}{}
		}
	}

	// search txs by approvees
	resultsByApprovee := make(map[string]struct{})
	for _, approveeHash := range query.Approvees {
		if !guards.IsTransactionHash(approveeHash) {
			e.Error = fmt.Sprintf("Aprovee hash invalid: %s", approveeHash)
			c.JSON(http.StatusBadRequest, e)
			return
		}

		for _, r := range tangle.GetApproverHashes(hornet.HashFromHashTrytes(approveeHash), maxResults-len(results)).Trytes() {
			resultsByApprovee[r] = struct{}{}
			results[r] = struct{}{}
		}
	}

	// search txs by tags
	resultsByTags := make(map[string]struct{})
	for _, tag := range query.Tags {
		if err := trinary.ValidTrytes(tag); err != nil {
			e.Error = fmt.Sprintf("Tag invalid: %s", tag)
			c.JSON(http.StatusBadRequest, e)
			return
		}

		if len(tag) != 27 {
			e.Error = fmt.Sprintf("Tag invalid length: %s", tag)
			c.JSON(http.StatusBadRequest, e)
			return
		}

		for _, r := range tangle.GetTagHashes(hornet.HashFromTagTrytes(tag), true, maxResults-len(results)).Trytes() {
			resultsByTags[r] = struct{}{}
			results[r] = struct{}{}
		}
	}

	// reduce result set to intersection of all result sets
	if len(query.Bundles) > 0 {
		mapIntersection(results, resultsByBundleHash)
	}

	if len(query.Addresses) > 0 {
		mapIntersection(results, resultsByAddress)
	}

	if len(query.Tags) > 0 {
		mapIntersection(results, resultsByTags)
	}

	if len(query.Approvees) > 0 {
		mapIntersection(results, resultsByApprovee)
	}

	// convert to slice
	var j int
	txHashes := make([]string, len(results))
	for r := range results {
		txHashes[j] = r
		j++
	}

	c.JSON(http.StatusOK, FindTransactionsReturn{Hashes: txHashes})
}

// modifies a in-place to be the intersection of the keys of a and b.
func mapIntersection(a map[string]struct{}, b map[string]struct{}) {
	for aKey := range a {
		if _, bHas := b[aKey]; !bHas {
			delete(a, aKey)
		}
	}
}

// redirect to broadcastTransactions
func storeTransactions(i interface{}, c *gin.Context, abortSignal <-chan struct{}) {
	broadcastTransactions(i, c, abortSignal)
}
