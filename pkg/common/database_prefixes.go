package common

const (
	StorePrefixMessages             byte = 1
	StorePrefixMessageMetadata      byte = 2
	StorePrefixMilestones           byte = 3
	StorePrefixChildren             byte = 4
	StorePrefixSnapshot             byte = 5
	StorePrefixUnreferencedMessages byte = 6
	StorePrefixHealth               byte = 255
)

//TODO: when migrating drop StorePrefixIndexation byte = 7
