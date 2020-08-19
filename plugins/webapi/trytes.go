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

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

func init() {
	addEndpoint("getTrytes", getTrytes, implementedAPIcalls)
}

func getTrytes(i interface{}, c *gin.Context, _ <-chan struct{}) {
	e := ErrorReturn{}
	query := &GetTrytes{}

	maxGetTrytes := config.NodeConfig.GetInt(config.CfgWebAPILimitsMaxGetTrytes)

	if err := mapstructure.Decode(i, query); err != nil {
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	if len(query.Hashes) > maxGetTrytes {
		e.Error = "Too many hashes. Max. allowed: " + strconv.Itoa(maxGetTrytes)
		c.JSON(http.StatusBadRequest, e)
		return
	}

	trytes := []string{}

	for _, hash := range query.Hashes {
		if !guards.IsTransactionHash(hash) {
			e.Error = fmt.Sprintf("Invalid hash supplied: %s", hash)
			c.JSON(http.StatusBadRequest, e)
			return
		}
	}

	for _, hash := range query.Hashes {

		cachedTx := tangle.GetCachedTransactionOrNil(hornet.HashFromHashTrytes(hash)) // tx +1

		if cachedTx == nil {
			trytes = append(trytes, strings.Repeat("9", 2673))
			continue
		}

		tx, err := transaction.TransactionToTrytes(cachedTx.GetTransaction().Tx)
		if err != nil {
			e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
			c.JSON(http.StatusInternalServerError, e)
			cachedTx.Release(true) // tx -1
			return
		}

		trytes = append(trytes, tx)
		cachedTx.Release(true) // tx -1
	}

	c.JSON(http.StatusOK, GetTrytesReturn{Trytes: trytes})
}
