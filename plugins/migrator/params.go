package migrator

import (
	"time"

	"github.com/gohornet/hornet/pkg/node"
	flag "github.com/spf13/pflag"
)

const (
	// CfgMigratorStateFilePath configures the path to the state file of the migrator.
	CfgMigratorStateFilePath = "migrator.stateFilePath"
	// CfgMigratorAPIAddress configures the address of the legacy node API.
	CfgMigratorAPIAddress = "migrator.apiAddress"
	// CfgMigratorAPITimeout configures the timeout of API calls.
	CfgMigratorAPITimeout = "migrator.apiTimeout"
	// CfgMigratorCoordinatorAddress configures the address of the legacy coordinator.
	CfgMigratorCoordinatorAddress = "migrator.coordinatorAddress"
	// CfgMigratorCoordinatorMerkleTreeDepth configures the depth of the Merkle tree of the legacy coordinator.
	CfgMigratorCoordinatorMerkleTreeDepth = "migrator.coordinatorMerkleTreeDepth"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.String(CfgMigratorStateFilePath, "migrator.state", "path to the state file of the migrator")
			fs.String(CfgMigratorAPIAddress, "http://localhost:14265", "address of the legacy node API")
			fs.Duration(CfgMigratorAPITimeout, 5*time.Second, "timeout of API calls")
			fs.String(CfgMigratorCoordinatorAddress, "JFQ999DVN9CBBQX9DSAIQRAFRALIHJMYOXAQSTCJLGA9DLOKIWHJIFQKMCQ9QHWW9RXQMDBVUIQNIY9GZ", "address of the legacy coordinator")
			fs.Int(CfgMigratorCoordinatorMerkleTreeDepth, 18, "depth of the Merkle tree of the coordinator")
			return fs
		}(),
	},
	Masked: nil,
}
