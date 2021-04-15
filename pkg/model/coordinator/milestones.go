package coordinator

import (
	"time"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/pow"
	iotago "github.com/iotaledger/iota.go/v2"
)

// createCheckpoint creates a checkpoint message.
func createCheckpoint(networkID uint64, parents hornet.MessageIDs, powWorkerCount int, powHandler *pow.Handler) (*storage.Message, error) {
	iotaMsg := &iotago.Message{
		NetworkID: networkID,
		Parents:   parents.ToSliceOfArrays(),
		Payload:   nil,
	}

	if err := powHandler.DoPoW(iotaMsg, nil, powWorkerCount); err != nil {
		return nil, err
	}

	msg, err := storage.NewMessage(iotaMsg, iotago.DeSeriModePerformValidation)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

// createMilestone creates a signed milestone message.
func createMilestone(index milestone.Index, networkID uint64, parents hornet.MessageIDs, signerProvider MilestoneSignerProvider, receipt *iotago.Receipt, whiteFlagMerkleRootTreeHash [iotago.MilestoneInclusionMerkleProofLength]byte, powWorkerCount int, powHandler *pow.Handler) (*storage.Message, error) {
	milestoneIndexSigner := signerProvider.MilestoneIndexSigner(index)
	pubKeys := milestoneIndexSigner.PublicKeys()

	parentsSliceOfArray := parents.ToSliceOfArrays()
	msPayload, err := iotago.NewMilestone(uint32(index), uint64(time.Now().Unix()), parentsSliceOfArray, whiteFlagMerkleRootTreeHash, pubKeys)
	if err != nil {
		return nil, err
	}
	if receipt != nil {
		msPayload.Receipt = receipt
	}

	iotaMsg := &iotago.Message{
		NetworkID: networkID,
		Parents:   parentsSliceOfArray,
		Payload:   msPayload,
	}

	if err := msPayload.Sign(milestoneIndexSigner.SigningFunc()); err != nil {
		return nil, err
	}

	if err = msPayload.VerifySignatures(signerProvider.PublicKeysCount(), milestoneIndexSigner.PublicKeysSet()); err != nil {
		return nil, err
	}

	if err := powHandler.DoPoW(iotaMsg, nil, powWorkerCount); err != nil {
		return nil, err
	}

	msg, err := storage.NewMessage(iotaMsg, iotago.DeSeriModePerformValidation)
	if err != nil {
		return nil, err
	}

	return msg, nil
}
