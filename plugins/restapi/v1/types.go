package v1

// infoResponse defines the response of a node info REST API call.
type infoResponse struct {
	// The name of the node software.
	Name string `json:"name"`
	// The semver version of the node software.
	Version string `json:"version"`
	// Whether the node is healthy.
	IsHealthy bool `json:"isHealthy"`
	// Whether the node is synchronized.
	IsSynced bool `json:"isSynced"`
	// The used coordinator public key.
	CoordinatorPublicKey string `json:"coordinatorPublicKey"`
	// The latest known milestone message ID.
	LatestMilestoneMessageID string `json:"latestMilestoneMessageId"`
	// The latest known milestone index.
	LatestMilestoneIndex uint64 `json:"latestMilestoneIndex"`
	// The current solid milestone's message ID.
	LatestSolidMilestoneMessageID string `json:"solidMilestoneMessageId"`
	// The current solid milestone's index.
	LatestSolidMilestoneIndex uint64 `json:"solidMilestoneIndex"`
	// The milestone index at which the last pruning commenced.
	PruningIndex uint64 `json:"pruningIndex"`
	// The features this node exposes.
	Features []string `json:"features"`
}

// tipsResponse defines the response of a tips REST API call.
type tipsResponse struct {
	// The hex encoded hash of the first tip message.
	Tip1 string `json:"tip1"`
	// The hex encoded hash of the second tip message.
	Tip2 string `json:"tip2"`
}

