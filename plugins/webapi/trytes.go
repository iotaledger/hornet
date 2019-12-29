package webapi

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
    "github.com/mitchellh/mapstructure"

	"github.com/iotaledger/iota.go/guards"
	"github.com/iotaledger/iota.go/transaction"

    "github.com/gohornet/hornet/packages/model/tangle"
    "github.com/iotaledger/hive.go/parameter"
)

func init() {
	addEndpoint("getTrytes", getTrytes, implementedAPIcalls)
}

func getTrytes(i interface{}, c *gin.Context) {

	maxGetTrytes := parameter.NodeConfig.GetInt("api.maxGetTrytes")

	gt := &GetTrytes{}
	e := ErrorReturn{}
	err := mapstructure.Decode(i, gt)
	if err != nil {
		e.Error = "Internal error"
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	if len(gt.Hashes) > maxGetTrytes {
		e.Error = "Too many hashes. Max. allowed: " + strconv.Itoa(maxGetTrytes)
		c.JSON(http.StatusBadRequest, e)
		return
	}

	trytes := []string{}

	for _, hash := range gt.Hashes {
		if !guards.IsTransactionHash(hash) {
			e.Error = fmt.Sprintf("Invalid hash supplied: %s", hash)
			c.JSON(http.StatusBadRequest, e)
			return
		}
	}

	for _, hash := range gt.Hashes {
		cachedTx, err := tangle.GetCachedTransaction(hash)
		if err != nil {
			e.Error = "Internal error"
			c.JSON(http.StatusInternalServerError, e)
			return
		}

		if cachedTx.Exists() {
			tx, err := transaction.TransactionToTrytes(cachedTx.GetTransaction().Tx)
			if err != nil {
				e.Error = "Internal error"
				c.JSON(http.StatusInternalServerError, e)
				return
			}
			trytes = append(trytes, tx)
		} else {
			trytes = append(trytes, strings.Repeat("9", 2673))
		}
		cachedTx.Release()
	}

	c.JSON(http.StatusOK, GetTrytesReturn{Trytes: trytes})
}
