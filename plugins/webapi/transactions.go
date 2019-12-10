package webapi

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/iotaledger/hive.go/parameter"
	"github.com/iotaledger/iota.go/address"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/mitchellh/mapstructure"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/plugins/gossip"
)

func init() {
	addEndpoint("broadcastTransactions", broadcastTransactions, implementedAPIcalls)
	addEndpoint("findTransactions", findTransactions, implementedAPIcalls)
	addEndpoint("storeTransactions", storeTransactions, implementedAPIcalls)
}

func broadcastTransactions(i interface{}, c *gin.Context) {

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

func findTransactions(i interface{}, c *gin.Context) {
	ft := &FindTransactions{}
	e := ErrorReturn{}

	maxFindTransactions := parameter.NodeConfig.GetInt("api.maxFindTransactions")

	err := mapstructure.Decode(i, ft)
	if err != nil {
		e.Error = "Internal error"
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	if len(ft.Bundles) > maxFindTransactions || len(ft.Addresses) > maxFindTransactions {
		e.Error = "Too many transaction or bundle hashes. Max. allowed: " + strconv.Itoa(maxFindTransactions)
		c.JSON(http.StatusBadRequest, e)
		return
	}

	txHashes := []string{}

	if len(ft.Bundles) == 0 && len(ft.Addresses) == 0 {
		e.Error = "Nothing to search for"
		c.JSON(http.StatusBadRequest, e)
		return
	}

	// Searching for transactions that contains the given bundle hash
	for _, bdl := range ft.Bundles {
		if err := trinary.ValidTrytes(bdl); err != nil {
			e.Error = fmt.Sprintf("Bundle hash invalid: %s", bdl)
			c.JSON(http.StatusBadRequest, e)
			return
		}
		bundleBucket, err := tangle.GetBundleBucket(bdl)
		if err != nil {
			e.Error = "Internal error"
			c.JSON(http.StatusInternalServerError, e)
			return
		}
		for _, txHash := range bundleBucket.TransactionHashes() {
			txHashes = append(txHashes, txHash)
		}
	}

	// Searching for transactions that contains the given address
	for _, addr := range ft.Addresses {
		err := address.ValidAddress(addr)
		if err == nil {
			if len(addr) == 90 {
				addr = addr[:81]
			}
			tx, err := tangle.ReadTransactionHashesForAddressFromDatabase(addr, maxFindTransactions)
			if err != nil {
				e.Error = "Internal error"
				c.JSON(http.StatusInternalServerError, e)
				return
			}
			txHashes = append(txHashes, tx...)
		}
	}

	c.JSON(http.StatusOK, FindTransactionsReturn{Hashes: txHashes})
}

// redirect to broadcastTransactions
func storeTransactions(i interface{}, c *gin.Context) {
	broadcastTransactions(i, c)
}
