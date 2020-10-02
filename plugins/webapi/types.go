package webapi

import (
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/p2p"
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

//////////////////////// error ////////////////////////////////////

// ErrorReturn struct
type ErrorReturn struct {
	Error string `json:"error"`
}

// ResultReturn struct
type ResultReturn struct {
	Message string `json:"message"`
}

////////////////////// getNeighbors ///////////////////////////////

// GetNeighbors struct
type GetNeighbors struct {
	Command string `mapstructure:"command"`
}

// GetNeighborsReturn struct
type GetNeighborsReturn struct {
	Neighbors []*p2p.PeerSnapshot `json:"neighbors"`
	Duration  int                 `json:"duration"`
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

///////////////// getTransactionsToApprove ////////////////////////

// GetTransactionsToApproveReturn struct
type GetTransactionsToApproveReturn struct {
	TrunkTransaction  string `json:"trunkTransaction"`
	BranchTransaction string `json:"branchTransaction"`
	Duration          int    `json:"duration"`
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
	MessageID        string          `json:"messageID"`
	Type             string          `json:"type"`
	TxExists         bool            `json:"txExists"`
	EnqueueTimestamp int64           `json:"enqueueTime"`
	MilestoneIndex   milestone.Index `json:"milestoneIndex"`
}

///////////////// triggerSolidifier /////////////////////////

// SearchConfirmedChild struct
type TriggerSolidifier struct {
	Command string `mapstructure:"command"`
}
