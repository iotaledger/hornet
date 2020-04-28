package webapi

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"

	"github.com/iotaledger/iota.go/guards"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/plugins/tipselection"
)

func init() {
	addEndpoint("getTransactionsToApprove", getTransactionsToApprove, implementedAPIcalls)
}

func getTransactionsToApprove(i interface{}, c *gin.Context, _ <-chan struct{}) {
	e := ErrorReturn{}
	query := &GetTransactionsToApprove{}

	if err := mapstructure.Decode(i, query); err != nil {
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
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
		if err == tipselection.ErrNodeNotSynced {
			e.Error = err.Error()
			c.JSON(http.StatusServiceUnavailable, e)
			return
		}
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	c.JSON(http.StatusOK, GetTransactionsToApproveReturn{TrunkTransaction: tips[0], BranchTransaction: tips[1]})
}
