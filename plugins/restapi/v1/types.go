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

type messageMetadataResponse struct {
	MessageID string `json:"messageId"`
	// The 1st parent the message references.
	Parent1 string `json:"parent1"`
	// The 2nd parent the message references.
	Parent2               string  `json:"parent2"`
	Solid                 bool    `json:"solid"`
	ReferencedByMilestone *uint64 `json:"referencedByMilestone"`
	LedgerInclusionState  *string `json:"ledgerInclusionState,omitempty"`
	ShouldPromote         *bool   `json:"shouldPromote,omitempty"`
	ShouldReattach        *bool   `json:"shouldReattach,omitempty"`
}

type messageResponse struct {
	MessageID string
	Data      string
}

type childrenResponse struct {
	MessageID  string   `json:"messageId"`
	MaxResults uint32   `json:"maxResults"`
	Count      uint32   `json:"count"`
	Children   []string `json:"children"`
}

type messageIDsResponse struct {
	Index      string   `json:"index"`
	MaxResults uint32   `json:"maxResults"`
	Count      uint32   `json:"count"`
	MessageIDs []string `json:"messageIds"`
}

type milestoneResponse struct {
	Index     uint32 `json:"milestoneIndex"`
	MessageID string `json:"messageId"`
	Time      string `json:"timestamp"`
}

type outputResponse struct {
	OutputID   string `json:"outputID"`
	MessageID  string `json:"messageId"`
	OutputType byte   `json:"outputType"`
	Address    string `json:"address"`
	Amount     uint64 `json:"amount"`
}

type addressOutputsResponse struct {
	Address    string   `json:"address"`
	MaxResults uint32   `json:"maxResults"`
	Count      uint32   `json:"count"`
	OutputIDs  []string `json:"outputIDs"`
}

type addressBalanceResponse struct {
	Address    string `json:"address"`
	MaxResults uint32 `json:"maxResults"`
	Count      uint32 `json:"count"`
	Balance    uint64 `json:"balance"`
}

