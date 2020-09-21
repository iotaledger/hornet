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
func createCheckpoint(parent1MessageID hornet.Hash, parent2MessageID hornet.Hash, mwm int, powHandler *pow.Handler) (*tangle.Message, error) {

	iotaMsg := &iotago.Message{Version: 1, Parent1: parent1MessageID.ID(), Parent2: parent2MessageID.ID(), Payload: nil}

	msg, err := tangle.NewMessage(iotaMsg)
	if err != nil {
		return nil, err
	}

	if err = doPow(msg, mwm, powHandler); err != nil {
		return nil, err
	}

	return msg, nil
}

// createMilestone creates a signed milestone message.
func createMilestone(privateKey ed25519.PrivateKey, index milestone.Index, parent1MessageID hornet.Hash, parent2MessageID hornet.Hash, mwm int, whiteFlagMerkleRootTreeHash [64]byte, powHandler *pow.Handler) (*tangle.Message, error) {

	msPayload := &iotago.MilestonePayload{Index: uint64(index), Timestamp: uint64(time.Now().Unix()), InclusionMerkleProof: whiteFlagMerkleRootTreeHash}
	iotaMsg := &iotago.Message{Version: 1, Parent1: parent1MessageID.ID(), Parent2: parent2MessageID.ID(), Payload: msPayload}

	err := msPayload.Sign(iotaMsg, privateKey)
	if err != nil {
		return nil, err
	}

	pubKey := privateKey.Public().(ed25519.PublicKey)
	err = msPayload.VerifySignature(iotaMsg, pubKey)
	if err != nil {
		return nil, err
	}

	msg, err := tangle.NewMessage(iotaMsg)
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
