package webapi

import (
	"github.com/gohornet/hornet/packages/model/queue"
	"github.com/gohornet/hornet/plugins/gossip"
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
	Command            string   `mapstructure:"command"`
	TrunkTransaction   string   `mapstructure:"trunkTransaction"`
	BranchTransaction  string   `mapstructure:"branchTransaction"`
	MinWeightMagnitude int      `mapstructure:"minWeightMagnitude"`
	Trytes             []string `mapstructure:"trytes"`
}

// AttachToTangleReturn struct
type AttachToTangleReturn struct {
	Trytes   []string `json:"trytes"`
	Duration int      `json:"duration"`
}

///////////////////////////////////////////////////////////////////

////////////////// broadcastTransactions //////////////////////////

// BroadcastTransactions struct
type BroadcastTransactions struct {
	Command string   `mapstructure:"command"`
	Trytes  []string `mapstructure:"trytes"`
}

// BradcastTransactionsReturn struct
type BradcastTransactionsReturn struct {
	Duration int `json:"duration"`
}

///////////////////////////////////////////////////////////////////

/////////////////// checkConsistency //////////////////////////////

// CheckConsistency struct
type CheckConsistency struct {
	Command string   `mapstructure:"command"`
	Tails   []string `mapstructure:"tails"`
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
	Command   string   `mapstructure:"command"`
	Bundles   []string `mapstructure:"bundles"`
	Addresses []string `mapstructure:"addresses"`
}

// FindTransactionsReturn struct
type FindTransactionsReturn struct {
	Hashes   []string `json:"hashes"`
	Duration int      `json:"duration"`
}

///////////////////////////////////////////////////////////////////

///////////////////// getBalances /////////////////////////////////

// GetBalances struct
type GetBalances struct {
	Command   string   `mapstructure:"command"`
	Addresses []string `mapstructure:"addresses"`
}

// GetBalancesReturn struct
type GetBalancesReturn struct {
	Balances       []string `json:"balances"`
	References     []string `json:"references"`
	MilestoneIndex uint32   `json:"milestoneIndex"`
	Duration       int      `json:"duration"`
}

///////////////////////////////////////////////////////////////////

/////////////////// getInclusionStates ////////////////////////////

// GetInclusionStates struct
type GetInclusionStates struct {
	Command      string   `mapstructure:"command"`
	Transactions []string `mapstructure:"transactions"`
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
	Neighbors []gossip.NeighborInfo `json:"neighbors"`
	Duration  int                   `json:"duration"`
}

///////////////////////////////////////////////////////////////////

/////////////////////// getNodeInfo ///////////////////////////////

// GetNodeInfo struct
type GetNodeInfo struct {
	Command string `mapstructure:"command"`
}

// GetNodeInfoReturn struct
type GetNodeInfoReturn struct {
	AppName                            string   `json:"appName"`
	AppVersion                         string   `json:"appVersion"`
	LatestMilestone                    string   `json:"latestMilestone"`
	LatestMilestoneIndex               uint32   `json:"latestMilestoneIndex"`
	LatestSolidSubtangleMilestone      string   `json:"latestSolidSubtangleMilestone"`
	LatestSolidSubtangleMilestoneIndex uint32   `json:"latestSolidSubtangleMilestoneIndex"`
	IsSynced                           bool     `json:"isSynced"`
	MilestoneStartIndex                uint32   `json:"milestoneStartIndex,omitempty"`
	LastSnapshottedMilestoneIndex      uint32   `json:"lastSnapshottedMilestoneIndex,omitempty"`
	Neighbors                          uint     `json:"neighbors"`
	Time                               int64    `json:"time"`
	Tips                               uint16   `json:"tips"`
	TransactionsToRequest              int      `json:"transactionsToRequest"`
	Features                           []string `json:"features"`
	CoordinatorAddress                 string   `json:"coordinatorAddress"`
	Duration                           int      `json:"duration"`
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
	MilestoneStartIndex uint32 `json:"milestoneStartIndex,omitempty"`
	Duration            int    `json:"duration"`
}

///////////////////////////////////////////////////////////////////

///////////////// getTransactionsToApprove ////////////////////////

// GetTransactionsToApprove struct
type GetTransactionsToApprove struct {
	Command   string `mapstructure:"command"`
	Depth     uint   `mapstructure:"depth"`
	Reference string `mapstructure:"reference"`
}

// GetTransactionsToApproveReturn struct
type GetTransactionsToApproveReturn struct {
	TrunkTransaction  string `json:"trunkTransaction"`
	BranchTransaction string `json:"branchTransaction"`
	Duration          int    `json:"duration"`
}

///////////////////////////////////////////////////////////////////

//////////////////////// getTrytes ////////////////////////////////

// GetTrytes struct
type GetTrytes struct {
	Command string   `mapstructure:"command"`
	Hashes  []string `mapstructure:"hashes"`
}

// GetTrytesReturn struct
type GetTrytesReturn struct {
	Trytes   []string `json:"trytes"`
	Duration int      `json:"duration"`
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
	Command string   `mapstructure:"command"`
	Trytes  []string `mapstructure:"trytes"`
}

///////////////////////////////////////////////////////////////////

/////////////////// wereAddressesSpentFrom ////////////////////////

// WereAddressesSpentFrom struct
type WereAddressesSpentFrom struct {
	Command   string   `mapstructure:"wereAddressesSpentFrom"`
	Addresses []string `mapstructure:"addresses"`
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
	Balances       map[string]uint64 `json:"balances"`
	MilestoneIndex uint64            `json:"milestoneIndex"`
	Duration       int               `json:"duration"`
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
	Diff           map[string]int64 `json:"diff"`
	MilestoneIndex uint64           `json:"milestoneIndex"`
	Duration       int              `json:"duration"`
}

// TxHashWithValue struct
type TxHashWithValue struct {
	TxHash     string `mapstructure:"txHash"`
	TailTxHash string `mapstructure:"tailTxHash"`
	BundleHash string `mapstructure:"bundleHash"`
	Address    string `mapstructure:"address"`
	Value      int64  `mapstructure:"value"`
}

// TxWithValue struct
type TxWithValue struct {
	TxHash  string `mapstructure:"txHash"`
	Address string `mapstructure:"address"`
	Index   uint64 `mapstructure:"index"`
	Value   int64  `mapstructure:"value"`
}

// BundleWithValue struct
type BundleWithValue struct {
	BundleHash string         `mapstructure:"bundleHash"`
	TailTxHash string         `mapstructure:"tailTxHash"`
	LastIndex  uint64         `mapstructure:"lastIndex"`
	Txs        []*TxWithValue `mapstructure:"txs"`
}

// GetLedgerDiffExtReturn struct
type GetLedgerDiffExtReturn struct {
	ConfirmedTxWithValue      []*TxHashWithValue `json:"confirmedTxWithValue"`
	ConfirmedBundlesWithValue []*BundleWithValue `json:"confirmedBundlesWithValue"`
	Diff                      map[string]int64   `json:"diff"`
	MilestoneIndex            uint64             `json:"milestoneIndex"`
	Duration                  int                `json:"duration"`
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
	Requests []*queue.DebugRequest `json:"requests"`
}

///////////////////////////////////////////////////////////////////
