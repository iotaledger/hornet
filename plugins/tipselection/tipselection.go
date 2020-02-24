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

	if !tangle.IsNodeSyncedWithThreshold() {
		return nil, nil, ErrNodeNotSynced
	}

	lastSolidIndex := tangle.GetSolidMilestoneIndex()
	msWalkStartIndex := milestone_index.MilestoneIndex(math.Max(0, float64(lastSolidIndex-milestone_index.MilestoneIndex(depth))))

	// either take the valid wanted ms at the given depth or use the initial snapshot milestone
	msWalkStartIndex = milestone_index.MilestoneIndex(math.Max(float64(msWalkStartIndex), float64(tangle.GetSnapshotInfo().SnapshotIndex+1)))
	if msWalkStartIndex > lastSolidIndex {
		msWalkStartIndex = lastSolidIndex
	}

	cachedMs := tangle.GetMilestoneOrNil(msWalkStartIndex) // bundle +1
	if cachedMs == nil {
		return nil, nil, errors.Wrapf(ErrMilestoneNotFound, "index: %d", msWalkStartIndex)
	}
	defer cachedMs.Release() // bundle -1

	// record stats
	start := time.Now()
	walkStats := &TipSelStats{EntryPoint: cachedMs.GetBundle().GetTailHash()}

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
	var cachedRefBundle *tangle.CachedBundle
	if reference != nil {
		cachedRefTx := tangle.GetCachedTransactionOrNil(*reference) // tx +1
		if cachedRefTx == nil {
			return nil, nil, errors.Wrap(ErrReferenceNotValid, "transaction doesn't exist")
		}

		if !cachedRefTx.GetMetadata().IsSolid() {
			cachedRefTx.Release() // tx -1
			return nil, nil, errors.Wrap(ErrReferenceNotValid, "transaction is not solid")
		}

		if !cachedRefTx.GetTransaction().IsTail() {
			cachedRefTx.Release() // tx -1
			return nil, nil, errors.Wrap(ErrReferenceNotValid, "transaction is not a tail transaction")
		}

		cachedBndl := tangle.GetBundleOfTailTransactionOrNil(cachedRefTx.GetTransaction().GetHash()) // bundle +1
		if cachedBndl == nil {
			// this should never happen if HORNET is programmed correctly
			if cachedRefTx.GetTransaction().Tx.CurrentIndex == 0 {
				log.Panicf("reference transaction is a tail but there's no bundle instance")
			}
			cachedRefTx.Release() // tx -1
			return nil, nil, errors.Wrap(ErrReferenceNotValid, "bundle tail not yet known (bundle is complete)")
		}

		cachedRefTx.Release() // tx -1

		if !cachedBndl.GetBundle().IsValid() {
			cachedBndl.Release() // bundle -1
			return nil, nil, errors.Wrap(ErrReferenceNotValid, "bundle is invalid")
		}
		cachedRefBundle = cachedBndl

		if tanglePlugin.IsBelowMaxDepth(cachedBndl.GetBundle().GetTail(), lowerAllowedSnapshotIndex) { // tx pass +1
			cachedBndl.Release() // bundle -1
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
			if !tanglePlugin.CheckConsistencyOfConeAndMutateDiff(cachedRefBundle.GetBundle().GetTailHash(), approved, diff) {
				cachedRefBundle.Release() // bundle -1
				return nil, nil, errors.Wrapf(ErrReferenceNotConsistent, "with milestone %d", tangle.GetSolidMilestoneIndex())
			}
			cachedRefBundle.Release() // bundle -1
			selected = *reference
		} else {
			if cachedMs.GetBundle().GetHash() == consts.NullHashTrytes {
				selected = cachedMs.GetBundle().GetHash()
			} else {
				cachedMsTailTx := cachedMs.GetBundle().GetTail() // tx +1
				selected = cachedMsTailTx.GetTransaction().GetHash()
				cachedMsTailTx.Release() // tx -1
			}
		}
		for {
			walkStats.StepsTaken++
			previousSelected := selected

			approverHashes := tangle.GetApproverHashes(selected)
			if len(approverHashes) == 0 {
				break
			}

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
				if tanglePlugin.ContainsInvalidBundleReference(candidateHash) {
					approverHashes = removeElementAtIndex(approverHashes, candidateIndex)
					continue
				}

				walkStats.Evaluated++

				cachedCandidateTx := tangle.GetCachedTransactionOrNil(candidateHash) // tx +1

				if cachedCandidateTx == nil {
					approverHashes = removeElementAtIndex(approverHashes, candidateIndex)
					continue
				}

				if !cachedCandidateTx.GetMetadata().IsSolid() {
					approverHashes = removeElementAtIndex(approverHashes, candidateIndex)
					cachedCandidateTx.Release() // tx -1
					continue
				}

				// a transaction can be within multiple bundle instances, because it is possible
				// that transactions are reattached "above" the origin bundle but pointing (via trunk)
				// to some transactions of the origin bundle.
				cachedBndls := tangle.GetBundlesOfTransactionOrNil(cachedCandidateTx.GetTransaction().GetHash()) // bundle +1

				// isn't in any bundle instance
				if cachedBndls == nil {
					approverHashes = removeElementAtIndex(approverHashes, candidateIndex)
					cachedCandidateTx.Release() // tx -1
					continue
				}

				// randomly select a bundle to which this transaction belongs to
				var cachedBndl *tangle.CachedBundle
				if len(cachedBndls) == 1 {
					cachedBndl = cachedBndls[0]
				} else {
					randomIndex := int(b[0]) % len(cachedBndls)
					cachedBndl = cachedBndls[randomIndex]

					// Release unused bundles
					for i := 0; i < len(cachedBndls); i++ {
						if i != randomIndex {
							cachedBndls[i].Release() // bundle -1
						}
					}
				}

				if cachedBndl == nil {
					approverHashes = removeElementAtIndex(approverHashes, candidateIndex)
					cachedCandidateTx.Release() // tx -1
					continue
				}

				if !cachedBndl.GetBundle().IsValid() {
					tanglePlugin.PutInvalidBundleReference(candidateHash)
					approverHashes = removeElementAtIndex(approverHashes, candidateIndex)
					cachedCandidateTx.Release() // tx -1
					cachedBndl.Release()        // bundle -1
					continue
				}
				if tanglePlugin.IsBelowMaxDepth(cachedBndl.GetBundle().GetTail(), lowerAllowedSnapshotIndex) { // tx pass +1
					approverHashes = removeElementAtIndex(approverHashes, candidateIndex)
					cachedCandidateTx.Release() // tx -1
					cachedBndl.Release()        // bundle -1
					continue
				}

				// if the transaction has already been confirmed by the current solid or previous
				// milestone, it is automatically consistent with our current walking diff
				confirmed, at := cachedCandidateTx.GetMetadata().GetConfirmed()
				// TODO: the second condition can be removed once the solidifier ensures, that the entire
				// ledger update process is write locked
				if !confirmed {
					if at > tangle.GetSolidMilestoneIndex() {
						log.Panicf("transaction %s was confirmed by a newer milestone %d", cachedCandidateTx.GetTransaction().GetHash(), at)
					}
					// check whether the bundle's approved cone is consistent with our current diff
					if !tanglePlugin.CheckConsistencyOfConeAndMutateDiff(cachedBndl.GetBundle().GetTailHash(), approved, diff) {
						approverHashes = removeElementAtIndex(approverHashes, candidateIndex)
						cachedCandidateTx.Release() // tx -1
						cachedBndl.Release()        // bundle -1
						continue
					}
				}
				cachedCandidateTx.Release() // tx -1

				// cache the hashes of txs which we approve, so we don't recheck them
				for _, txHash := range cachedBndl.GetBundle().GetTransactionHashes() {
					approved[txHash] = struct{}{}
				}

				// auto jump to tail of bundle
				selected = cachedBndl.GetBundle().GetTailHash()
				cachedBndl.Release() // bundle -1
				break
			}
			if previousSelected == selected {
				break
			}
		}
		tips = append(tips, selected)
	}

	walkStats.Duration = time.Since(start)
	walkStats.GlobalBelowMaxDepthCacheHitRatio = tanglePlugin.BelowDepthMemoizationCache.CacheHitRatio()
	Events.TipSelPerformed.Trigger(walkStats)
	return tips, walkStats, nil
}

func removeElementAtIndex(s []trinary.Hash, index int) []trinary.Hash {
	s[index] = s[len(s)-1]
	return s[:len(s)-1]
}
