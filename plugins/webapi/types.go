package webapi

import (
	"github.com/gohornet/hornet/plugins/gossip"
)

//////////////////// addNeighbors /////////////////////////////////

// AddNeighbors legacy struct
type AddNeighbors struct {
	Command string   `json:"command"`
	Uris    []string `json:"uris"`
}

// AddNeighborsHornet struct
type AddNeighborsHornet struct {
	Command   string     `json:"command"`
	Neighbors []Neighbor `json:"neighbors"`
}

// Neighbor struct
type Neighbor struct {
	Identity   string `json:"identity"`
	PreferIPv6 bool   `json:"preferIPv6"`
	Alias      string `json:"alias"`
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
	Command            string   `json:"command"`
	TrunkTransaction   string   `json:"trunkTransaction"`
	BranchTransaction  string   `json:"branchTransaction"`
	MinWeightMagnitude int      `json:"minWeightMagnitude"`
	Trytes             []string `json:"trytes"`
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
	Command string   `json:"command"`
	Trytes  []string `json:"trytes"`
}

// BradcastTransactionsReturn struct
type BradcastTransactionsReturn struct {
	Duration int `json:"duration"`
}

///////////////////////////////////////////////////////////////////

/////////////////// checkConsistency //////////////////////////////

// CheckConsistency struct
type CheckConsistency struct {
	Command string   `json:"command"`
	Tails   []string `json:"tails"`
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
	Command   string   `json:"command"`
	Bundles   []string `json:"bundles,omitempty"`
	Addresses []string `json:"addresses,omitempty"`
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
	Command   string   `json:"command"`
	Addresses []string `json:"addresses"`
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
	Command      string   `json:"command"`
	Transactions []string `json:"transactions"`
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
	Command string `json:"command"`
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
	Command string `json:"command"`
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
	Command string `json:"command"`
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
	Command   string `json:"command"`
	Depth     uint   `json:"depth"`
	Reference string `json:"reference,omitempty"`
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
	Command string   `json:"command"`
	Hashes  []string `json:"hashes"`
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
	Command string   `json:"removeNeighbors"`
	Uris    []string `json:"uris"`
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
	Command string   `json:"command"`
	Trytes  []string `json:"trytes"`
}

///////////////////////////////////////////////////////////////////

/////////////////// wereAddressesSpentFrom ////////////////////////

// WereAddressesSpentFrom struct
type WereAddressesSpentFrom struct {
	Command   string   `json:"wereAddressesSpentFrom"`
	Addresses []string `json:"addresses"`
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
	Command string `json:"command"`
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
	Command        string `json:"command"`
	MilestoneIndex uint64 `json:"milestoneIndex"`
}

// GetLedgerDiffExt struct
type GetLedgerDiffExt struct {
	Command        string `json:"command"`
	MilestoneIndex uint64 `json:"milestoneIndex"`
}

// GetLedgerDiffReturn struct
type GetLedgerDiffReturn struct {
	Diff           map[string]int64 `json:"diff"`
	MilestoneIndex uint64           `json:"milestoneIndex"`
	Duration       int              `json:"duration"`
}

type TxHashWithValue struct {
	TxHash     string `json:"txHash"`
	TailTxHash string `json:"tailTxHash"`
	BundleHash string `json:"bundleHash"`
	Address    string `json:"address"`
	Value      int64  `json:"value"`
}

type TxWithValue struct {
	TxHash  string `json:"txHash"`
	Address string `json:"address"`
	Index   uint64 `json:"index"`
	Value   int64  `json:"value"`
}

type BundleWithValue struct {
	BundleHash string         `json:"bundleHash"`
	TailTxHash string         `json:"tailTxHash"`
	LastIndex  uint64         `json:"lastIndex"`
	Txs        []*TxWithValue `json:"txs"`
}

// GetLedgerDiffReturn struct
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
	Command     string `json:"command"`
	TargetIndex uint64 `json:"targetIndex"`
	FilePath    string `json:"filePath"`
}

// GetSnapshotReturn struct
type CreateSnapshotReturn struct {
	Duration int `json:"duration"`
}
