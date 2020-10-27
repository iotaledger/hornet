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

	err := powHandler.DoPoW(iotaMsg, nil, 1)
	if err != nil {
		return nil, err
	}

	msg, err := tangle.NewMessage(iotaMsg, iotago.DeSeriModePerformValidation)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

// createMilestone creates a signed milestone message.
func createMilestone(index milestone.Index, parent1MessageID *hornet.MessageID, parent2MessageID *hornet.MessageID, pubKeys []iotago.MilestonePublicKey, milestoneSignFunc iotago.MilestoneSigningFunc, whiteFlagMerkleRootTreeHash [64]byte, powHandler *pow.Handler) (*tangle.Message, error) {

	// ToDo: sort
	//sort.Sort(iotago.SortedSerializables(pubKeys))

	msPayload := &iotago.Milestone{
		Index:                uint32(index),
		Timestamp:            uint64(time.Now().Unix()),
		Parent1:              *parent1MessageID,
		Parent2:              *parent2MessageID,
		InclusionMerkleProof: whiteFlagMerkleRootTreeHash,
		PublicKeys:           pubKeys,
	}

	iotaMsg := &iotago.Message{Version: 1, Parent1: *parent1MessageID, Parent2: *parent2MessageID, Payload: msPayload}

	err := msPayload.Sign(milestoneSignFunc)
	if err != nil {
		return nil, err
	}

	err = powHandler.DoPoW(iotaMsg, nil, 1)
	if err != nil {
		return nil, err
	}

	msg, err := tangle.NewMessage(iotaMsg, iotago.DeSeriModePerformValidation)
	if err != nil {
		return nil, err
	}

	return msg, nil
}
