package tangle

import (
	"context"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
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
	// maxDeltaMsgYoungestConeRootIndexToCMI is the maximum allowed delta
	// value for the YCRI of a given message in relation to the current CMI before it gets lazy.
	maxDeltaMsgYoungestConeRootIndexToCMI milestone.Index
	// maxDeltaMsgOldestConeRootIndexToCMI is the maximum allowed delta
	// value between OCRI of a given message in relation to the current CMI before it gets semi-lazy.
	maxDeltaMsgOldestConeRootIndexToCMI milestone.Index
	// belowMaxDepth is the maximum allowed delta
	// value between OCRI of a given message in relation to the current CMI before it gets lazy.
	belowMaxDepth milestone.Index
}

func NewTipScoreCalculator(storage *storage.Storage, maxDeltaMsgYoungestConeRootIndexToCMI int, maxDeltaMsgOldestConeRootIndexToCMI int, belowMaxDepth int) *TipScoreCalculator {
	return &TipScoreCalculator{
		storage:                               storage,
		maxDeltaMsgYoungestConeRootIndexToCMI: milestone.Index(maxDeltaMsgYoungestConeRootIndexToCMI),
		maxDeltaMsgOldestConeRootIndexToCMI:   milestone.Index(maxDeltaMsgOldestConeRootIndexToCMI),
		belowMaxDepth:                         milestone.Index(belowMaxDepth),
	}
}

func (t *TipScoreCalculator) TipScore(ctx context.Context, blockID hornet.BlockID, cmi milestone.Index) (TipScore, error) {
	cachedBlockMeta := t.storage.CachedMessageMetadataOrNil(blockID) // meta +1
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

	// if the CMI to YCRI delta is over maxDeltaMsgYoungestConeRootIndexToCMI, then the tip is lazy
	if (cmi - ycri) > t.maxDeltaMsgYoungestConeRootIndexToCMI {
		return TipScoreYCRIThresholdReached, nil
	}

	// if the OCRI to CMI delta is over maxDeltaMsgOldestConeRootIndexToCMI, the tip is semi-lazy
	if (cmi - ocri) > t.maxDeltaMsgOldestConeRootIndexToCMI {
		return TipScoreOCRIThresholdReached, nil
	}

	return TipScoreHealthy, nil
}
