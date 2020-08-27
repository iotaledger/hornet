package webapi

import (
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/peering/peer"
)

//////////////////// addNeighbors /////////////////////////////////

// AddNeighbors legacy struct
type AddNeighbors struct {
	Command string   `mapstructure:"command"`
	Uris    []string `mapstructure:"uris"`
}

// AddNeighborsHornet struct
type AddNeighborsHornet struct {
	Command   string     `mapstructure:"command"`
	Neighbors []Neighbor `mapstructure:"neighbors"`
}

// Neighbor struct
type Neighbor struct {
	Identity   string `mapstructure:"identity"`
	Alias      string `mapstructure:"alias"`
	PreferIPv6 bool   `mapstructure:"prefer_ipv6"`
}

// AddNeighborsResponse struct
type AddNeighborsResponse struct {
	AddedNeighbors int `json:"addedNeighbors"`
	Duration       int `json:"duration"`
}

//////////////////// attachToTangle ///////////////////////////////

// AttachToTangle struct
type AttachToTangle struct {
	Command            string           `mapstructure:"command"`
	TrunkTransaction   trinary.Hash     `mapstructure:"trunkTransaction"`
	BranchTransaction  trinary.Hash     `mapstructure:"branchTransaction"`
	MinWeightMagnitude int              `mapstructure:"minWeightMagnitude,omitempty"`
	Trytes             []trinary.Trytes `mapstructure:"trytes"`
}

// AttachToTangleReturn struct
type AttachToTangleReturn struct {
	Trytes   []trinary.Trytes `json:"trytes"`
	Duration int              `json:"duration"`
}

////////////////// broadcastTransactions //////////////////////////

// BroadcastTransactions struct
type BroadcastTransactions struct {
	Command string           `mapstructure:"command"`
	Trytes  []trinary.Trytes `mapstructure:"trytes"`
}

// BradcastTransactionsReturn struct
type BradcastTransactionsReturn struct {
	Duration int `json:"duration"`
}

/////////////////// checkConsistency //////////////////////////////

// CheckConsistencyReturn struct
type CheckConsistencyReturn struct {
	State    bool `json:"state"`
	Duration int  `json:"duration"`
}

//////////////////////// error ////////////////////////////////////

// ErrorReturn struct
type ErrorReturn struct {
	Error string `json:"error"`
}

// ResultReturn struct
type ResultReturn struct {
	Message string `json:"message"`
}

/////////////////// findTransactions //////////////////////////////

// FindTransactions struct
type FindTransactions struct {
	Command    string         `mapstructure:"command"`
	Bundles    []trinary.Hash `mapstructure:"bundles"`
	Addresses  []trinary.Hash `mapstructure:"addresses"`
	Tags       []trinary.Hash `mapstructure:"tags"`
	Approvees  []trinary.Hash `mapstructure:"approvees"`
	MaxResults int            `mapstructure:"maxresults"`
	ValueOnly  bool           `json:"valueOnly"`
}

// FindTransactionsReturn struct
type FindTransactionsReturn struct {
	Hashes   []trinary.Hash `json:"hashes"`
	Duration int            `json:"duration"`
}

///////////////////// getBalances /////////////////////////////////

// GetBalances struct
type GetBalances struct {
	Command   string         `mapstructure:"command"`
	Addresses []trinary.Hash `mapstructure:"addresses"`
}

// GetBalancesReturn struct
type GetBalancesReturn struct {
	Balances       []trinary.Hash  `json:"balances"`
	References     []trinary.Hash  `json:"references"`
	MilestoneIndex milestone.Index `json:"milestoneIndex"`
	Duration       int             `json:"duration"`
}

/////////////////// getInclusionStates ////////////////////////////

// GetInclusionStates struct
type GetInclusionStates struct {
	Command      string         `mapstructure:"command"`
	Transactions []trinary.Hash `mapstructure:"transactions"`
}

// GetInclusionStatesReturn struct
type GetInclusionStatesReturn struct {
	States   []bool `json:"states"`
	Duration int    `json:"duration"`
}

////////////////////// getNeighbors ///////////////////////////////

// GetNeighbors struct
type GetNeighbors struct {
	Command string `mapstructure:"command"`
}

// GetNeighborsReturn struct
type GetNeighborsReturn struct {
	Neighbors []*peer.Info `json:"neighbors"`
	Duration  int          `json:"duration"`
}

/////////////////////// getNodeInfo ///////////////////////////////

// GetNodeInfo struct
type GetNodeInfo struct {
	Command string `mapstructure:"command"`
}

// GetNodeInfoReturn struct
type GetNodeInfoReturn struct {
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

////////////////// getNodeAPIConfiguration //////////////////////////

// GetNodeAPIConfiguration struct
type GetNodeAPIConfiguration struct {
	Command string `mapstructure:"command"`
}

// GetNodeAPIConfigurationReturn struct
type GetNodeAPIConfigurationReturn struct {
	MaxFindTransactions int             `json:"maxFindTransactions"`
	MaxRequestsList     int             `json:"maxRequestsList"`
	MaxGetTrytes        int             `json:"maxGetTrytes"`
	MaxBodyLength       int             `json:"maxBodyLength"`
	MilestoneStartIndex milestone.Index `json:"milestoneStartIndex"`
	Duration            int             `json:"duration"`
}

///////////////// getTipInfo ////////////////////////

// GetTipInfo struct
type GetTipInfo struct {
	Command         string       `mapstructure:"command"`
	TailTransaction trinary.Hash `mapstructure:"tailTransaction"`
}

// GetTipInfoReturn struct
type GetTipInfoReturn struct {
	Confirmed      bool `json:"confirmed"`
	Conflicting    bool `json:"conflicting"`
	ShouldPromote  bool `json:"shouldPromote"`
	ShouldReattach bool `json:"shouldReattach"`
	Duration       int  `json:"duration"`
}

///////////////// getTransactionsToApprove ////////////////////////

// GetTransactionsToApprove struct
type GetTransactionsToApprove struct {
	Command   string       `mapstructure:"command"`
	Depth     uint         `mapstructure:"depth"`
	Reference trinary.Hash `mapstructure:"reference"`
}

// GetTransactionsToApproveReturn struct
type GetTransactionsToApproveReturn struct {
	TrunkTransaction  trinary.Hash `json:"trunkTransaction"`
	BranchTransaction trinary.Hash `json:"branchTransaction"`
	Duration          int          `json:"duration"`
}

//////////////////////// getTrytes ////////////////////////////////

// GetTrytes struct
type GetTrytes struct {
	Command string         `mapstructure:"command"`
	Hashes  []trinary.Hash `mapstructure:"hashes"`
}

// GetTrytesReturn struct
type GetTrytesReturn struct {
	Trytes   []trinary.Trytes `json:"trytes"`
	Duration int              `json:"duration"`
}

///////////////////// removeNeighbors /////////////////////////////

// RemoveNeighbors struct
type RemoveNeighbors struct {
	Command string   `mapstructure:"removeNeighbors"`
	Uris    []string `mapstructure:"uris"`
}

// RemoveNeighborsReturn struct
type RemoveNeighborsReturn struct {
	RemovedNeighbors uint `json:"removedNeighbors"`
	Duration         int  `json:"duration"`
}

////////////////////// storeTransactions //////////////////////////

// StoreTransactions struct
type StoreTransactions struct {
	Command string           `mapstructure:"command"`
	Trytes  []trinary.Trytes `mapstructure:"trytes"`
}

/////////////////// wereAddressesSpentFrom ////////////////////////

// WereAddressesSpentFrom struct
type WereAddressesSpentFrom struct {
	Command   string         `mapstructure:"wereAddressesSpentFrom"`
	Addresses []trinary.Hash `mapstructure:"addresses"`
}

// WereAddressesSpentFromReturn struct
type WereAddressesSpentFromReturn struct {
	States   []bool `json:"states"`
	Duration int    `json:"duration"`
}

/////////////////// getLedgerDiff ////////////////////////

// GetLedgerDiff struct
type GetLedgerDiff struct {
	Command        string          `mapstructure:"command"`
	MilestoneIndex milestone.Index `mapstructure:"milestoneIndex"`
}

// GetLedgerDiffExt struct
type GetLedgerDiffExt struct {
	Command        string          `mapstructure:"command"`
	MilestoneIndex milestone.Index `mapstructure:"milestoneIndex"`
}

// GetLedgerDiffReturn struct
type GetLedgerDiffReturn struct {
	Diff           map[trinary.Hash]int64 `json:"diff"`
	MilestoneIndex milestone.Index        `json:"milestoneIndex"`
	Duration       int                    `json:"duration"`
}

// TxHashWithValue struct
type TxHashWithValue struct {
	TxHash     trinary.Hash `mapstructure:"txHash"`
	TailTxHash trinary.Hash `mapstructure:"tailTxHash"`
	BundleHash trinary.Hash `mapstructure:"bundleHash"`
	Address    trinary.Hash `mapstructure:"address"`
	Value      int64        `mapstructure:"value"`
}

// TxWithValue struct
type TxWithValue struct {
	TxHash  trinary.Hash `mapstructure:"txHash"`
	Address trinary.Hash `mapstructure:"address"`
	Index   uint64       `mapstructure:"index"`
	Value   int64        `mapstructure:"value"`
}

// BundleWithValue struct
type BundleWithValue struct {
	BundleHash trinary.Hash   `mapstructure:"bundleHash"`
	TailTxHash trinary.Hash   `mapstructure:"tailTxHash"`
	LastIndex  uint64         `mapstructure:"lastIndex"`
	Txs        []*TxWithValue `mapstructure:"txs"`
}

// GetLedgerDiffExtReturn struct
type GetLedgerDiffExtReturn struct {
	ConfirmedTxWithValue      []*TxHashWithValue     `json:"confirmedTxWithValue"`
	ConfirmedBundlesWithValue []*BundleWithValue     `json:"confirmedBundlesWithValue"`
	Diff                      map[trinary.Hash]int64 `json:"diff"`
	MilestoneIndex            milestone.Index        `json:"milestoneIndex"`
	Duration                  int                    `json:"duration"`
}

/////////////////// getLedgerState ////////////////////////

// GetLedgerState struct
type GetLedgerState struct {
	Command     string          `mapstructure:"command"`
	TargetIndex milestone.Index `mapstructure:"targetIndex,omitempty"`
}

// GetLedgerStateReturn struct
type GetLedgerStateReturn struct {
	Balances       map[trinary.Hash]uint64 `json:"balances"`
	MilestoneIndex milestone.Index         `json:"milestoneIndex"`
	Duration       int                     `json:"duration"`
}

/////////////////// createSnapshotFile ////////////////////////

// CreateSnapshotFile struct
type CreateSnapshotFile struct {
	Command     string          `mapstructure:"command"`
	TargetIndex milestone.Index `mapstructure:"targetIndex"`
}

// CreateSnapshotFileReturn struct
type CreateSnapshotFileReturn struct {
	Duration int `json:"duration"`
}

/////////////////// pruneDatabase ////////////////////////

// PruneDatabase struct
type PruneDatabase struct {
	Command     string          `mapstructure:"command"`
	TargetIndex milestone.Index `mapstructure:"targetIndex"`
	Depth       milestone.Index `mapstructure:"depth"`
}

// PruneDatabaseReturn struct
type PruneDatabaseReturn struct {
	Duration int `json:"duration"`
}

///////////////////// getRequests /////////////////////////////////

// GetRequests struct
type GetRequests struct {
	Command string `mapstructure:"command"`
}

// GetRequestsReturn struct
type GetRequestsReturn struct {
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

// SearchConfirmedApprover struct
type SearchConfirmedApprover struct {
	Command         string       `mapstructure:"command"`
	TxHash          trinary.Hash `mapstructure:"txHash"`
	SearchMilestone bool         `mapstructure:"searchMilestone"`
}

// ApproverStruct struct
type ApproverStruct struct {
	TxHash            trinary.Hash `mapstructure:"txHash"`
	ReferencedByTrunk bool         `mapstructure:"referencedByTrunk"`
}

// SearchConfirmedApproverReturn struct
type SearchConfirmedApproverReturn struct {
	ConfirmedTxHash           trinary.Hash      `json:"confirmedTxHash"`
	ConfirmedByMilestoneIndex milestone.Index   `json:"confirmedByMilestoneIndex"`
	TanglePath                []*ApproverStruct `json:"tanglePath"`
	TanglePathLength          int               `json:"tanglePathLength"`
}

///////////////// searchEntryPoint /////////////////////////

// SearchEntryPoint struct
type SearchEntryPoint struct {
	Command string       `mapstructure:"command"`
	TxHash  trinary.Hash `mapstructure:"txHash"`
}

// EntryPoint struct
type EntryPoint struct {
	TxHash                    trinary.Hash    `json:"txHash"`
	ConfirmedByMilestoneIndex milestone.Index `json:"confirmedByMilestoneIndex"`
}

type TransactionWithApprovers struct {
	TxHash            trinary.Hash `json:"txHash"`
	TrunkTransaction  trinary.Hash `json:"trunkTransaction"`
	BranchTransaction trinary.Hash `json:"branchTransaction"`
}

// SearchEntryPointReturn struct
type SearchEntryPointReturn struct {
	TanglePath       []*TransactionWithApprovers `json:"tanglePath"`
	EntryPoints      []*EntryPoint               `json:"entryPoints"`
	TanglePathLength int                         `json:"tanglePathLength"`
}

///////////////// triggerSolidifier /////////////////////////

// SearchConfirmedApprover struct
type TriggerSolidifier struct {
	Command string `mapstructure:"command"`
}

/////////////////// getFundsOnSpentAddresses //////////////////////////////

// GetFundsOnSpentAddressesReturn struct
type GetFundsOnSpentAddressesReturn struct {
	Command   string                `mapstructure:"command"`
	Addresses []*AddressWithBalance `mapstructure:"addresses"`
}

type AddressWithBalance struct {
	Address trinary.Hash `mapstructure:"address"`
	Balance uint64       `mapstructure:"balance"`
}
