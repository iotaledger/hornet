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

	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/parameter"
	"github.com/gohornet/hornet/plugins/gossip"
)

func init() {
	addEndpoint("broadcastTransactions", broadcastTransactions, implementedAPIcalls)
	addEndpoint("findTransactions", findTransactions, implementedAPIcalls)
	addEndpoint("storeTransactions", storeTransactions, implementedAPIcalls)
}

func broadcastTransactions(i interface{}, c *gin.Context, abortSignal <-chan struct{}) {

	bt := &BroadcastTransactions{}
	e := ErrorReturn{}

	err := mapstructure.Decode(i, bt)
	if err != nil {
		e.Error = "Internal error"
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	if len(bt.Trytes) == 0 {
		e.Error = "No trytes provided"
		c.JSON(http.StatusBadRequest, e)
		return
	}

	for _, trytes := range bt.Trytes {
		if err := trinary.ValidTrytes(trytes); err != nil {
			e.Error = "Trytes invalid"
			c.JSON(http.StatusBadRequest, e)
			return
		}
	}

	for _, trytes := range bt.Trytes {
		err = gossip.BroadcastTransactionFromAPI(trytes)
		if err != nil {
			e.Error = err.Error()
			c.JSON(http.StatusBadRequest, e)
			return
		}
	}
	c.JSON(http.StatusOK, BradcastTransactionsReturn{})
}

func findTransactions(i interface{}, c *gin.Context, abortSignal <-chan struct{}) {
	ft := &FindTransactions{}
	e := ErrorReturn{}

	maxResults := parameter.NodeConfig.GetInt("api.maxFindTransactions")
	if (ft.MaxResults != 0) && (ft.MaxResults < maxResults) {
		maxResults = ft.MaxResults
	}

	err := mapstructure.Decode(i, ft)
	if err != nil {
		e.Error = "Internal error"
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	if (len(ft.Bundles) + len(ft.Addresses) + len(ft.Approvees) + len(ft.Tags)) > maxResults {
		e.Error = "Too many bundle, address, approvee or tag hashes. Max. allowed: " + strconv.Itoa(maxResults)
		c.JSON(http.StatusBadRequest, e)
		return
	}

	txHashes := []string{}

	if len(ft.Bundles) == 0 && len(ft.Addresses) == 0 && len(ft.Approvees) == 0 && len(ft.Tags) == 0 {
		c.JSON(http.StatusOK, FindTransactionsReturn{Hashes: []string{}})
		return
	}

	// Searching for transactions that contains the given bundle hash
	for _, bdl := range ft.Bundles {
		if err := trinary.ValidTrytes(bdl); err != nil {
			e.Error = fmt.Sprintf("Bundle hash invalid: %s", bdl)
			c.JSON(http.StatusBadRequest, e)
			return
		}

		txHashes = append(txHashes, tangle.GetBundleTransactionHashes(bdl, maxResults-len(txHashes))...)
	}

	// Searching for transactions that contains the given address
	for _, addr := range ft.Addresses {
		if err := address.ValidAddress(addr); err != nil {
			e.Error = fmt.Sprintf("address hash invalid: %s", addr)
			c.JSON(http.StatusBadRequest, e)
			return
		}

		if len(addr) == 90 {
			addr = addr[:81]
		}

		txHashes = append(txHashes, tangle.GetTransactionHashesForAddress(addr, maxResults-len(txHashes))...)
	}

	// Searching for all approovers of the given transactions
	for _, approveeHash := range ft.Approvees {
		if !guards.IsTransactionHash(approveeHash) {
			e.Error = fmt.Sprintf("Aprovee hash invalid: %s", approveeHash)
			c.JSON(http.StatusBadRequest, e)
			return
		}

		txHashes = append(txHashes, tangle.GetApproverHashes(approveeHash, maxResults-len(txHashes))...)
	}

	// Searching for transactions that contain the given tag
	for _, tag := range ft.Tags {
		if err := trinary.ValidTrytes(tag); err != nil {
			e.Error = fmt.Sprintf("Tag invalid: %s", tag)
			c.JSON(http.StatusBadRequest, e)
			return
		}

		txHashes = append(txHashes, tangle.GetTagHashes(tag, maxResults-len(txHashes))...)
	}

	c.JSON(http.StatusOK, FindTransactionsReturn{Hashes: txHashes})
}

// redirect to broadcastTransactions
func storeTransactions(i interface{}, c *gin.Context, abortSignal <-chan struct{}) {
	broadcastTransactions(i, c, abortSignal)
}
