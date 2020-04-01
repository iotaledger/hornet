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

///////////////////////////////////////////////////////////////////

//////////////////// attachToTangle ///////////////////////////////

// AttachToTangle struct
type AttachToTangle struct {
	Command            string           `mapstructure:"command"`
	TrunkTransaction   trinary.Hash     `mapstructure:"trunkTransaction"`
	BranchTransaction  trinary.Hash     `mapstructure:"branchTransaction"`
	MinWeightMagnitude int              `mapstructure:"minWeightMagnitude"`
	Trytes             []trinary.Trytes `mapstructure:"trytes"`
}

// AttachToTangleReturn struct
type AttachToTangleReturn struct {
	Trytes   []trinary.Trytes `json:"trytes"`
	Duration int              `json:"duration"`
}

///////////////////////////////////////////////////////////////////

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

///////////////////////////////////////////////////////////////////

/////////////////// checkConsistency //////////////////////////////

// CheckConsistency struct
type CheckConsistency struct {
	Command string         `mapstructure:"command"`
	Tails   []trinary.Hash `mapstructure:"tails"`
}

// CheckConsistencyReturn struct
type CheckConsistencyReturn struct {
	State    bool   `json:"state"`
	Info     string `json:"info,omitempty"`
	Duration int    `json:"duration"`
}

///////////////////////////////////////////////////////////////////

//////////////////////// error ////////////////////////////////////

// ErrorReturn struct
type ErrorReturn struct {
	Error string `json:"error"`
}

///////////////////////////////////////////////////////////////////

/////////////////// findTransactions //////////////////////////////

// FindTransactions struct
type FindTransactions struct {
	Command    string         `mapstructure:"command"`
	Bundles    []trinary.Hash `mapstructure:"bundles"`
	Addresses  []trinary.Hash `mapstructure:"addresses"`
	Tags       []trinary.Hash `mapstructure:"tags"`
	Approvees  []trinary.Hash `mapstructure:"approvees"`
	MaxResults int            `mapstructure:"maxresults"`
}

// FindTransactionsReturn struct
type FindTransactionsReturn struct {
	Hashes   []trinary.Hash `json:"hashes"`
	Duration int            `json:"duration"`
}

///////////////////////////////////////////////////////////////////

///////////////////// getBalances /////////////////////////////////

// GetBalances struct
type GetBalances struct {
	Command   string         `mapstructure:"command"`
	Addresses []trinary.Hash `mapstructure:"addresses"`
}

// GetBalancesReturn struct
type GetBalancesReturn struct {
	Balances       []trinary.Hash `json:"balances"`
	References     []trinary.Hash `json:"references"`
	MilestoneIndex uint32         `json:"milestoneIndex"`
	Duration       int            `json:"duration"`
}

///////////////////////////////////////////////////////////////////

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

///////////////////////////////////////////////////////////////////

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

///////////////////////////////////////////////////////////////////

/////////////////////// getNodeInfo ///////////////////////////////

// GetNodeInfo struct
type GetNodeInfo struct {
	Command string `mapstructure:"command"`
}

// GetNodeInfoReturn struct
type GetNodeInfoReturn struct {
	AppName                            string       `json:"appName"`
	AppVersion                         string       `json:"appVersion"`
	NodeAlias                          string       `json:"nodeAlias,omitempty"`
	LatestMilestone                    trinary.Hash `json:"latestMilestone"`
	LatestMilestoneIndex               uint32       `json:"latestMilestoneIndex"`
	LatestSolidSubtangleMilestone      trinary.Hash `json:"latestSolidSubtangleMilestone"`
	LatestSolidSubtangleMilestoneIndex uint32       `json:"latestSolidSubtangleMilestoneIndex"`
	IsSynced                           bool         `json:"isSynced"`
	MilestoneStartIndex                uint32       `json:"milestoneStartIndex"`
	LastSnapshottedMilestoneIndex      uint32       `json:"lastSnapshottedMilestoneIndex"`
	Neighbors                          uint         `json:"neighbors"`
	Time                               int64        `json:"time"`
	Tips                               uint16       `json:"tips"`
	TransactionsToRequest              int          `json:"transactionsToRequest"`
	Features                           []string     `json:"features"`
	CoordinatorAddress                 trinary.Hash `json:"coordinatorAddress"`
	Duration                           int          `json:"duration"`
}

///////////////////////////////////////////////////////////////////

/////////////////////// getNodeAPIConfiguration ///////////////////////////////

// GetNodeAPIConfiguration struct
type GetNodeAPIConfiguration struct {
	Command string `mapstructure:"command"`
}

// GetNodeAPIConfigurationReturn struct
type GetNodeAPIConfigurationReturn struct {
	MaxFindTransactions int    `json:"maxFindTransactions"`
	MaxRequestsList     int    `json:"maxRequestsList"`
	MaxGetTrytes        int    `json:"maxGetTrytes"`
	MaxBodyLength       int    `json:"maxBodyLength"`
	MilestoneStartIndex uint32 `json:"milestoneStartIndex"`
	Duration            int    `json:"duration"`
}

///////////////////////////////////////////////////////////////////

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

///////////////////////////////////////////////////////////////////

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

///////////////////////////////////////////////////////////////////

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

///////////////////////////////////////////////////////////////////

////////////////////// storeTransactions //////////////////////////

// StoreTransactions struct
type StoreTransactions struct {
	Command string           `mapstructure:"command"`
	Trytes  []trinary.Trytes `mapstructure:"trytes"`
}

///////////////////////////////////////////////////////////////////

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

///////////////////////////////////////////////////////////////////

/////////////////// getSnapshot ////////////////////////

// GetSnapshot struct
type GetSnapshot struct {
	Command string `mapstructure:"command"`
}

// GetSnapshotReturn struct
type GetSnapshotReturn struct {
	Balances       map[trinary.Hash]uint64 `json:"balances"`
	MilestoneIndex uint64                  `json:"milestoneIndex"`
	Duration       int                     `json:"duration"`
}

///////////////////////////////////////////////////////////////////

/////////////////// getLedgerDiff ////////////////////////

// GetLedgerDiff struct
type GetLedgerDiff struct {
	Command        string `mapstructure:"command"`
	MilestoneIndex uint64 `mapstructure:"milestoneIndex"`
}

// GetLedgerDiffExt struct
type GetLedgerDiffExt struct {
	Command        string `mapstructure:"command"`
	MilestoneIndex uint64 `mapstructure:"milestoneIndex"`
}

// GetLedgerDiffReturn struct
type GetLedgerDiffReturn struct {
	Diff           map[trinary.Hash]int64 `json:"diff"`
	MilestoneIndex uint64                 `json:"milestoneIndex"`
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
	MilestoneIndex            uint64                 `json:"milestoneIndex"`
	Duration                  int                    `json:"duration"`
}

///////////////////////////////////////////////////////////////////

/////////////////// createSnapshot ////////////////////////

// CreateSnapshot struct
type CreateSnapshot struct {
	Command     string `mapstructure:"command"`
	TargetIndex uint64 `mapstructure:"targetIndex"`
	FilePath    string `mapstructure:"filePath"`
}

// CreateSnapshotReturn struct
type CreateSnapshotReturn struct {
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

///////////////////////////////////////////////////////////////////

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
	ConfirmedByMilestoneIndex uint32            `json:"confirmedByMilestoneIndex"`
	TanglePath                []*ApproverStruct `json:"tanglePath"`
	TanglePathLength          int               `json:"tanglePathLength"`
}

}

///////////////////////////////////////////////////////////////////
