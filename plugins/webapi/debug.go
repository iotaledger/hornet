package webapi

import (
	"bytes"
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/iota.go/guards"

	"github.com/iotaledger/hornet/pkg/dag"
	"github.com/iotaledger/hornet/pkg/model/hornet"
	"github.com/iotaledger/hornet/pkg/model/tangle"
	"github.com/iotaledger/hornet/plugins/gossip"
	tanglePlugin "github.com/iotaledger/hornet/plugins/tangle"
)

func (s *WebAPIServer) rpcGetRequests(c echo.Context) (interface{}, error) {
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

	return &GetRequestsResponse{Requests: debugReqs}, nil
}

func createConfirmedApproverResult(confirmedTxHash hornet.Hash, path []bool) ([]*ApproverStruct, error) {
	tanglePath := make([]*ApproverStruct, 0)

	txHash := confirmedTxHash
	for len(path) > 0 {
		cachedTxMeta := tangle.GetCachedTxMetadataOrNil(txHash) // meta +1
		if cachedTxMeta == nil {
			return nil, fmt.Errorf("createConfirmedApproverResult: Transaction not found: %v", txHash.Trytes())
		}

		isTrunk := path[len(path)-1]
		path = path[:len(path)-1]

		var nextTxHash hornet.Hash
		if isTrunk {
			nextTxHash = cachedTxMeta.GetMetadata().GetTrunkHash()
		} else {
			nextTxHash = cachedTxMeta.GetMetadata().GetBranchHash()
		}
		cachedTxMeta.Release(true)

		tanglePath = append(tanglePath, &ApproverStruct{TxHash: nextTxHash.Trytes(), ReferencedByTrunk: isTrunk})
		txHash = nextTxHash
	}

	return tanglePath, nil
}

func (s *WebAPIServer) rpcSearchConfirmedApprover(c echo.Context) (interface{}, error) {
	request := &SearchConfirmedApprover{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid request, error: %s", err)
	}

	result := SearchConfirmedApproverResponse{}

	if !guards.IsTransactionHash(request.TxHash) {
		return nil, errors.WithMessagef(echo.ErrBadRequest, "Invalid hash supplied: %s", request.TxHash)
	}

	txsToTraverse := make(map[string][]bool)
	txsToTraverse[string(hornet.HashFromHashTrytes(request.TxHash))] = make([]bool, 0)

	// Collect all tx to check by traversing the tangle
	// Loop as long as new transactions are added in every loop cycle
	for len(txsToTraverse) != 0 {
		for txHash := range txsToTraverse {

			if daemon.IsStopped() {
				return nil, errors.WithMessage(echo.ErrInternalServerError, "operation aborted")
			}

			cachedTxMeta := tangle.GetCachedTxMetadataOrNil(hornet.Hash(txHash)) // meta +1
			if cachedTxMeta == nil {
				delete(txsToTraverse, txHash)
				s.logger.Warnf("searchConfirmedApprover: Transaction not found: %v", hornet.Hash(txHash).Trytes())
				continue
			}

			confirmed, at := cachedTxMeta.GetMetadata().GetConfirmed()
			isTailTx := cachedTxMeta.GetMetadata().IsTail()

			cachedTxMeta.Release(true) // meta -1

			if confirmed {
				resultFound := false

				if request.SearchMilestone {
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
						return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
					}

					result.ConfirmedTxHash = hornet.Hash(txHash).Trytes()
					result.ConfirmedByMilestoneIndex = at
					result.TanglePath = approversResult
					result.TanglePathLength = len(approversResult)

					return result, nil
				}
			}

			approverHashes := tangle.GetApproverHashes(hornet.Hash(txHash))
			for _, approverHash := range approverHashes {

				approverTxMeta := tangle.GetCachedTxMetadataOrNil(approverHash) // meta +1
				if approverTxMeta == nil {
					s.logger.Warnf("searchConfirmedApprover: Approver not found: %v", approverHash.Trytes())
					continue
				}

				txsToTraverse[string(approverHash)] = append(txsToTraverse[txHash], bytes.Equal(approverTxMeta.GetMetadata().GetTrunkHash(), hornet.Hash(txHash)))
				approverTxMeta.Release(true) // meta -1
			}

			delete(txsToTraverse, txHash)
		}
	}

	return nil, errors.WithMessagef(echo.ErrInternalServerError, "No confirmed approver found: %s", request.TxHash)
}

func (s *WebAPIServer) rpcSearchEntryPoints(c echo.Context) (interface{}, error) {
	request := &SearchEntryPoint{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid request, error: %s", err)
	}

	result := &SearchEntryPointResponse{}

	if !guards.IsTransactionHash(request.TxHash) {
		return nil, errors.WithMessagef(echo.ErrBadRequest, "Invalid hash supplied: %s", request.TxHash)
	}

	cachedStartTxMeta := tangle.GetCachedTxMetadataOrNil(hornet.HashFromHashTrytes(request.TxHash)) // meta +1
	if cachedStartTxMeta == nil {
		return nil, errors.WithMessagef(echo.ErrBadRequest, "Start transaction not found: %v", request.TxHash)
	}
	_, startTxConfirmedAt := cachedStartTxMeta.GetMetadata().GetConfirmed()
	defer cachedStartTxMeta.Release(true)

	dag.TraverseApprovees(cachedStartTxMeta.GetMetadata().GetTxHash(),
		// traversal stops if no more transactions pass the given condition
		// Caution: condition func is not in DFS order
		func(cachedTxMeta *tangle.CachedMetadata) (bool, error) { // meta +1
			defer cachedTxMeta.Release(true) // meta -1

			if confirmed, at := cachedTxMeta.GetMetadata().GetConfirmed(); confirmed {
				if (startTxConfirmedAt == 0) || (at < startTxConfirmedAt) {
					result.EntryPoints = append(result.EntryPoints, &EntryPoint{TxHash: cachedTxMeta.GetMetadata().GetTxHash().Trytes(), ConfirmedByMilestoneIndex: at})
					return false, nil
				}
			}

			return true, nil
		},
		// consumer
		func(cachedTxMeta *tangle.CachedMetadata) error { // meta +1
			cachedTxMeta.ConsumeMetadata(func(metadata *hornet.TransactionMetadata) { // meta -1

				result.TanglePath = append(result.TanglePath,
					&TransactionWithApprovers{
						TxHash:            metadata.GetTxHash().Trytes(),
						TrunkTransaction:  metadata.GetTrunkHash().Trytes(),
						BranchTransaction: metadata.GetBranchHash().Trytes(),
					},
				)
			})

			return nil
		},
		// called on missing approvees
		func(approveeHash hornet.Hash) error { return nil },
		// called on solid entry points
		func(txHash hornet.Hash) {
			entryPointIndex, _ := tangle.SolidEntryPointsIndex(txHash)
			result.EntryPoints = append(result.EntryPoints, &EntryPoint{TxHash: txHash.Trytes(), ConfirmedByMilestoneIndex: entryPointIndex})
		}, false, false, nil)

	result.TanglePathLength = len(result.TanglePath)

	if len(result.EntryPoints) == 0 {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "No confirmed approvee found: %s", request.TxHash)
	}

	return result, nil
}

func (s *WebAPIServer) rpcTriggerSolidifier(c echo.Context) (interface{}, error) {
	tanglePlugin.TriggerSolidifier()
	return nil, nil
}

func (s *WebAPIServer) rpcGetFundsOnSpentAddresses(c echo.Context) (interface{}, error) {
	result := &GetFundsOnSpentAddressesResponse{}

	if !tangle.GetSnapshotInfo().IsSpentAddressesEnabled() {
		return nil, errors.WithMessage(echo.ErrBadRequest, "getFundsOnSpentAddresses not available in this node")
	}

	balances, _, err := tangle.GetLedgerStateForLSMI(c.Request().Context())
	if err != nil {
		return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
	}

	for address := range balances {
		if tangle.WasAddressSpentFrom(hornet.Hash(address)) {
			result.Addresses = append(result.Addresses, &AddressWithBalance{Address: hornet.Hash(address).Trytes(), Balance: balances[address]})
		}
	}

	return result, nil
}
