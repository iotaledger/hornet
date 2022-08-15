package testsuite

import (
	"context"
	"crypto/ed25519"
	"fmt"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hornet/v2/pkg/dag"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/whiteflag"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/builder"
	"github.com/iotaledger/iota.go/v3/keymanager"
)

type MockCoo struct {
	te *TestEnvironment

	cooPrivateKeys        []ed25519.PrivateKey
	genesisMilestoneIndex iotago.MilestoneIndex
	keyManager            *keymanager.KeyManager

	lastMilestonePayload *iotago.Milestone
	lastMilestoneBlockID iotago.BlockID
}

func (coo *MockCoo) LastMilestonePayload() *iotago.Milestone {
	return coo.lastMilestonePayload
}

func (coo *MockCoo) LastMilestoneIndex() iotago.MilestoneIndex {
	lastMilestonePayload := coo.LastMilestonePayload()
	if lastMilestonePayload == nil {
		return 0
	}

	return lastMilestonePayload.Index
}

// LastMilestoneID calculates the milestone ID of the last issued milestone.
func (coo *MockCoo) LastMilestoneID() iotago.MilestoneID {
	lastMilestonePayload := coo.LastMilestonePayload()
	if lastMilestonePayload == nil {
		// return null milestone ID
		return iotago.MilestoneID{}
	}

	msID, err := lastMilestonePayload.ID()
	if err != nil {
		panic(err)
	}

	return msID
}

// LastPreviousMilestoneID returns the PreviousMilestoneID of the last issued milestone.
func (coo *MockCoo) LastPreviousMilestoneID() iotago.MilestoneID {
	lastMilestonePayload := coo.LastMilestonePayload()
	if lastMilestonePayload == nil {
		// return null milestone ID
		return iotago.MilestoneID{}
	}

	return lastMilestonePayload.PreviousMilestoneID
}

func (coo *MockCoo) LastMilestoneBlockID() iotago.BlockID {
	return coo.lastMilestoneBlockID
}

func (coo *MockCoo) LastMilestoneParents() iotago.BlockIDs {
	lastMilestonePayload := coo.LastMilestonePayload()
	if lastMilestonePayload == nil {
		// return genesis hash
		return iotago.BlockIDs{iotago.EmptyBlockID()}
	}

	return lastMilestonePayload.Parents
}

func (coo *MockCoo) storeBlock(iotaBlock *iotago.Block) iotago.BlockID {
	block, err := storage.NewBlock(iotaBlock, serializer.DeSeriModeNoValidation, nil) // no need to validate bytes, they come pre-validated from the coo
	require.NoError(coo.te.TestInterface, err)
	cachedBlock := coo.te.StoreBlock(block) // iotaBlock +1, no need to release, since we remember all the blocks for later cleanup

	milestonePayload := cachedBlock.Block().Milestone()
	if milestonePayload != nil {
		// iotaBlock is a milestone
		coo.te.syncManager.SetLatestMilestoneIndex(milestonePayload.Index)
	}

	return block.BlockID()
}

func (coo *MockCoo) bootstrap() {
	coo.lastMilestonePayload = nil
	coo.lastMilestoneBlockID = iotago.EmptyBlockID()
	_, _, err := coo.issueMilestoneOnTips(iotago.BlockIDs{}, true)
	require.NoError(coo.te.TestInterface, err)
}

func (coo *MockCoo) computeWhiteflag(index iotago.MilestoneIndex, timestamp uint32, parents iotago.BlockIDs, lastMilestoneID iotago.MilestoneID) (*whiteflag.WhiteFlagMutations, error) {
	blocksMemcache := storage.NewBlocksMemcache(coo.te.storage.CachedBlock)
	metadataMemcache := storage.NewMetadataMemcache(coo.te.storage.CachedBlockMetadata)
	memcachedTraverserStorage := dag.NewMemcachedTraverserStorage(coo.te.storage, metadataMemcache)

	defer func() {
		// all releases are forced since the cone is referenced and not needed anymore
		memcachedTraverserStorage.Cleanup(true)

		// release all blocks at the end
		blocksMemcache.Cleanup(true)

		// Release all block metadata at the end
		metadataMemcache.Cleanup(true)
	}()

	parentsTraverser := dag.NewParentsTraverser(memcachedTraverserStorage)

	// compute merkle tree root
	return whiteflag.ComputeWhiteFlagMutations(context.Background(),
		coo.te.UTXOManager(),
		parentsTraverser,
		blocksMemcache.CachedBlock,
		index,
		timestamp,
		parents,
		lastMilestoneID,
		coo.genesisMilestoneIndex,
		whiteflag.DefaultWhiteFlagTraversalCondition)
}

func (coo *MockCoo) milestonePayload(parents iotago.BlockIDs) (*iotago.Milestone, error) {

	sortedParents := parents.RemoveDupsAndSort()

	milestoneIndex := coo.LastMilestoneIndex() + 1
	milestoneTimestamp := milestoneIndex * 100

	mutations, err := coo.computeWhiteflag(milestoneIndex, milestoneTimestamp, sortedParents, coo.LastMilestoneID())
	if err != nil {
		return nil, err
	}

	milestonePayload := iotago.NewMilestone(milestoneIndex, milestoneTimestamp, coo.te.protoParams.Version, coo.LastMilestoneID(), sortedParents, mutations.InclusionMerkleRoot, mutations.AppliedMerkleRoot)

	keymapping := coo.keyManager.MilestonePublicKeyMappingForMilestoneIndex(milestoneIndex, coo.cooPrivateKeys, len(coo.cooPrivateKeys))

	pubKeys := []iotago.MilestonePublicKey{}
	pubKeysSet := iotago.MilestonePublicKeySet{}
	for k := range keymapping {
		pubKeys = append(pubKeys, k)
		pubKeysSet[k] = struct{}{}
	}

	err = milestonePayload.Sign(pubKeys, iotago.InMemoryEd25519MilestoneSigner(keymapping))
	if err != nil {
		return nil, err
	}

	err = milestonePayload.VerifySignatures(len(coo.cooPrivateKeys), pubKeysSet)
	if err != nil {
		return nil, err
	}

	return milestonePayload, nil
}

// issueMilestoneOnTips creates a milestone on top of the given tips.
func (coo *MockCoo) issueMilestoneOnTips(tips iotago.BlockIDs, addLastMilestoneAsParent bool) (*storage.Milestone, iotago.BlockID, error) {

	currentIndex := coo.LastMilestoneIndex()
	coo.te.VerifyLMI(currentIndex)
	milestoneIndex := currentIndex + 1

	fmt.Printf("Issue milestone %v\n", milestoneIndex)

	if addLastMilestoneAsParent {
		tips = append(tips, coo.LastMilestoneBlockID())
	}

	milestonePayload, err := coo.milestonePayload(tips)
	if err != nil {
		return nil, iotago.EmptyBlockID(), err
	}

	iotaBlock, err := builder.NewBlockBuilder().
		ProtocolVersion(coo.te.protoParams.Version).
		Parents(tips).
		Payload(milestonePayload).
		ProofOfWork(context.Background(), coo.te.protoParams, float64(coo.te.protoParams.MinPoWScore)).
		Build()
	if err != nil {
		return nil, iotago.EmptyBlockID(), err
	}

	milestoneBlockID := coo.storeBlock(iotaBlock)
	coo.lastMilestoneBlockID = milestoneBlockID

	coo.te.VerifyLMI(milestoneIndex)
	cachedMilestone := coo.te.storage.CachedMilestoneByIndexOrNil(milestoneIndex) // milestone +1
	require.NotNil(coo.te.TestInterface, cachedMilestone)

	coo.te.Milestones = append(coo.te.Milestones, cachedMilestone)
	coo.lastMilestonePayload = cachedMilestone.Milestone().Milestone()

	return cachedMilestone.Milestone(), milestoneBlockID, nil
}
