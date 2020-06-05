package tipselection

import (
	"time"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/iotaledger/iota.go/trinary"
)

// TipSelectionFunc is a function which performs a tipselection and returns two tips.
type TipSelectionFunc = func(depth uint, reference *hornet.Hash) (hornet.Hashes, *TipSelStats, error)

// TipSelStats holds the stats for a tipselection run.
type TipSelStats struct {
	// The duration of the tip-selection for both walks.
	Duration time.Duration `json:"duration"`
	// The entry point of the tip-selection.
	EntryPoint trinary.Hash `json:"entry_point"`
	// The optional supplied reference transaction hash.
	Reference *trinary.Hash `json:"reference"`
	// The used depth for the tip-selection.
	Depth uint64 `json:"depth"`
	// The amount of steps taken, respectively transactions walked towards the present of the graph.
	StepsTaken uint64 `json:"steps_taken"`
	// The amount of steps jumped, meaning approvers selected without validating, as they were
	// walked/validated into by the previous walk.
	StepsJumped uint64 `json:"steps_jumped"`
	// The amount of transactions which were evaluated.
	Evaluated uint64 `json:"evaluated"`
	// Represents the cache hit ration for every call to belowMaxDepth globally over all tip-selections.
	GlobalBelowMaxDepthCacheHitRatio float64 `json:"global_below_max_depth_cache_hit_ratio"`
}
