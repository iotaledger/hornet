package coreapi

import (
	"encoding/json"

	"github.com/iotaledger/hornet/v2/core/protocfg"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/hornet/v2/pkg/protocol"
	"github.com/iotaledger/hornet/v2/pkg/protocol/gossip"
	iotago "github.com/iotaledger/iota.go/v3"
)

// milestoneInfoResponse defines the milestone info response.
type milestoneInfoResponse struct {
	// The index of the milestone.
	Index iotago.MilestoneIndex `json:"index"`
	// The unix time of the milestone payload.
	// The timestamp can be omitted if the milestone is not available
	// (no milestone received yet after starting from snapshot).
	Timestamp uint32 `json:"timestamp,omitempty"`
	// The ID of the milestone.
	// The ID can be omitted if the milestone is not available.
	// (no milestone received yet after starting from snapshot).
	MilestoneID string `json:"milestoneId,omitempty"`
}

type nodeStatus struct {
	// Whether the node is healthy.
	IsHealthy bool `json:"isHealthy"`
	// The latest known milestone index.
	LatestMilestone milestoneInfoResponse `json:"latestMilestone"`
	// The current confirmed milestone's index.
	ConfirmedMilestone milestoneInfoResponse `json:"confirmedMilestone"`
	// The milestone index at which the last pruning commenced.
	PruningIndex iotago.MilestoneIndex `json:"pruningIndex"`
}

type nodeMetrics struct {
	// The current rate of new blocks per second.
	BlocksPerSecond float64 `json:"blocksPerSecond"`
	// The current rate of referenced blocks per second.
	ReferencedBlocksPerSecond float64 `json:"referencedBlocksPerSecond"`
	// The ratio of referenced blocks in relation to new blocks of the last confirmed milestone.
	ReferencedRate float64 `json:"referencedRate"`
}

// infoResponse defines the response of a GET info REST API call.
type infoResponse struct {
	// The name of the node software.
	Name string `json:"name"`
	// The semver version of the node software.
	Version string `json:"version"`
	// The current status of this node.
	Status nodeStatus `json:"status"`
	// The protocol versions this node supports.
	SupportedProtocolVersions protocol.Versions `json:"supportedProtocolVersions"`
	// The protocol parameters used by this node.
	ProtocolParameters *iotago.ProtocolParameters `json:"protocol"`
	// The pending protocol parameters.
	PendingProtocolParameters []*iotago.ProtocolParamsMilestoneOpt `json:"pendingProtocolParameters"`
	// The base token of the network.
	BaseToken *protocfg.BaseToken `json:"baseToken"`
	// The metrics of this node.
	Metrics nodeMetrics `json:"metrics"`
	// The features this node exposes.
	Features []string `json:"features"`
}

// tipsResponse defines the response of a GET tips REST API call.
type tipsResponse struct {
	// The hex encoded block IDs of the tips.
	Tips []string `json:"tips"`
}

// receiptsResponse defines the response of a receipts REST API call.
type receiptsResponse struct {
	Receipts []*utxo.ReceiptTuple `json:"receipts"`
}

// blockMetadataResponse defines the response of a GET block metadata REST API call.
type blockMetadataResponse struct {
	// The hex encoded block ID of the block.
	BlockID string `json:"blockId"`
	// The hex encoded block IDs of the parents the block references.
	Parents []string `json:"parents"`
	// Whether the block is solid.
	Solid bool `json:"isSolid"`
	// The milestone index that references this block.
	ReferencedByMilestoneIndex iotago.MilestoneIndex `json:"referencedByMilestoneIndex,omitempty"`
	// If this block represents a milestone this is the milestone index
	MilestoneIndex iotago.MilestoneIndex `json:"milestoneIndex,omitempty"`
	// The ledger inclusion state of the transaction payload.
	LedgerInclusionState string `json:"ledgerInclusionState,omitempty"`
	// The reason why this block is marked as conflicting.
	ConflictReason *storage.Conflict `json:"conflictReason,omitempty"`
	// Whether the block should be promoted.
	ShouldPromote *bool `json:"shouldPromote,omitempty"`
	// Whether the block should be reattached.
	ShouldReattach *bool `json:"shouldReattach,omitempty"`
	// If this block is referenced by a milestone this returns the index of that block inside the milestone by whiteflag ordering.
	WhiteFlagIndex *uint32 `json:"whiteFlagIndex,omitempty"`
}

// blockCreatedResponse defines the response of a POST blocks REST API call.
type blockCreatedResponse struct {
	// The hex encoded block ID of the block.
	BlockID string `json:"blockId"`
}

// milestoneUTXOChangesResponse defines the response of a GET milestone UTXO changes REST API call.
type milestoneUTXOChangesResponse struct {
	// The index of the milestone.
	Index iotago.MilestoneIndex `json:"index"`
	// The output IDs (transaction hash + output index) of the newly created outputs.
	CreatedOutputs []string `json:"createdOutputs"`
	// The output IDs (transaction hash + output index) of the consumed (spent) outputs.
	ConsumedOutputs []string `json:"consumedOutputs"`
}

// OutputMetadataResponse defines the response of a GET outputs metadata REST API call.
type OutputMetadataResponse struct {
	// The hex encoded block ID of the block.
	BlockID string `json:"blockId"`
	// The hex encoded transaction id from which this output originated.
	TransactionID string `json:"transactionId"`
	// The index of the output.
	OutputIndex uint16 `json:"outputIndex"`
	// Whether this output is spent.
	Spent bool `json:"isSpent"`
	// The milestone index at which this output was spent.
	MilestoneIndexSpent iotago.MilestoneIndex `json:"milestoneIndexSpent,omitempty"`
	// The milestone timestamp this output was spent.
	MilestoneTimestampSpent uint32 `json:"milestoneTimestampSpent,omitempty"`
	// The transaction this output was spent with.
	TransactionIDSpent string `json:"transactionIdSpent,omitempty"`
	// The milestone index at which this output was booked into the ledger.
	MilestoneIndexBooked iotago.MilestoneIndex `json:"milestoneIndexBooked"`
	// The milestone timestamp this output was booked in the ledger.
	MilestoneTimestampBooked uint32 `json:"milestoneTimestampBooked"`
	// The ledger index at which this output was available at.
	LedgerIndex iotago.MilestoneIndex `json:"ledgerIndex"`
}

// OutputResponse defines the response of a GET outputs REST API call.
type OutputResponse struct {
	Metadata *OutputMetadataResponse `json:"metadata"`
	// The output in its serialized form.
	RawOutput *json.RawMessage `json:"output"`
}

// addPeerRequest defines the request for a POST peer REST API call.
type addPeerRequest struct {
	// The libp2p multi address of the peer.
	MultiAddress string `json:"multiAddress"`
	// The alias of the peer.
	Alias *string `json:"alias,omitempty"`
}

// PeerResponse defines the response of a GET peer REST API call.
type PeerResponse struct {
	// The libp2p identifier of the peer.
	ID string `json:"id"`
	// The libp2p multi addresses of the peer.
	MultiAddresses []string `json:"multiAddresses"`
	// The alias of the peer.
	Alias *string `json:"alias,omitempty"`
	// The relation (static, autopeered) of the peer.
	Relation string `json:"relation"`
	// Whether the peer is connected.
	Connected bool `json:"connected"`
	// The gossip protocol information of the peer.
	Gossip *gossip.Info `json:"gossip,omitempty"`
}

// pruneDatabaseRequest defines the request of a prune database REST API call.
type pruneDatabaseRequest struct {
	// The pruning target index.
	Index *iotago.MilestoneIndex `json:"index,omitempty"`
	// The pruning depth.
	Depth *iotago.MilestoneIndex `json:"depth,omitempty"`
	// The target size of the database.
	TargetDatabaseSize *string `json:"targetDatabaseSize,omitempty"`
}

// pruneDatabaseResponse defines the response of a prune database REST API call.
type pruneDatabaseResponse struct {
	// The index of the snapshot.
	Index iotago.MilestoneIndex `json:"index"`
}

// createSnapshotsRequest defines the request of a create snapshots REST API call.
type createSnapshotsRequest struct {
	// The index of the snapshot.
	Index iotago.MilestoneIndex `json:"index"`
}

// createSnapshotsResponse defines the response of a create snapshots REST API call.
type createSnapshotsResponse struct {
	// The index of the snapshot.
	Index iotago.MilestoneIndex `json:"index"`
	// The file path of the snapshot file.
	FilePath string `json:"filePath"`
}

// ComputeWhiteFlagMutationsRequest defines the request for a POST debugComputeWhiteFlagMutations REST API call.
type ComputeWhiteFlagMutationsRequest struct {
	// The index of the milestone.
	Index iotago.MilestoneIndex `json:"index"`
	// The timestamp of the milestone.
	Timestamp uint32 `json:"timestamp"`
	// The hex encoded block IDs of the parents the milestone references.
	Parents []string `json:"parents"`
	// The hex encoded milestone ID of the previous milestone.
	PreviousMilestoneID string `json:"previousMilestoneId"`
}

// ComputeWhiteFlagMutationsResponse defines the response for a POST debugComputeWhiteFlagMutations REST API call.
type ComputeWhiteFlagMutationsResponse struct {
	// The hex encoded inclusion merkle tree root as a result of the white flag computation.
	InclusionMerkleRoot string `json:"inclusionMerkleRoot"`
	// The hex encoded applied merkle tree root as a result of the white flag computation.
	AppliedMerkleRoot string `json:"appliedMerkleRoot"`
}
