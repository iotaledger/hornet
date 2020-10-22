package coordinator

import (
	"crypto/ed25519"
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
		return nil, fmt.Errorf("failed to finalize: %w", err)
	}

	msg, err := tangle.NewMessage(iotaMsg, iotago.DeSeriModePerformValidation)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

// createMilestone creates a signed milestone message.
func createMilestone(privateKey ed25519.PrivateKey, index milestone.Index, parent1MessageID *hornet.MessageID, parent2MessageID *hornet.MessageID, mwm int, whiteFlagMerkleRootTreeHash [64]byte, powHandler *pow.Handler) (*tangle.Message, error) {

	msPayload := &iotago.Milestone{Index: uint64(index), Timestamp: uint64(time.Now().Unix()), InclusionMerkleProof: whiteFlagMerkleRootTreeHash}
	iotaMsg := &iotago.Message{Version: 1, Parent1: *parent1MessageID, Parent2: *parent2MessageID, Payload: msPayload}

	err := msPayload.Sign(iotaMsg, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to finalize: %w", err)
	}

	pubKey := privateKey.Public().(ed25519.PublicKey)
	err = msPayload.VerifySignature(iotaMsg, pubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign: %w", err)
	}

	msg, err := tangle.NewMessage(iotaMsg, iotago.DeSeriModePerformValidation)
	if err != nil {
		return nil, err
	}

	if err = doPow(msg, mwm, powHandler); err != nil {
		return nil, err
	}

	return msg, nil
}

// doPow calculates the message nonce and the hash.
func doPow(msg *tangle.Message, mwm int, powHandler *pow.Handler) error {

	msg.GetMessage().Nonce = 0

	/*
		ToDo:
		nonce, err := powHandler.DoPoW(trytes, mwm)
		if err != nil {
			return err
		}
	*/

	return nil
}
