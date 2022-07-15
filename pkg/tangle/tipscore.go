package tangle

import (
	"context"

	"github.com/iotaledger/hornet/v2/pkg/dag"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
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
	maxDeltaBlockYoungestConeRootIndexToCMI iotago.MilestoneIndex
	// maxDeltaBlockOldestConeRootIndexToCMI is the maximum allowed delta
	// value between OCRI of a given block in relation to the current CMI before it gets semi-lazy.
	maxDeltaBlockOldestConeRootIndexToCMI iotago.MilestoneIndex
	// belowMaxDepth is the maximum allowed delta
	// value between OCRI of a given block in relation to the current CMI before it gets lazy.
	belowMaxDepth iotago.MilestoneIndex
}

func NewTipScoreCalculator(storage *storage.Storage, maxDeltaBlockYoungestConeRootIndexToCMI int, maxDeltaBlockOldestConeRootIndexToCMI int, belowMaxDepth int) *TipScoreCalculator {
	return &TipScoreCalculator{
		storage:                                 storage,
		maxDeltaBlockYoungestConeRootIndexToCMI: syncmanager.MilestoneIndexDelta(maxDeltaBlockYoungestConeRootIndexToCMI),
		maxDeltaBlockOldestConeRootIndexToCMI:   syncmanager.MilestoneIndexDelta(maxDeltaBlockOldestConeRootIndexToCMI),
		belowMaxDepth:                           syncmanager.MilestoneIndexDelta(belowMaxDepth),
	}
}

func (t *TipScoreCalculator) TipScore(ctx context.Context, blockID iotago.BlockID, cmi iotago.MilestoneIndex) (TipScore, error) {
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
