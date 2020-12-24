package coordinator

import (
	"time"

	iotago "github.com/iotaledger/iota.go"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/pow"
)

// createCheckpoint creates a checkpoint message.
func createCheckpoint(networkID uint64, parent1MessageID *hornet.MessageID, parent2MessageID *hornet.MessageID, powParallelism int, powHandler *pow.Handler) (*storage.Message, error) {
	iotaMsg := &iotago.Message{NetworkID: networkID, Parent1: *parent1MessageID, Parent2: *parent2MessageID, Payload: nil}

	if err := powHandler.DoPoW(iotaMsg, nil, powParallelism); err != nil {
		return nil, err
	}

	msg, err := storage.NewMessage(iotaMsg, iotago.DeSeriModePerformValidation)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

// createMilestone creates a signed milestone message.
func createMilestone(index milestone.Index, networkID uint64, parent1MessageID *hornet.MessageID, parent2MessageID *hornet.MessageID, signerProvider MilestoneSignerProvider, whiteFlagMerkleRootTreeHash [iotago.MilestoneInclusionMerkleProofLength]byte, powParallelism int, powHandler *pow.Handler) (*storage.Message, error) {

	milestoneIndexSigner := signerProvider.MilestoneIndexSigner(index)
	pubKeys := milestoneIndexSigner.PublicKeys()

	msPayload, err := iotago.NewMilestone(uint32(index), uint64(time.Now().Unix()), *parent1MessageID, *parent2MessageID, whiteFlagMerkleRootTreeHash, pubKeys)
	if err != nil {
		return nil, err
	}

	iotaMsg := &iotago.Message{NetworkID: networkID, Parent1: *parent1MessageID, Parent2: *parent2MessageID, Payload: msPayload}

	if err := msPayload.Sign(milestoneIndexSigner.SigningFunc()); err != nil {
		return nil, err
	}

	if err = msPayload.VerifySignatures(signerProvider.PublicKeysCount(), milestoneIndexSigner.PublicKeysSet()); err != nil {
		return nil, err
	}

	if err := powHandler.DoPoW(iotaMsg, nil, powParallelism); err != nil {
		return nil, err
	}

	msg, err := storage.NewMessage(iotaMsg, iotago.DeSeriModePerformValidation)
	if err != nil {
		return nil, err
	}

	return msg, nil
}
