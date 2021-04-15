package receipt

import (
	"time"

	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// CfgReceiptsBackupEnabled configures whether receipts are additionally stored in the specified folder.
	CfgReceiptsBackupEnabled = "receipts.backup.enabled"
	// CfgReceiptsBackupFolder configures the path to the receipts backup folder.
	CfgReceiptsBackupFolder = "receipts.backup.folder"
	// CfgReceiptsValidatorValidate configures whether to validate receipts.
	CfgReceiptsValidatorValidate = "receipts.validator.validate"
	// CfgReceiptsValidatorIgnoreSoftErrors configures the node to not panic if a soft error is encountered.
	CfgReceiptsValidatorIgnoreSoftErrors = "receipts.validator.ignoreSoftErrors"
	// CfgReceiptsValidatorAPIAddress configures the address of the legacy node API to query for white-flag confirmation data.
	CfgReceiptsValidatorAPIAddress = "receipts.validator.api.address"
	// CfgReceiptsValidatorAPITimeout configures the timeout of API calls.
	CfgReceiptsValidatorAPITimeout = "receipts.validator.api.timeout"
	// CfgReceiptsValidatorCoordinatorAddress configures the address of the legacy coordinator.
	CfgReceiptsValidatorCoordinatorAddress = "receipts.validator.coordinator.address"
	// CfgReceiptsValidatorCoordinatorMerkleTreeDepth configures the depth of the Merkle tree of the legacy coordinator.
	CfgReceiptsValidatorCoordinatorMerkleTreeDepth = "receipts.validator.coordinator.merkleTreeDepth"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.String(CfgReceiptsBackupFolder, "receipts", "path to the receipts backup folder")
			fs.Bool(CfgReceiptsBackupEnabled, false, "whether to backup receipts in the backup folder")
			fs.Bool(CfgReceiptsValidatorIgnoreSoftErrors, false, "whether to ignore soft errors and not panic if one is encountered")
			fs.Bool(CfgReceiptsValidatorValidate, false, "whether to validate receipts")
			fs.String(CfgReceiptsValidatorAPIAddress, "http://localhost:14266", "address of the legacy node API")
			fs.Duration(CfgReceiptsValidatorAPITimeout, 5*time.Second, "timeout of API calls")
			fs.String(CfgReceiptsValidatorCoordinatorAddress, "JFQ999DVN9CBBQX9DSAIQRAFRALIHJMYOXAQSTCJLGA9DLOKIWHJIFQKMCQ9QHWW9RXQMDBVUIQNIY9GZ", "address of the legacy coordinator")
			fs.Int(CfgReceiptsValidatorCoordinatorMerkleTreeDepth, 18, "depth of the Merkle tree of the coordinator")
			return fs
		}(),
	},
	Masked: nil,
}
