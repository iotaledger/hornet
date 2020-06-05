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
		err := gossip.Processor().ValidateTransactionTrytesAndEmit(trytes)
		if err != nil {
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

	txHashes := []string{}

	if len(query.Bundles) == 0 && len(query.Addresses) == 0 && len(query.Approvees) == 0 && len(query.Tags) == 0 {
		c.JSON(http.StatusOK, FindTransactionsReturn{Hashes: []string{}})
		return
	}

	// Searching for transactions that contains the given bundle hash
	for _, bdl := range query.Bundles {
		if err := trinary.ValidTrytes(bdl); err != nil {
			e.Error = fmt.Sprintf("Bundle hash invalid: %s", bdl)
			c.JSON(http.StatusBadRequest, e)
			return
		}

		txHashes = append(txHashes, tangle.GetBundleTransactionHashes(hornet.Hash(trinary.MustTrytesToBytes(bdl)[:49]), true, maxResults-len(txHashes)).Trytes()...)
	}

	// Searching for transactions that contains the given address
	for _, addr := range query.Addresses {
		if err := address.ValidAddress(addr); err != nil {
			e.Error = fmt.Sprintf("address hash invalid: %s", addr)
			c.JSON(http.StatusBadRequest, e)
			return
		}

		if len(addr) == 90 {
			addr = addr[:81]
		}

		txHashes = append(txHashes, tangle.GetTransactionHashesForAddress(hornet.Hash(trinary.MustTrytesToBytes(addr)[:49]), query.ValueOnly, true, maxResults-len(txHashes)).Trytes()...)
	}

	// Searching for all approvers of the given transactions
	for _, approveeHash := range query.Approvees {
		if !guards.IsTransactionHash(approveeHash) {
			e.Error = fmt.Sprintf("Aprovee hash invalid: %s", approveeHash)
			c.JSON(http.StatusBadRequest, e)
			return
		}

		txHashes = append(txHashes, tangle.GetApproverHashes(hornet.Hash(trinary.MustTrytesToBytes(approveeHash)[:49]), true, maxResults-len(txHashes)).Trytes()...)
	}

	// Searching for transactions that contain the given tag
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

		txHashes = append(txHashes, tangle.GetTagHashes(hornet.Hash(trinary.MustTrytesToBytes(tag)[:17]), true, maxResults-len(txHashes)).Trytes()...)
	}

	c.JSON(http.StatusOK, FindTransactionsReturn{Hashes: txHashes})
}

// redirect to broadcastTransactions
func storeTransactions(i interface{}, c *gin.Context, abortSignal <-chan struct{}) {
	broadcastTransactions(i, c, abortSignal)
}
