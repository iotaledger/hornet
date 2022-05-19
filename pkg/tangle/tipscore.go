package tangle

import (
	"context"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	iotago "github.com/iotaledger/iota.go/v3"
)

type TipScore int

const (
	TipScoreNotFound TipScore = iota
	TipScoreBelowMaxDepth
	TipScoreYCRIThresholdReached
	TipScoreOCRIThresholdReached
	TipScoreHealthy
)

type TipScoreCalculator struct {
	storage *storage.Storage
	// maxDeltaBlockYoungestConeRootIndexToCMI is the maximum allowed delta
	// value for the YCRI of a given block in relation to the current CMI before it gets lazy.
	maxDeltaBlockYoungestConeRootIndexToCMI milestone.Index
	// maxDeltaBlockOldestConeRootIndexToCMI is the maximum allowed delta
	// value between OCRI of a given block in relation to the current CMI before it gets semi-lazy.
	maxDeltaBlockOldestConeRootIndexToCMI milestone.Index
	// belowMaxDepth is the maximum allowed delta
	// value between OCRI of a given block in relation to the current CMI before it gets lazy.
	belowMaxDepth milestone.Index
}

func NewTipScoreCalculator(storage *storage.Storage, maxDeltaBlockYoungestConeRootIndexToCMI int, maxDeltaBlockOldestConeRootIndexToCMI int, belowMaxDepth int) *TipScoreCalculator {
	return &TipScoreCalculator{
		storage:                                 storage,
		maxDeltaBlockYoungestConeRootIndexToCMI: milestone.Index(maxDeltaBlockYoungestConeRootIndexToCMI),
		maxDeltaBlockOldestConeRootIndexToCMI:   milestone.Index(maxDeltaBlockOldestConeRootIndexToCMI),
		belowMaxDepth:                           milestone.Index(belowMaxDepth),
	}
}

func (t *TipScoreCalculator) TipScore(ctx context.Context, blockID iotago.BlockID, cmi milestone.Index) (TipScore, error) {
	cachedBlockMeta := t.storage.CachedBlockMetadataOrNil(blockID) // meta +1
	if cachedBlockMeta == nil {
		return TipScoreNotFound, nil
	}
	defer cachedBlockMeta.Release(true)

	ycri, ocri, err := dag.ConeRootIndexes(ctx, t.storage, cachedBlockMeta.Retain(), cmi) // meta +1
	if err != nil {
		return TipScoreNotFound, err
	}

	// if the OCRI to CMI delta is over BelowMaxDepth/below-max-depth, then the tip is lazy
	if (cmi - ocri) > t.belowMaxDepth {
		return TipScoreBelowMaxDepth, nil
	}

	// if the CMI to YCRI delta is over maxDeltaBlockYoungestConeRootIndexToCMI, then the tip is lazy
	if (cmi - ycri) > t.maxDeltaBlockYoungestConeRootIndexToCMI {
		return TipScoreYCRIThresholdReached, nil
	}

	// if the OCRI to CMI delta is over maxDeltaBlockOldestConeRootIndexToCMI, the tip is semi-lazy
	if (cmi - ocri) > t.maxDeltaBlockOldestConeRootIndexToCMI {
		return TipScoreOCRIThresholdReached, nil
	}

	return TipScoreHealthy, nil
}
