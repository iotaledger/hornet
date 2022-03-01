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

	topicOutputs                                 = "outputs/{outputId}"
	topicOutputsByUnlockConditionAndAddress      = "outputs/unlock/{condition}/{address}"
	topicSpentOutputsByUnlockConditionAndAddress = "outputs/unlock/{condition}/{address}/spent"

	topicReceipts = "receipts"
)

type unlockCondition string

const (
	unlockConditionAny              unlockCondition = "+"
	unlockConditionAddress          unlockCondition = "address"
	unlockConditionStorageReturn    unlockCondition = "storageReturn"
	unlockConditionExpirationReturn unlockCondition = "expirationReturn"
	unlockConditionStateController  unlockCondition = "stateController"
	unlockConditionGovernor         unlockCondition = "governor"
	unlockConditionImmutableAlias   unlockCondition = "immutableAlias"
)
