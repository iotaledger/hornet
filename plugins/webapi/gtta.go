package webapi

import (
	"github.com/gin-gonic/gin"
	"github.com/iotaledger/iota.go/guards"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/mitchellh/mapstructure"
	"github.com/gohornet/hornet/plugins/tipselection"
	"net/http"
)

func init() {
	addEndpoint("getTransactionsToApprove", getTransactionsToApprove, implementedAPIcalls)
}

func getTransactionsToApprove(i interface{}, c *gin.Context) {
	e := ErrorReturn{}
	query := &GetTransactionsToApprove{}
	result := GetTransactionsToApproveReturn{}

	err := mapstructure.Decode(i, query)
	if err != nil {
		e.Error = "Internal error"
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	var reference *trinary.Hash
	if len(query.Reference) > 0 {
		if !guards.IsTransactionHash(query.Reference) {
			e.Error = "Invalid reference hash supplied"
			c.JSON(http.StatusBadRequest, e)
			return
		}
		reference = &query.Reference
	}

	tips, _, err := tipselection.SelectTips(query.Depth, reference)
	if err != nil {
		e.Error = err.Error()
		if err == tipselection.ErrNodeNotSynced {
			c.JSON(http.StatusServiceUnavailable, e)
			return
		}
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	result.TrunkTransaction = tips[0]
	result.BranchTransaction = tips[1]
	c.JSON(http.StatusOK, result)
}
