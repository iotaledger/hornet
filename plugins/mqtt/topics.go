package mqtt

// Topic names
const (
	topicMilestonesLatest    = "milestones/latest"
	topicMilestonesConfirmed = "milestones/confirmed"

	topicMessages           = "messages"
	topicMessagesReferenced = "messages/referenced"
	topicMessagesTaggedData = "messages/data/{tag}"
	topicMessagesMetadata   = "messages/{messageId}/metadata"

	topicTransactionsIncludedMessage = "transactions/{transactionId}/included-message"

	topicOutputs = "outputs/{outputId}"

	topicReceipts = "receipts"

	topicAddressesOutput        = "addresses/{address}/outputs"
	topicAddressesEd25519Output = "addresses/ed25519/{address}/outputs"
)
