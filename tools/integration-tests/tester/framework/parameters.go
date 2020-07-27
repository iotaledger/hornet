package framework

const (
	autopeeringMaxTries = 50

	// The API port to use on every instance to expose the web API.
	APIPort = 14265

	containerNameTester      = "/tester"
	containerNameEntryNode   = "entry_node"
	containerNameReplica     = "replica_"
	containerNameSuffixPumba = "_pumba"

	logsDir = "/tmp/logs/"

	disabledPluginsEntryNode = "dashboard,profiling,gossip"
	disabledPluginsPeer      = "dashboard"
	snapshotFilePath         = "/assets/snapshot.txt"
	dockerLogsPrefixLen      = 8

	coordinatorSeed            = "MFKHXRCTTRSPATPMAGFCSSIBJXDWNCLBQJPMHQGMGJEWYJNUXSXAGVSUJK9BMCAFEDNTXHDWWVVNILDRG"
	coordinatorAddress         = "JFQ999DVN9CBBQX9DSAIQRAFRALIHJMYOXAQSTCJLGA9DLOKIWHJIFQKMCQ9QHWW9RXQMDBVUIQNIY9GZ"
	coordinatorSecurityLevel   = 2
	coordinatorIntervalSeconds = 10
	coordinatorMerkleTreeDepth = 18

	genesisSeed    = "BCPCHOJNIRZDMNDFSEEBIBGFQRGZL9PRXJQBSGHQAKCTYXBSDQEQIJ9STDFHFKMVSAZOHKSVIUBSZYBUC"
	genesisAddress = "9QJKPJPYTNPF9AFCLGLMAGXOR9ZIPYTRISKOGJPM9ZKKDXGRXWFJZMQTETDJJOGYEVRMLAOECBPWTUZ9B"

	exitStatusSuccessful = 0
)

// Parameters to override before calling any peer creation function.
var (
	// ParaPoWDifficulty defines the PoW difficulty.
	ParaPoWDifficulty = 3
)

// NodeConfig defines the config of a Hornet node.
type NodeConfig struct {
	// The autopeering seed this peer is going to use.
	AutopeeringSeed string
	// Whether the given node should act as the coordinator in the network.
	Coordinator bool
	// The name of this peer constructed out of the
	Name string
	// The hostname of the entry node.
	EntryNodeHost string
	// The entry node's public key.
	EntryNodePublicKey string
	// The plugins to disable on this node.
	DisabledPlugins string
	// The snapshot file path.
	SnapshotFilePath string
}
