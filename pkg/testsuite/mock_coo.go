package testsuite

import (
	"context"
	"crypto/ed25519"
	"fmt"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/keymanager"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/whiteflag"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/builder"
)

type MockCoo struct {
	te *TestEnvironment

	cooPrivateKeys []ed25519.PrivateKey
	keyManager     *keymanager.KeyManager

	lastMilestonePayload   *iotago.Milestone
	lastMilestoneMessageID hornet.BlockID
}

func (coo *MockCoo) LastMilestonePayload() *iotago.Milestone {
	return coo.lastMilestonePayload
}

func (coo *MockCoo) LastMilestoneIndex() milestone.Index {
	lastMilestonePayload := coo.LastMilestonePayload()
	if lastMilestonePayload == nil {
		return 0
	}
	return milestone.Index(lastMilestonePayload.Index)
}

// LastMilestoneID calculates the milestone ID of the last issued milestone.
func (coo *MockCoo) LastMilestoneID() iotago.MilestoneID {
	lastMilestonePayload := coo.LastMilestonePayload()
	if lastMilestonePayload == nil {
		// return null milestone ID
		return iotago.MilestoneID{}
	}

	msIDPtr, err := lastMilestonePayload.ID()
	if err != nil {
		panic(err)
	}

	return *msIDPtr
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

func (coo *MockCoo) LastMilestoneMessageID() hornet.BlockID {
	return coo.lastMilestoneMessageID
}

func (coo *MockCoo) LastMilestoneParents() hornet.BlockIDs {
	lastMilestonePayload := coo.LastMilestonePayload()
	if lastMilestonePayload == nil {
		// return genesis hash
		return hornet.BlockIDs{hornet.NullBlockID()}
	}
	return hornet.BlockIDsFromSliceOfArrays(lastMilestonePayload.Parents)
}

func (coo *MockCoo) storeMessage(message *iotago.Block) hornet.BlockID {
	msg, err := storage.NewBlock(message, serializer.DeSeriModeNoValidation, nil) // no need to validate bytes, they come pre-validated from the coo
	require.NoError(coo.te.TestInterface, err)
	cachedBlock := coo.te.StoreMessage(msg) // block +1, no need to release, since we remember all the messages for later cleanup

	milestonePayload := cachedBlock.Block().Milestone()
	if milestonePayload != nil {
		// message is a milestone
		coo.te.syncManager.SetLatestMilestoneIndex(milestone.Index(milestonePayload.Index))
	}
	return msg.BlockID()
}

func (coo *MockCoo) bootstrap() {
	coo.lastMilestonePayload = nil
	coo.lastMilestoneMessageID = hornet.NullBlockID()
	coo.issueMilestoneOnTips(hornet.BlockIDs{}, true)
}

func (coo *MockCoo) computeWhiteflag(index milestone.Index, timestamp uint32, parents hornet.BlockIDs, lastMilestoneID iotago.MilestoneID) (*whiteflag.WhiteFlagMutations, error) {
	messagesMemcache := storage.NewBlocksMemcache(coo.te.storage.CachedBlock)
	metadataMemcache := storage.NewMetadataMemcache(coo.te.storage.CachedBlockMetadata)
	memcachedTraverserStorage := dag.NewMemcachedTraverserStorage(coo.te.storage, metadataMemcache)

	defer func() {
		// all releases are forced since the cone is referenced and not needed anymore
		memcachedTraverserStorage.Cleanup(true)

		// release all messages at the end
		messagesMemcache.Cleanup(true)

		// Release all message metadata at the end
		metadataMemcache.Cleanup(true)
	}()

	parentsTraverser := dag.NewParentsTraverser(memcachedTraverserStorage)

	// compute merkle tree root
	return whiteflag.ComputeWhiteFlagMutations(context.Background(),
		coo.te.UTXOManager(),
		parentsTraverser,
		messagesMemcache.CachedBlock,
		index,
		timestamp,
		parents,
		lastMilestoneID,
		whiteflag.DefaultWhiteFlagTraversalCondition)
}

func (coo *MockCoo) milestonePayload(parents hornet.BlockIDs) (*iotago.Milestone, error) {

	sortedParents := parents.RemoveDupsAndSortByLexicalOrder()

	milestoneIndex := coo.LastMilestoneIndex() + 1
	milestoneTimestamp := uint32(milestoneIndex * 100)

	mutations, err := coo.computeWhiteflag(milestoneIndex, milestoneTimestamp, sortedParents, coo.LastMilestoneID())
	if err != nil {
		return nil, err
	}

	milestonePayload := iotago.NewMilestone(uint32(milestoneIndex), milestoneTimestamp, coo.te.protoParas.Version, coo.LastMilestoneID(), sortedParents.ToSliceOfArrays(), mutations.InclusionMerkleRoot, mutations.AppliedMerkleRoot)

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
func (coo *MockCoo) issueMilestoneOnTips(tips hornet.BlockIDs, addLastMilestoneAsParent bool) (*storage.Milestone, hornet.BlockID, error) {

	currentIndex := coo.LastMilestoneIndex()
	coo.te.VerifyLMI(currentIndex)
	milestoneIndex := currentIndex + 1

	fmt.Printf("Issue milestone %v\n", milestoneIndex)

	if addLastMilestoneAsParent {
		tips = append(tips, coo.LastMilestoneMessageID())
	}

	milestonePayload, err := coo.milestonePayload(tips)
	if err != nil {
		return nil, nil, err
	}

	msg, err := builder.NewMessageBuilder(coo.te.protoParas.Version).
		ParentsMessageIDs(tips.ToSliceOfArrays()).
		Payload(milestonePayload).
		ProofOfWork(context.Background(), coo.te.protoParas, coo.te.protoParas.MinPoWScore).
		Build()
	if err != nil {
		return nil, nil, err
	}

	milestoneMessageID := coo.storeMessage(msg)
	if err != nil {
		return nil, nil, err
	}
	coo.lastMilestoneMessageID = milestoneMessageID

	coo.te.VerifyLMI(milestoneIndex)
	cachedMilestone := coo.te.storage.CachedMilestoneByIndexOrNil(milestoneIndex) // milestone +1
	require.NotNil(coo.te.TestInterface, cachedMilestone)

	coo.te.Milestones = append(coo.te.Milestones, cachedMilestone)
	coo.lastMilestonePayload = cachedMilestone.Milestone().Milestone()

	return cachedMilestone.Milestone(), milestoneMessageID, nil
}
