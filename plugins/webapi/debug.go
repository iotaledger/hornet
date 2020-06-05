package webapi

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/mitchellh/mapstructure"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/iota.go/guards"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/plugins/gossip"
	tanglePlugin "github.com/gohornet/hornet/plugins/tangle"
)

func init() {
	addEndpoint("getRequests", getRequests, implementedAPIcalls)
	addEndpoint("searchConfirmedApprover", searchConfirmedApprover, implementedAPIcalls)
	addEndpoint("searchEntryPoints", searchEntryPoints, implementedAPIcalls)
	addEndpoint("triggerSolidifier", triggerSolidifier, implementedAPIcalls)
	addEndpoint("getFundsOnSpentAddresses", getFundsOnSpentAddresses, implementedAPIcalls)
}

func getRequests(_ interface{}, c *gin.Context, _ <-chan struct{}) {
	queued, pending, processing := gossip.RequestQueue().Requests()
	debugReqs := make([]*DebugRequest, len(queued)+len(pending))

	offset := 0
	for i := 0; i < len(queued); i++ {
		req := queued[i]
		debugReqs[offset+i] = &DebugRequest{
			Hash:             req.Hash.Trytes(),
			Type:             "queued",
			TxExists:         tangle.ContainsTransaction(req.Hash),
			MilestoneIndex:   req.MilestoneIndex,
			EnqueueTimestamp: req.EnqueueTime.Unix(),
		}
	}
	offset += len(queued)
	for i := 0; i < len(pending); i++ {
		req := pending[i]
		debugReqs[offset+i] = &DebugRequest{
			Hash:             req.Hash.Trytes(),
			Type:             "pending",
			TxExists:         tangle.ContainsTransaction(req.Hash),
			MilestoneIndex:   req.MilestoneIndex,
			EnqueueTimestamp: req.EnqueueTime.Unix(),
		}
	}
	offset += len(pending)
	for i := 0; i < len(processing); i++ {
		req := processing[i]
		debugReqs[offset+i] = &DebugRequest{
			Hash:             req.Hash.Trytes(),
			Type:             "processing",
			TxExists:         tangle.ContainsTransaction(req.Hash),
			MilestoneIndex:   req.MilestoneIndex,
			EnqueueTimestamp: req.EnqueueTime.Unix(),
		}
	}
	c.JSON(http.StatusOK, GetRequestsReturn{Requests: debugReqs})
}

func createConfirmedApproverResult(confirmedTxHash hornet.Hash, path []bool) ([]*ApproverStruct, error) {

	tanglePath := make([]*ApproverStruct, 0)

	txHash := confirmedTxHash
	for len(path) > 0 {
		cachedTx := tangle.GetCachedTransactionOrNil(txHash) // tx +1
		if cachedTx == nil {
			return nil, fmt.Errorf("createConfirmedApproverResult: Transaction not found: %v", txHash.Trytes())
		}

		isTrunk := path[len(path)-1]
		path = path[:len(path)-1]

		var nextTxHash hornet.Hash
		if isTrunk {
			nextTxHash = cachedTx.GetTransaction().GetTrunkHash()
		} else {
			nextTxHash = cachedTx.GetTransaction().GetBranchHash()
		}
		cachedTx.Release(true)

		tanglePath = append(tanglePath, &ApproverStruct{TxHash: nextTxHash.Trytes(), ReferencedByTrunk: isTrunk})
		txHash = nextTxHash
	}

	return tanglePath, nil
}

func searchConfirmedApprover(i interface{}, c *gin.Context, _ <-chan struct{}) {
	e := ErrorReturn{}
	query := &SearchConfirmedApprover{}
	result := SearchConfirmedApproverReturn{}

	if err := mapstructure.Decode(i, query); err != nil {
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	if !guards.IsTransactionHash(query.TxHash) {
		e.Error = fmt.Sprintf("Invalid hash supplied: %s", query.TxHash)
		c.JSON(http.StatusBadRequest, e)
		return
	}

	txsToTraverse := make(map[string][]bool)
	txsToTraverse[string(hornet.Hash(trinary.MustTrytesToBytes(query.TxHash)[:49]))] = make([]bool, 0)

	// Collect all tx to check by traversing the tangle
	// Loop as long as new transactions are added in every loop cycle
	for len(txsToTraverse) != 0 {
		for txHash := range txsToTraverse {

			if daemon.IsStopped() {
				e.Error = "operation aborted"
				c.JSON(http.StatusInternalServerError, e)
				return
			}

			cachedTx := tangle.GetCachedTransactionOrNil(hornet.Hash(txHash)) // tx +1
			if cachedTx == nil {
				delete(txsToTraverse, txHash)
				log.Warnf("searchConfirmedApprover: Transaction not found: %v", hornet.Hash(txHash).Trytes())
				continue
			}

			confirmed, at := cachedTx.GetMetadata().GetConfirmed()
			isTailTx := cachedTx.GetTransaction().IsTail()

			cachedTx.Release(true) // tx -1

			if confirmed {
				resultFound := false

				if query.SearchMilestone {
					if isTailTx {
						// Check if the bundle is a milestone, otherwise go on
						cachedBndl := tangle.GetCachedBundleOrNil(hornet.Hash(txHash))
						if cachedBndl != nil {
							if cachedBndl.GetBundle().IsMilestone() {
								resultFound = true
							}
							cachedBndl.Release(true)
						}
					}
				} else {
					resultFound = true
				}

				if resultFound {
					approversResult, err := createConfirmedApproverResult(hornet.Hash(txHash), txsToTraverse[txHash])
					if err != nil {
						e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
						c.JSON(http.StatusInternalServerError, e)
						return
					}

					result.ConfirmedTxHash = hornet.Hash(txHash).Trytes()
					result.ConfirmedByMilestoneIndex = at
					result.TanglePath = approversResult
					result.TanglePathLength = len(approversResult)

					c.JSON(http.StatusOK, result)
					return
				}
			}

			approverHashes := tangle.GetApproverHashes(hornet.Hash(txHash), true)
			for _, approverHash := range approverHashes {

				approverTx := tangle.GetCachedTransactionOrNil(approverHash) // tx +1
				if approverTx == nil {
					log.Warnf("searchConfirmedApprover: Approver not found: %v", approverHash.Trytes())
					continue
				}

				txsToTraverse[string(approverHash)] = append(txsToTraverse[txHash], bytes.Equal(approverTx.GetTransaction().GetTrunkHash(), hornet.Hash(txHash)))
				approverTx.Release(true) // tx -1
			}

			delete(txsToTraverse, txHash)
		}
	}

	e.Error = fmt.Sprintf("No confirmed approver found: %s", query.TxHash)
	c.JSON(http.StatusInternalServerError, e)
}

func searchEntryPoints(i interface{}, c *gin.Context, _ <-chan struct{}) {
	e := ErrorReturn{}
	query := &SearchEntryPoint{}
	result := &SearchEntryPointReturn{}

	if err := mapstructure.Decode(i, query); err != nil {
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	if !guards.IsTransactionHash(query.TxHash) {
		e.Error = fmt.Sprintf("Invalid hash supplied: %s", query.TxHash)
		c.JSON(http.StatusBadRequest, e)
		return
	}

	cachedStartTx := tangle.GetCachedTransactionOrNil(hornet.Hash(trinary.MustTrytesToBytes(query.TxHash)[:49])) // tx +1
	if cachedStartTx == nil {
		e.Error = fmt.Sprintf("Start transaction not found: %v", query.TxHash)
		c.JSON(http.StatusBadRequest, e)
		return
	}
	_, startTxConfirmedAt := cachedStartTx.GetMetadata().GetConfirmed()
	defer cachedStartTx.Release(true)

	if !tangle.SolidEntryPointsContain(cachedStartTx.GetTransaction().GetTxHash()) {

		dag.TraverseApprovees(cachedStartTx.GetTransaction().GetTxHash(),
			// predicate
			func(cachedTx *tangle.CachedTransaction) bool { // tx +1
				defer cachedTx.Release(true) // tx -1

				if tangle.SolidEntryPointsContain(cachedTx.GetTransaction().GetTxHash()) {
					result.EntryPoints = append(result.EntryPoints, &EntryPoint{TxHash: cachedTx.GetTransaction().GetTxHash().Trytes(), ConfirmedByMilestoneIndex: 0})
					return false
				}

				if confirmed, at := cachedTx.GetMetadata().GetConfirmed(); confirmed {
					if (startTxConfirmedAt == 0) || (at < startTxConfirmedAt) {
						result.EntryPoints = append(result.EntryPoints, &EntryPoint{TxHash: cachedTx.GetTransaction().GetTxHash().Trytes(), ConfirmedByMilestoneIndex: at})
						return false
					}
				}

				return true
			},

			// consumer
			func(cachedTx *tangle.CachedTransaction) { // tx +1
				defer cachedTx.Release(true) // tx -1

				result.TanglePath = append(result.TanglePath,
					&TransactionWithApprovers{
						TxHash:            cachedTx.GetTransaction().GetTxHash().Trytes(),
						TrunkTransaction:  cachedTx.GetTransaction().GetTrunkHash().Trytes(),
						BranchTransaction: cachedTx.GetTransaction().GetBranchHash().Trytes(),
					},
				)
			},
			// called on missing approvees
			func(approveeHash hornet.Hash) {}, true)

	} else {
		result.EntryPoints = append(result.EntryPoints, &EntryPoint{TxHash: cachedStartTx.GetTransaction().GetTxHash().Trytes(), ConfirmedByMilestoneIndex: 0})
	}

	result.TanglePathLength = len(result.TanglePath)

	if len(result.EntryPoints) == 0 {
		e.Error = fmt.Sprintf("No confirmed approvee found: %s", query.TxHash)
		c.JSON(http.StatusInternalServerError, e)
		return
	}
	c.JSON(http.StatusOK, result)
}

func triggerSolidifier(i interface{}, c *gin.Context, _ <-chan struct{}) {
	tanglePlugin.TriggerSolidifier()
	c.Status(http.StatusAccepted)
}

func getFundsOnSpentAddresses(i interface{}, c *gin.Context, _ <-chan struct{}) {
	e := ErrorReturn{}
	result := &GetFundsOnSpentAddressesReturn{}

	if !tangle.GetSnapshotInfo().IsSpentAddressesEnabled() {
		e.Error = "getFundsOnSpentAddresses not available in this node"
		c.JSON(http.StatusBadRequest, e)
		return
	}

	balances, _, err := tangle.GetLedgerStateForLSMI(nil)
	if err != nil {
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	for address := range balances {
		if tangle.WasAddressSpentFrom(hornet.Hash(address)) {
			result.Addresses = append(result.Addresses, &AddressWithBalance{Address: hornet.Hash(address).Trytes(), Balance: balances[address]})
		}
	}

	c.JSON(http.StatusOK, result)
}
