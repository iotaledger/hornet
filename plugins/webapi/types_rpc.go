package webapi

import (
	"github.com/iotaledger/hornet/pkg/model/milestone"
	"github.com/iotaledger/hornet/pkg/peering/peer"

	"github.com/iotaledger/iota.go/trinary"
)

// Request struct.
type Request struct {
	Command string `json:"command"`
}

// ErrorReturn struct.
type ErrorReturn struct {
	Error string `json:"error"`
}

///////////////////// getBalances /////////////////////////////////

// GetBalances struct.
type GetBalances struct {
	Addresses []trinary.Hash `json:"addresses"`
}

// GetBalancesResponse struct.
type GetBalancesResponse struct {
	Balances       []uint64        `json:"balances"`
	References     []trinary.Hash  `json:"references"`
	MilestoneIndex milestone.Index `json:"milestoneIndex"`
	Duration       int             `json:"duration"`
}

/////////////////// checkConsistency //////////////////////////////

// CheckConsistencyResponse struct
type CheckConsistencyResponse struct {
	State    bool `json:"state"`
	Duration int  `json:"duration"`
}

///////////////////// getRequests /////////////////////////////////

// GetRequestsResponse struct.
type GetRequestsResponse struct {
	Requests []*DebugRequest `json:"requests"`
}

type DebugRequest struct {
	Hash             trinary.Hash    `json:"hash"`
	Type             string          `json:"type"`
	TxExists         bool            `json:"txExists"`
	EnqueueTimestamp int64           `json:"enqueueTime"`
	MilestoneIndex   milestone.Index `json:"milestoneIndex"`
}

///////////////// searchConfirmedApprover /////////////////////////

// SearchConfirmedApprover struct.
type SearchConfirmedApprover struct {
	TxHash          trinary.Hash `json:"txHash"`
	SearchMilestone bool         `json:"searchMilestone"`
}

// ApproverStruct struct.
type ApproverStruct struct {
	TxHash            trinary.Hash `json:"txHash"`
	ReferencedByTrunk bool         `json:"referencedByTrunk"`
}

// SearchConfirmedApproverResponse struct.
type SearchConfirmedApproverResponse struct {
	ConfirmedTxHash           trinary.Hash      `json:"confirmedTxHash"`
	ConfirmedByMilestoneIndex milestone.Index   `json:"confirmedByMilestoneIndex"`
	TanglePath                []*ApproverStruct `json:"tanglePath"`
	TanglePathLength          int               `json:"tanglePathLength"`
}

///////////////// searchEntryPoints /////////////////////////

// SearchEntryPoint struct.
type SearchEntryPoint struct {
	TxHash trinary.Hash `json:"txHash"`
}

// EntryPoint struct.
type EntryPoint struct {
	TxHash                    trinary.Hash    `json:"txHash"`
	ConfirmedByMilestoneIndex milestone.Index `json:"confirmedByMilestoneIndex"`
}

type TransactionWithApprovers struct {
	TxHash            trinary.Hash `json:"txHash"`
	TrunkTransaction  trinary.Hash `json:"trunkTransaction"`
	BranchTransaction trinary.Hash `json:"branchTransaction"`
}

// SearchEntryPointResponse struct.
type SearchEntryPointResponse struct {
	TanglePath       []*TransactionWithApprovers `json:"tanglePath"`
	EntryPoints      []*EntryPoint               `json:"entryPoints"`
	TanglePathLength int                         `json:"tanglePathLength"`
}

/////////////////// getFundsOnSpentAddresses //////////////////////////////

// GetFundsOnSpentAddressesResponse struct.
type GetFundsOnSpentAddressesResponse struct {
	Addresses []*AddressWithBalance `json:"addresses"`
}

type AddressWithBalance struct {
	Address trinary.Hash `json:"address"`
	Balance uint64       `json:"balance"`
}

/////////////////// getInclusionStates ////////////////////////////

// GetInclusionStates struct.
type GetInclusionStates struct {
	Transactions []trinary.Hash `json:"transactions"`
}

// GetInclusionStatesResponse struct.
type GetInclusionStatesResponse struct {
	States   []bool `json:"states"`
	Duration int    `json:"duration"`
}

/////////////////////// getNodeInfo ///////////////////////////////

// GetNodeInfoResponse struct.
type GetNodeInfoResponse struct {
	AppName                            string          `json:"appName"`
	AppVersion                         string          `json:"appVersion"`
	NodeAlias                          string          `json:"nodeAlias,omitempty"`
	LatestMilestone                    trinary.Hash    `json:"latestMilestone"`
	LatestMilestoneIndex               milestone.Index `json:"latestMilestoneIndex"`
	LatestSolidSubtangleMilestone      trinary.Hash    `json:"latestSolidSubtangleMilestone"`
	LatestSolidSubtangleMilestoneIndex milestone.Index `json:"latestSolidSubtangleMilestoneIndex"`
	IsSynced                           bool            `json:"isSynced"`
	Health                             bool            `json:"isHealthy"`
	MilestoneStartIndex                milestone.Index `json:"milestoneStartIndex"`
	LastSnapshottedMilestoneIndex      milestone.Index `json:"lastSnapshottedMilestoneIndex"`
	Neighbors                          uint            `json:"neighbors"`
	Time                               int64           `json:"time"`
	Tips                               uint32          `json:"tips"`
	TransactionsToRequest              int             `json:"transactionsToRequest"`
	Features                           []string        `json:"features"`
	CoordinatorAddress                 trinary.Hash    `json:"coordinatorAddress"`
	Duration                           int             `json:"duration"`
}

/////////////////////// getNodeAPIConfiguration ///////////////////////////////

// GetNodeAPIConfigurationResponse struct.
type GetNodeAPIConfigurationResponse struct {
	MaxFindTransactions int             `json:"maxFindTransactions"`
	MaxRequestsList     int             `json:"maxRequestsList"`
	MaxGetTrytes        int             `json:"maxGetTrytes"`
	MaxBodyLength       int             `json:"maxBodyLength"`
	MilestoneStartIndex milestone.Index `json:"milestoneStartIndex"`
	Duration            int             `json:"duration"`
}

/////////////////// getLedgerDiff ////////////////////////

// GetLedgerDiff struct.
type GetLedgerDiff struct {
	MilestoneIndex milestone.Index `json:"milestoneIndex"`
}

// GetLedgerDiffResponse struct.
type GetLedgerDiffResponse struct {
	Diff           map[trinary.Hash]int64 `json:"diff"`
	MilestoneIndex milestone.Index        `json:"milestoneIndex"`
	Duration       int                    `json:"duration"`
}

/////////////////// getLedgerDiffExt ////////////////////////

// GetLedgerDiffExt struct.
type GetLedgerDiffExt struct {
	MilestoneIndex milestone.Index `json:"milestoneIndex"`
}

// TxHashWithValue struct.
type TxHashWithValue struct {
	TxHash     trinary.Hash `json:"txHash"`
	TailTxHash trinary.Hash `json:"tailTxHash"`
	BundleHash trinary.Hash `json:"bundleHash"`
	Address    trinary.Hash `json:"address"`
	Value      int64        `json:"value"`
}

func (tx *TxHashWithValue) Item() Container {
	return tx
}

// TxWithValue struct.
type TxWithValue struct {
	TxHash  trinary.Hash `json:"txHash"`
	Address trinary.Hash `json:"address"`
	Index   uint64       `json:"index"`
	Value   int64        `json:"value"`
}

func (tx *TxWithValue) Item() Container {
	return tx
}

// BundleWithValue struct.
type BundleWithValue struct {
	BundleHash trinary.Hash   `json:"bundleHash"`
	TailTxHash trinary.Hash   `json:"tailTxHash"`
	LastIndex  uint64         `json:"lastIndex"`
	Txs        []*TxWithValue `json:"txs"`
}

func (b *BundleWithValue) Item() Container {
	return b
}

// GetLedgerDiffExtResponse struct.
type GetLedgerDiffExtResponse struct {
	ConfirmedTxWithValue      []*TxHashWithValue     `json:"confirmedTxWithValue"`
	ConfirmedBundlesWithValue []*BundleWithValue     `json:"confirmedBundlesWithValue"`
	Diff                      map[trinary.Hash]int64 `json:"diff"`
	MilestoneIndex            milestone.Index        `json:"milestoneIndex"`
	Duration                  int                    `json:"duration"`
}

/////////////////// getLedgerState ////////////////////////

// GetLedgerState struct.
type GetLedgerState struct {
	TargetIndex milestone.Index `json:"targetIndex,omitempty"`
}

// GetLedgerStateResponse struct.
type GetLedgerStateResponse struct {
	Balances       map[trinary.Hash]uint64 `json:"balances"`
	MilestoneIndex milestone.Index         `json:"milestoneIndex"`
	Duration       int                     `json:"duration"`
}

//////////////////// addNeighbors /////////////////////////////////

// AddNeighbors legacy struct.
type AddNeighbors struct {
	Uris []string `json:"uris"`
}

// AddNeighborsHornet struct.
type AddNeighborsHornet struct {
	Neighbors []Neighbor `json:"neighbors"`
}

// Neighbor struct.
type Neighbor struct {
	Identity   string `json:"identity"`
	Alias      string `json:"alias"`
	PreferIPv6 bool   `json:"prefer_ipv6"`
}

// AddNeighborsResponse struct.
type AddNeighborsResponse struct {
	AddedNeighbors int `json:"addedNeighbors"`
	Duration       int `json:"duration"`
}

///////////////////// removeNeighbors /////////////////////////////

// RemoveNeighbors struct.
type RemoveNeighbors struct {
	Uris []string `json:"uris"`
}

// RemoveNeighborsResponse struct.
type RemoveNeighborsResponse struct {
	RemovedNeighbors uint `json:"removedNeighbors"`
	Duration         int  `json:"duration"`
}

////////////////////// getNeighbors ///////////////////////////////

// GetNeighborsResponse struct.
type GetNeighborsResponse struct {
	Neighbors []*peer.Info `json:"neighbors"`
	Duration  int          `json:"duration"`
}

//////////////////// attachToTangle ///////////////////////////////

// AttachToTangle struct.
type AttachToTangle struct {
	TrunkTransaction   trinary.Hash     `json:"trunkTransaction"`
	BranchTransaction  trinary.Hash     `json:"branchTransaction"`
	MinWeightMagnitude int              `json:"minWeightMagnitude,omitempty"`
	Trytes             []trinary.Trytes `json:"trytes"`
}

// AttachToTangleResponse struct.
type AttachToTangleResponse struct {
	Trytes   []trinary.Trytes `json:"trytes"`
	Duration int              `json:"duration"`
}

/////////////////// pruneDatabase ////////////////////////

// PruneDatabase struct.
type PruneDatabase struct {
	TargetIndex milestone.Index `json:"targetIndex"`
	Depth       milestone.Index `json:"depth"`
}

// PruneDatabaseResponse struct.
type PruneDatabaseResponse struct {
	Duration int `json:"duration"`
}

/////////////////// createSnapshotFile ////////////////////////

// CreateSnapshotFile struct.
type CreateSnapshotFile struct {
	TargetIndex milestone.Index `json:"targetIndex"`
}

// CreateSnapshotFileResponse struct.
type CreateSnapshotFileResponse struct {
	Duration int `json:"duration"`
}

/////////////////// wereAddressesSpentFrom ////////////////////////

// WereAddressesSpentFrom struct.
type WereAddressesSpentFrom struct {
	Addresses []trinary.Hash `json:"addresses"`
}

// WereAddressesSpentFromResponse struct.
type WereAddressesSpentFromResponse struct {
	States   []bool `json:"states"`
	Duration int    `json:"duration"`
}

///////////////// getTipInfo ////////////////////////

// GetTipInfo struct.
type GetTipInfo struct {
	TailTransaction trinary.Hash `json:"tailTransaction"`
}

// GetTipInfoResponse struct.
type GetTipInfoResponse struct {
	Confirmed      bool `json:"confirmed"`
	Conflicting    bool `json:"conflicting"`
	ShouldPromote  bool `json:"shouldPromote"`
	ShouldReattach bool `json:"shouldReattach"`
	Duration       int  `json:"duration"`
}

///////////////// getTransactionsToApprove ////////////////////////

// GetTransactionsToApprove struct.
type GetTransactionsToApprove struct {
	Depth     uint         `json:"depth"`
	Reference trinary.Hash `json:"reference"`
}

// GetTransactionsToApproveResponse struct.
type GetTransactionsToApproveResponse struct {
	TrunkTransaction  trinary.Hash `json:"trunkTransaction"`
	BranchTransaction trinary.Hash `json:"branchTransaction"`
	Duration          int          `json:"duration"`
}

////////////////// broadcastTransactions //////////////////////////

// BroadcastTransactions struct.
type BroadcastTransactions struct {
	Trytes []trinary.Trytes `json:"trytes"`
}

// BradcastTransactionsResponse struct.
type BradcastTransactionsResponse struct {
	Duration int `json:"duration"`
}

/////////////////// findTransactions //////////////////////////////

// FindTransactions struct.
type FindTransactions struct {
	Bundles    []trinary.Hash `json:"bundles"`
	Addresses  []trinary.Hash `json:"addresses"`
	Tags       []trinary.Hash `json:"tags"`
	Approvees  []trinary.Hash `json:"approvees"`
	MaxResults int            `json:"maxresults"`
	ValueOnly  bool           `json:"valueOnly"`
}

// FindTransactionsResponse struct.
type FindTransactionsResponse struct {
	Hashes   []trinary.Hash `json:"hashes"`
	Duration int            `json:"duration"`
}

type GetMigration struct {
	MilestoneIndex milestone.Index `json:"milestoneIndex"`
}

////////////////////// storeTransactions //////////////////////////

// StoreTransactions struct.
type StoreTransactions struct {
	Trytes []trinary.Trytes `json:"trytes"`
}

//////////////////////// getTrytes ////////////////////////////////

// GetTrytes struct.
type GetTrytes struct {
	Hashes []trinary.Hash `json:"hashes"`
}

// GetTrytesResponse struct.
type GetTrytesResponse struct {
	Trytes   []trinary.Trytes `json:"trytes"`
	Duration int              `json:"duration"`
}

//////////////////////// getWhiteFlagConfirmation ////////////////////////////////

// GetWhiteFlagConfirmationResponse defines the response of a getWhiteFlagConfirmation HTTP API call.
type GetWhiteFlagConfirmationResponse struct {
	// The trytes of the milestone bundle.
	MilestoneBundle []trinary.Trytes `json:"milestoneBundle"`
	// The included bundles of the white-flag confirmation in their DFS order.
	IncludedBundles [][]trinary.Trytes `json:"includedBundles"`
}
