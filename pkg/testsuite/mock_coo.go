package testsuite

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"time"

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

	LastMilestoneIndex     milestone.Index
	LastMilestoneTimestamp uint32
	LastMilestoneID        iotago.MilestoneID

	LastMilestoneMessageID hornet.MessageID
}

func (coo *MockCoo) storeMessage(message *iotago.Message) hornet.MessageID {
	msg, err := storage.NewMessage(message, serializer.DeSeriModeNoValidation, nil) // no need to validate bytes, they come pre-validated from the coo
	require.NoError(coo.te.TestInterface, err)
	cachedMsg := coo.te.StoreMessage(msg) // message +1, no need to release, since we remember all the messages for later cleanup

	ms := cachedMsg.Message().Milestone()
	if ms != nil {
		coo.te.syncManager.SetLatestMilestoneIndex(milestone.Index(ms.Index))
	}
	return msg.MessageID()
}

func (coo *MockCoo) bootstrap() {
	coo.LastMilestoneMessageID = hornet.NullMessageID()
	coo.LastMilestoneID = iotago.MilestoneID{}
	coo.issueMilestoneOnTips(hornet.MessageIDs{coo.LastMilestoneMessageID}, false)
}

func (coo *MockCoo) computeWhiteflag(index milestone.Index, timestamp uint32, parents hornet.MessageIDs, lastMilestoneID iotago.MilestoneID) (*whiteflag.WhiteFlagMutations, error) {
	messagesMemcache := storage.NewMessagesMemcache(coo.te.storage.CachedMessage)
	metadataMemcache := storage.NewMetadataMemcache(coo.te.storage.CachedMessageMetadata)
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
		messagesMemcache.CachedMessage,
		coo.te.protoParas.NetworkID(),
		index,
		timestamp,
		parents,
		lastMilestoneID,
		whiteflag.DefaultWhiteFlagTraversalCondition)
}

func (coo *MockCoo) milestonePayload(parents hornet.MessageIDs) (*iotago.Milestone, error) {

	sortedParents := parents.RemoveDupsAndSortByLexicalOrder()

	milestoneIndex := coo.LastMilestoneIndex + 1
	milestoneTimestamp := uint32(time.Now().Unix())

	mutations, err := coo.computeWhiteflag(milestoneIndex, milestoneTimestamp, sortedParents, coo.LastMilestoneID)
	if err != nil {
		return nil, err
	}

	payload, err := iotago.NewMilestone(uint32(milestoneIndex), milestoneTimestamp, coo.LastMilestoneID, sortedParents.ToSliceOfArrays(), mutations.ConfirmedMerkleRoot, mutations.AppliedMerkleRoot)
	if err != nil {
		return nil, err
	}

	keymapping := coo.keyManager.MilestonePublicKeyMappingForMilestoneIndex(milestoneIndex, coo.cooPrivateKeys, len(coo.cooPrivateKeys))

	pubKeys := []iotago.MilestonePublicKey{}
	pubKeysSet := iotago.MilestonePublicKeySet{}
	for k := range keymapping {
		pubKeys = append(pubKeys, k)
		pubKeysSet[k] = struct{}{}
	}

	err = payload.Sign(pubKeys, iotago.InMemoryEd25519MilestoneSigner(keymapping))
	if err != nil {
		return nil, err
	}

	err = payload.VerifySignatures(len(coo.cooPrivateKeys), pubKeysSet)
	if err != nil {
		return nil, err
	}

	return payload, nil
}

// issueMilestoneOnTips creates a milestone on top of the given tips.
func (coo *MockCoo) issueMilestoneOnTips(tips hornet.MessageIDs, addLastMilestoneAsParent bool) (*storage.Milestone, error) {

	currentIndex := coo.LastMilestoneIndex
	coo.te.VerifyLMI(currentIndex)
	milestoneIndex := currentIndex + 1

	fmt.Printf("Issue milestone %v\n", milestoneIndex)

	if addLastMilestoneAsParent {
		tips = append(tips, coo.LastMilestoneMessageID)
	}

	milestonePayload, err := coo.milestonePayload(tips)
	if err != nil {
		return nil, err
	}

	msg, err := builder.NewMessageBuilder(coo.te.protoParas.Version).
		ParentsMessageIDs(tips.ToSliceOfArrays()).
		Payload(milestonePayload).
		ProofOfWork(context.Background(), coo.te.protoParas, coo.te.PoWMinScore).
		Build()
	if err != nil {
		return nil, err
	}

	milestoneMessageID := coo.storeMessage(msg)
	if err != nil {
		return nil, err
	}
	coo.LastMilestoneMessageID = milestoneMessageID

	coo.te.VerifyLMI(milestoneIndex)
	cachedMilestone := coo.te.storage.CachedMilestoneOrNil(milestoneIndex) // milestone +1
	require.NotNil(coo.te.TestInterface, cachedMilestone)

	coo.te.Milestones = append(coo.te.Milestones, cachedMilestone)

	milestoneId, err := milestonePayload.ID()
	require.NoError(coo.te.TestInterface, err)

	coo.LastMilestoneID = *milestoneId
	coo.LastMilestoneIndex = milestoneIndex
	coo.LastMilestoneTimestamp = milestonePayload.Timestamp

	return cachedMilestone.Milestone(), nil
}
