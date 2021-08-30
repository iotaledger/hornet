package receipt

import (
	"time"

	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// CfgReceiptsBackupEnabled configures whether receipts are additionally stored in the specified folder.
	CfgReceiptsBackupEnabled = "receipts.backup.enabled"
	// CfgReceiptsBackupPath configures the path to the receipts backup folder.
	CfgReceiptsBackupPath = "receipts.backup.path"
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
			fs.Bool(CfgReceiptsBackupEnabled, false, "whether to backup receipts in the backup folder")
			fs.String(CfgReceiptsBackupPath, "receipts", "path to the receipts backup folder")
			fs.Bool(CfgReceiptsValidatorValidate, false, "whether to validate receipts")
			fs.Bool(CfgReceiptsValidatorIgnoreSoftErrors, false, "whether to ignore soft errors and not panic if one is encountered")
			fs.String(CfgReceiptsValidatorAPIAddress, "http://localhost:14266", "address of the legacy node API")
			fs.Duration(CfgReceiptsValidatorAPITimeout, 5*time.Second, "timeout of API calls")
			fs.String(CfgReceiptsValidatorCoordinatorAddress, "UDYXTZBE9GZGPM9SSQV9LTZNDLJIZMPUVVXYXFYVBLIEUHLSEWFTKZZLXYRHHWVQV9MNNX9KZC9D9UZWZ", "address of the legacy coordinator")
			fs.Int(CfgReceiptsValidatorCoordinatorMerkleTreeDepth, 24, "depth of the Merkle tree of the coordinator")
			return fs
		}(),
	},
	Masked: nil,
}
