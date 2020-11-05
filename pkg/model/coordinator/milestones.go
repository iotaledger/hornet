package coordinator

import (
	"time"

	iotago "github.com/iotaledger/iota.go"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/pow"
)

// createCheckpoint creates a checkpoint message.
func createCheckpoint(parent1MessageID *hornet.MessageID, parent2MessageID *hornet.MessageID, powHandler *pow.Handler) (*tangle.Message, error) {
	iotaMsg := &iotago.Message{Version: 1, Parent1: *parent1MessageID, Parent2: *parent2MessageID, Payload: nil}

	if err := powHandler.DoPoW(iotaMsg, nil, 1); err != nil {
		return nil, err
	}

	msg, err := tangle.NewMessage(iotaMsg, iotago.DeSeriModePerformValidation)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

// createMilestone creates a signed milestone message.
func createMilestone(index milestone.Index, parent1MessageID *hornet.MessageID, parent2MessageID *hornet.MessageID, signerProvider MilestoneSignerProvider, whiteFlagMerkleRootTreeHash [64]byte, powHandler *pow.Handler) (*tangle.Message, error) {

	milestoneIndexSigner := signerProvider.MilestoneIndexSigner(index)
	pubKeys := milestoneIndexSigner.PublicKeys()

	msPayload, err := iotago.NewMilestone(uint32(index), uint64(time.Now().Unix()), *parent1MessageID, *parent2MessageID, whiteFlagMerkleRootTreeHash, pubKeys)
	if err != nil {
		return nil, err
	}

	iotaMsg := &iotago.Message{Version: 1, Parent1: *parent1MessageID, Parent2: *parent2MessageID, Payload: msPayload}

	if err := msPayload.Sign(milestoneIndexSigner.SigningFunc()); err != nil {
		return nil, err
	}

	if err = msPayload.VerifySignatures(signerProvider.PublicKeysCount(), milestoneIndexSigner.PublicKeysSet()); err != nil {
		return nil, err
	}

	if err := powHandler.DoPoW(iotaMsg, nil, 1); err != nil {
		return nil, err
	}

	msg, err := tangle.NewMessage(iotaMsg, iotago.DeSeriModePerformValidation)
	if err != nil {
		return nil, err
	}

	return msg, nil
}
