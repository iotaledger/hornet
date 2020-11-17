package mqtt

import (
	"encoding/json"

	"github.com/gohornet/hornet/pkg/model/milestone"
)

// milestonePayload defines the payload of the milestone latest and solid topics
type milestonePayload struct {
	// The index of the milestone.
	Index uint32 `json:"milestoneIndex"`
	// The unix time of the milestone payload.
	Time int64 `json:"timestamp"`
}

// messageMetadataPayload defines the payload of the message metadata topic
type messageMetadataPayload struct {
	// The hex encoded message ID of the message.
	MessageID string `json:"messageId"`
	// The hex encoded message ID of the 1st parent the message references.
	Parent1 string `json:"parent1MessageId"`
	// The hex encoded message ID of the 2nd parent the message references.
	Parent2 string `json:"parent2MessageId"`
	// Whether the message is solid.
	Solid bool `json:"isSolid"`
	// The milestone index that references this message.
	ReferencedByMilestoneIndex *milestone.Index `json:"referencedByMilestoneIndex,omitempty"`
	// The ledger inclusion state of the transaction payload.
	LedgerInclusionState *string `json:"ledgerInclusionState,omitempty"`
	// Whether the message should be promoted.
	ShouldPromote *bool `json:"shouldPromote,omitempty"`
	// Whether the message should be reattached.
	ShouldReattach *bool `json:"shouldReattach,omitempty"`
}

// outputPayload defines the payload of the output topics
type outputPayload struct {
	// The hex encoded message ID of the message.
	MessageID string `json:"messageId"`
	// The hex encoded transaction id from which this output originated.
	TransactionID string `json:"transactionId"`
	// The index of the output.
	OutputIndex uint16 `json:"outputIndex"`
	// Whether this output is spent.
	Spent bool `json:"isSpent"`
	// The output in its serialized form.
	RawOutput *json.RawMessage `json:"output"`
}
