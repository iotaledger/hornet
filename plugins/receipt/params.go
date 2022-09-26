package receipt

import (
	"time"

	"github.com/iotaledger/hive.go/core/app"
)

// ParametersReceipts contains the definition of the parameters used by receipts.
type ParametersReceipts struct {
	// Enabled defines whether the receipts plugin is enabled.
	Enabled bool `default:"false" usage:"whether the receipts plugin is enabled"`

	Backup struct {
		// CfgReceiptsBackupEnabled configures whether receipts are additionally stored in the specified folder.
		Enabled bool `default:"false" usage:"whether to backup receipts in the backup folder"`
		// CfgReceiptsBackupPath configures the path to the receipts backup folder.
		Path string `default:"receipts" usage:"path to the receipts backup folder"`
	}

	Validator struct {
		// CfgReceiptsValidatorValidate configures whether to validate receipts.
		Validate bool `default:"false" usage:"whether to validate receipts"`
		// CfgReceiptsValidatorIgnoreSoftErrors configures the node to not panic if a soft error is encountered.
		IgnoreSoftErrors bool `default:"false" usage:"whether to ignore soft errors and not panic if one is encountered"`

		API struct {
			// CfgReceiptsValidatorAPIAddress configures the address of the legacy node API to query for white-flag confirmation data.
			Address string `default:"http://localhost:14266" usage:"address of the legacy node API"`
			// CfgReceiptsValidatorAPITimeout configures the timeout of API calls.
			Timeout time.Duration `default:"5s" usage:"timeout of API calls"`
		} `name:"api"`

		Coordinator struct {
			// Address configures the address of the legacy coordinator.
			Address string `default:"UDYXTZBE9GZGPM9SSQV9LTZNDLJIZMPUVVXYXFYVBLIEUHLSEWFTKZZLXYRHHWVQV9MNNX9KZC9D9UZWZ" usage:"address of the legacy coordinator"`
			// MerkleTreeDepth configures the depth of the Merkle tree of the legacy coordinator.
			MerkleTreeDepth int `default:"24" usage:"depth of the Merkle tree of the coordinator"`
		}
	}
}

var ParamsReceipts = &ParametersReceipts{}

var params = &app.ComponentParams{
	Params: map[string]any{
		"receipts": ParamsReceipts,
	},
	Masked: nil,
}
