package tipselection

import (
	"crypto/rand"
	"math"
	"time"

	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	tanglePlugin "github.com/gohornet/hornet/plugins/tangle"
)

var ErrNodeNotSynced = errors.New("node is not synchronized")
var ErrMilestoneNotFound = errors.New("milestone not found")
var ErrDepthTooHigh = errors.New("depth is too high")
var ErrReferenceNotValid = errors.New("reference transaction is not valid")
var ErrReferenceNotConsistent = errors.New("reference transaction is not consistent")

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

func SelectTips(depth uint, reference *trinary.Hash) ([]trinary.Hash, *TipSelStats, error) {
	if int(depth) > maxDepth {
		return nil, nil, errors.Wrapf(ErrDepthTooHigh, "max supported is: %d", maxDepth)
	}

	tangle.ReadLockLedger()
	defer tangle.ReadUnlockLedger()

	if !tangle.IsNodeSynced() {
		return nil, nil, ErrNodeNotSynced
	}

	lastSolidIndex := tangle.GetSolidMilestoneIndex()
	msWalkStartIndex := milestone_index.MilestoneIndex(math.Max(0, float64(lastSolidIndex-milestone_index.MilestoneIndex(depth))))

	// either take the valid wanted ms at the given depth or use the initial snapshot milestone
	msWalkStartIndex = milestone_index.MilestoneIndex(math.Max(float64(msWalkStartIndex), float64(tangle.GetSnapshotInfo().SnapshotIndex+1)))
	if msWalkStartIndex > lastSolidIndex {
		msWalkStartIndex = lastSolidIndex
	}
	ms, err := tangle.GetMilestone(msWalkStartIndex)
	if err != nil {
		return nil, nil, err
	}

	if ms == nil {
		return nil, nil, errors.Wrapf(ErrMilestoneNotFound, "index: %d", msWalkStartIndex)
	}

	// record stats
	start := time.Now()
	walkStats := &TipSelStats{EntryPoint: ms.GetTailHash()}

	// compute the range in which we allow approvers to reference transactions in
	lowerAllowedSnapshotIndex := int(math.Max(float64(int(tangle.GetSolidMilestoneIndex())-maxDepth), float64(0)))

	diff := map[trinary.Hash]int64{}
	approved := map[trinary.Hash]struct{}{}
	solidEntryPoints := tangle.GetSolidEntryPointsHashes()
	for _, selectEntryPoint := range solidEntryPoints {
		approved[selectEntryPoint] = struct{}{}
	}

	// it is safe to cache the below max depth flag of transactions as long as the same milestone is solid.
	tanglePlugin.BelowDepthMemoizationCache.ResetIfNewerMilestone(tangle.GetSolidMilestoneIndex())

	// check whether the given reference tx is valid for the walk
	var refBundle *tangle.Bundle
	if reference != nil {
		refTx, err := tangle.GetTransaction(*reference)
		if err != nil {
			log.Panic(err)
		}
		if refTx == nil {
			return nil, nil, errors.Wrap(ErrReferenceNotValid, "transaction doesn't exist")
		}
		if !refTx.IsSolid() {
			return nil, nil, errors.Wrap(ErrReferenceNotValid, "transaction is not solid")
		}
		if !refTx.IsTail() {
			return nil, nil, errors.Wrap(ErrReferenceNotValid, "transaction is not a tail transaction")
		}

		bundleBucket, err := tangle.GetBundleBucket(refTx.Tx.Bundle)
		if err != nil {
			return nil, nil, err
		}

		bundle := bundleBucket.GetBundleOfTailTransaction(refTx.GetHash())
		if bundle == nil {
			// this should never happen if Hornet is programmed correctly
			if refTx.Tx.CurrentIndex == 0 {
				log.Panicf("reference transaction is a tail but there's no bundle instance")
			}
			return nil, nil, errors.Wrap(ErrReferenceNotValid, "bundle tail not yet known (bundle is complete)")
		}
		if !bundle.IsComplete() {
			return nil, nil, errors.Wrap(ErrReferenceNotValid, "bundle is not complete")
		}
		if !bundle.IsValid() {
			return nil, nil, errors.Wrap(ErrReferenceNotValid, "bundle is invalid")
		}
		refBundle = bundle

		if tanglePlugin.IsBelowMaxDepth(bundle.GetTail(), lowerAllowedSnapshotIndex) {
			return nil, nil, errors.Wrap(ErrReferenceNotValid, "transaction is below max depth")
		}
		walkStats.Reference = reference
	}

	tips := trinary.Hashes{}
	for i := 0; i < 2; i++ {
		var selected trinary.Hash
		// on the second walk, use the given reference as a starting point
		if i == 1 && reference != nil {
			// check whether the reference transaction itself is consistent with the first walk's diff
			if !tanglePlugin.CheckConsistencyOfConeAndMutateDiff(refBundle.GetTailHash(), approved, diff) {
				return nil, nil, errors.Wrapf(ErrReferenceNotConsistent, "with milestone %d", tangle.GetSolidMilestoneIndex())
			}
			selected = *reference
		} else {
			if ms.GetHash() == consts.NullHashTrytes {
				selected = ms.GetHash()
			} else {
				selected = ms.GetTail().GetHash()
			}
		}
		for {
			walkStats.StepsTaken++
			previousSelected := selected
			approvers, err := tangle.GetApprovers(selected)
			if err != nil {
				return nil, nil, err
			}

			if len(approvers.GetHashes()) == 0 {
				break
			}

			approverHashes := approvers.GetHashes()
			for len(approverHashes) != 0 {
				b := make([]byte, 1)
				_, err := rand.Read(b)
				if err != nil {
					return nil, nil, err
				}
				var candidateIndex int
				if len(approverHashes) == 1 {
					candidateIndex = 0
				} else {
					candidateIndex = int(b[0]) % len(approverHashes)
				}
				candidateHash := approverHashes[candidateIndex]

				// skip validating the tx if we already approved it
				if _, alreadyApproved := approved[candidateHash]; alreadyApproved {
					walkStats.StepsJumped++
					selected = candidateHash
					break
				}

				// check whether we determined by a previous tip-sel whether this
				// transaction references an invalid bundle
				if tanglePlugin.RefsAnInvalidBundleCache.Contains(candidateHash) {
					approverHashes = removeElementAtIndex(approverHashes, candidateIndex)
					continue
				}

				walkStats.Evaluated++

				candidateTx, err := tangle.GetTransaction(candidateHash)
				if err != nil {
					return nil, nil, err
				}

				if !candidateTx.IsSolid() {
					approverHashes = removeElementAtIndex(approverHashes, candidateIndex)
					continue
				}

				bundleBucket, err := tangle.GetBundleBucket(candidateTx.Tx.Bundle)
				if err != nil {
					return nil, nil, err
				}

				// a transaction can be within multiple bundle instances, because it is possible
				// that transactions are reattached "above" the origin bundle but pointing (via trunk)
				// to some transactions of the origin bundle.
				bundles := bundleBucket.GetBundlesOfTransaction(candidateTx.GetHash())

				// isn't in any bundle instance
				if len(bundles) == 0 {
					approverHashes = removeElementAtIndex(approverHashes, candidateIndex)
					continue
				}

				// randomly select a bundle to which this transaction belongs to
				var bundle *tangle.Bundle
				if len(bundles) == 1 {
					bundle = bundles[0]
				} else {
					bundle = bundles[int(b[0])%len(bundles)]
				}

				if bundle == nil || !bundle.IsComplete() {
					approverHashes = removeElementAtIndex(approverHashes, candidateIndex)
					continue
				}

				if !bundle.IsValid() {
					tanglePlugin.RefsAnInvalidBundleCache.Set(candidateHash, true)
					approverHashes = removeElementAtIndex(approverHashes, candidateIndex)
					continue
				}

				if tanglePlugin.IsBelowMaxDepth(bundle.GetTail(), lowerAllowedSnapshotIndex) {
					approverHashes = removeElementAtIndex(approverHashes, candidateIndex)
					continue
				}

				// if the transaction has already been confirmed by the current solid or previous
				// milestone, it is automatically consistent with our current walking diff
				confirmed, at := candidateTx.GetConfirmed()
				// TODO: the second condition can be removed once the solidifier ensures, that the entire
				// ledger update process is write locked
				if !confirmed {
					if at > tangle.GetSolidMilestoneIndex() {
						log.Panicf("transaction %s was confirmed by a newer milestone %d", candidateTx.GetHash(), at)
					}
					// check whether the bundle's approved cone is consistent with our current diff
					if !tanglePlugin.CheckConsistencyOfConeAndMutateDiff(bundle.GetTailHash(), approved, diff) {
						approverHashes = removeElementAtIndex(approverHashes, candidateIndex)
						continue
					}
				}

				// cache the hashes of txs which we approve, so we don't recheck them
				for _, txHash := range bundle.GetTransactionHashes() {
					approved[txHash] = struct{}{}
				}

				// auto jump to tail of bundle
				selected = bundle.GetTailHash()
				break
			}
			if previousSelected == selected {
				break
			}
		}
		tips = append(tips, selected)
	}

	walkStats.Duration = time.Now().Sub(start)
	walkStats.GlobalBelowMaxDepthCacheHitRatio = tanglePlugin.BelowDepthMemoizationCache.CacheHitRatio()
	Events.TipSelPerformed.Trigger(walkStats)
	return tips, walkStats, nil
}

func removeElementAtIndex(s []trinary.Hash, index int) []trinary.Hash {
	s[index] = s[len(s)-1]
	return s[:len(s)-1]
}
