package mqtt

// Topic names
const (
	topicMilestonesLatest = "milestones/latest"
	topicMilestonesSolid  = "milestones/solid"

	topicMessages           = "messages"
	topicMessagesReferenced = "messages/referenced"
	topicMessagesIndexation = "messages/indexation/{index}"
	topicMessagesMetadata   = "messages/{messageId}/metadata"

	topicOutputs = "outputs/{outputId}"

	topicAddressesOutput = "addresses/{address}/outputs"
)
