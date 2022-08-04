package pruning

import (
	"time"

	"github.com/iotaledger/hive.go/core/app"
)

// ParametersPruning contains the definition of the parameters used by pruning.
type ParametersPruning struct {
	Milestones struct {
		// Enabled defines whether to delete old block data from the database based on maximum milestones to keep
		Enabled bool `default:"false" usage:"whether to delete old block data from the database based on maximum milestones to keep"`
		// MaxMilestonesToKeep defines the maximum amount of milestone cones to keep in the database
		MaxMilestonesToKeep int `default:"60480" usage:"maximum amount of milestone cones to keep in the database"`
	}
	Size struct {
		// Enabled defines whether to delete old block data from the database based on maximum database size
		Enabled bool `default:"true" usage:"whether to delete old block data from the database based on maximum database size"`
		// TargetSize defines the target size of the database
		TargetSize string `default:"30GB" usage:"target size of the database"`
		// ThresholdPercentage defines the percentage the database size gets reduced if the target size is reached
		ThresholdPercentage float64 `default:"10.0" usage:"the percentage the database size gets reduced if the target size is reached"`
		// CooldownTime defines the cooldown time between two pruning by database size events
		CooldownTime time.Duration `default:"5m" usage:"cooldown time between two pruning by database size events"`
	}

	// PruneReceipts defines whether to delete old receipts data from the database
	PruneReceipts bool `default:"false" usage:"whether to delete old receipts data from the database"`
}

var ParamsPruning = &ParametersPruning{}

var params = &app.ComponentParams{
	Params: map[string]any{
		"pruning": ParamsPruning,
	},
	Masked: nil,
}
