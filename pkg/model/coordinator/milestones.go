package coordinator

import (
	"fmt"
	"strings"
	"time"

	"github.com/iotaledger/iota.go/bundle"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/encoding/b1t6"
	"github.com/iotaledger/iota.go/merkle"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/gohornet/hornet/plugins/curl"
)

// tagForIndex creates a tag for a specific index.
func tagForIndex(index milestone.Index) trinary.Trytes {
	return trinary.IntToTrytes(int64(index), 27)
}

// randomTrytesWithRandomLengthPadded creates Trytes with random length in the range from min to length and pads it with 9's
func randomTrytesWithRandomLengthPadded(min int, length int) trinary.Trytes {
	return trinary.MustPad(utils.RandomTrytesInsecure(utils.RandomInsecure(0, length)), length)
}

// createCheckpoint creates a checkpoint transaction.
func createCheckpoint(trunkHash hornet.Hash, branchHash hornet.Hash, mwm int, powHandler *pow.Handler) (bundle.Bundle, error) {

	tag := randomTrytesWithRandomLengthPadded(5, consts.TagTrinarySize/3)

	b := bundle.Bundle{
		transaction.Transaction{
			SignatureMessageFragment:      randomTrytesWithRandomLengthPadded(100, consts.SignatureMessageFragmentTrinarySize/3),
			Address:                       utils.RandomTrytesInsecure(consts.AddressTrinarySize / 3),
			Value:                         0,
			ObsoleteTag:                   tag,
			Timestamp:                     uint64(time.Now().Unix()),
			CurrentIndex:                  0,
			LastIndex:                     0,
			Bundle:                        consts.NullHashTrytes,
			TrunkTransaction:              trunkHash.Trytes(),
			BranchTransaction:             branchHash.Trytes(),
			Tag:                           tag,
			AttachmentTimestamp:           0,
			AttachmentTimestampLowerBound: consts.LowerBoundAttachmentTimestamp,
			AttachmentTimestampUpperBound: consts.UpperBoundAttachmentTimestamp,
			Nonce:                         consts.NullTagTrytes,
		},
	}

	// finalize bundle by adding the bundle hash
	b, err := bundle.FinalizeInsecure(b)
	if err != nil {
		return nil, fmt.Errorf("failed to finalize: %w", err)
	}

	if err = doPow(&b[0], mwm, powHandler); err != nil {
		return nil, fmt.Errorf("failed to do PoW: %w", err)
	}
	return b, nil
}

// createMilestone creates a signed milestone bundle.
func createMilestone(seed trinary.Hash, index milestone.Index, securityLvl consts.SecurityLevel, trunkHash hornet.Hash, branchHash hornet.Hash, mwm int, merkleTree *merkle.MerkleTree, whiteFlagMerkleRootTreeHash []byte, powHandler *pow.Handler) (bundle.Bundle, error) {

	// get the siblings in the current Merkle tree
	leafSiblings, err := merkleTree.AuditPath(uint32(index))
	if err != nil {
		return nil, fmt.Errorf("failed to compute Merkle audit path: %w", err)
	}

	siblingsTrytes := strings.Join(leafSiblings, "")

	// append the b1t6 encoded Merkle tree hash to the head transaction's signature message fragment
	siblingsTrytes += b1t6.EncodeToTrytes(whiteFlagMerkleRootTreeHash)
	if len(siblingsTrytes) > consts.SignatureMessageFragmentSizeInTrytes {
		return nil, ErrInvalidSiblingsTrytesLength
	}
	paddedSiblingsTrytes := trinary.MustPad(siblingsTrytes, consts.SignatureMessageFragmentSizeInTrytes)

	tag := tagForIndex(index)

	// a milestone bundle consists of securityLvl transactions for the signatures and one for the audit path
	b := make(bundle.Bundle, securityLvl+1)

	// the last transaction (currentIndex == lastIndex) contains the siblings for the Merkle tree.
	txSiblings := &b[securityLvl]
	txSiblings.SignatureMessageFragment = paddedSiblingsTrytes
	txSiblings.Address = merkleTree.Root
	txSiblings.CurrentIndex = uint64(securityLvl)
	txSiblings.LastIndex = uint64(securityLvl)
	txSiblings.Timestamp = uint64(time.Now().Unix())
	txSiblings.ObsoleteTag = tag
	txSiblings.Value = 0
	txSiblings.Bundle = consts.NullHashTrytes
	txSiblings.TrunkTransaction = trunkHash.Trytes()
	txSiblings.BranchTransaction = branchHash.Trytes()
	txSiblings.Tag = tag
	txSiblings.Nonce = consts.NullTagTrytes

	// the other transactions contain a signature that signs the siblings and thereby ensures the integrity.
	for i := 0; i < int(securityLvl); i++ {
		tx := &b[i]

		tx.SignatureMessageFragment = consts.NullSignatureMessageFragmentTrytes
		tx.Address = merkleTree.Root
		tx.CurrentIndex = uint64(i)
		tx.LastIndex = uint64(securityLvl)
		tx.Timestamp = uint64(time.Now().Unix())
		tx.ObsoleteTag = tag
		tx.Value = 0
		tx.Bundle = consts.NullHashTrytes
		tx.TrunkTransaction = consts.NullHashTrytes
		tx.BranchTransaction = trunkHash.Trytes()
		tx.Tag = tag
		tx.Nonce = consts.NullTagTrytes
	}

	// finalize bundle by adding the bundle hash
	b, err = bundle.FinalizeInsecure(b)
	if err != nil {
		return nil, fmt.Errorf("failed to finalize: %w", err)
	}

	if err = doPow(txSiblings, mwm, powHandler); err != nil {
		return nil, fmt.Errorf("failed to do PoW: %w", err)
	}

	fragments, err := merkle.SignatureFragments(seed, uint32(index), securityLvl, txSiblings.Hash)
	if err != nil {
		return nil, fmt.Errorf("failed to sign: %w", err)
	}

	// verify milestone signature
	if valid, err := merkle.ValidateSignatureFragments(merkleTree.Root, uint32(index), leafSiblings, fragments, txSiblings.Hash); !valid {
		return nil, fmt.Errorf("signature validation failed: %w", err)
	}

	if err = chainTransactionsFillSignatures(b, fragments, mwm, powHandler); err != nil {
		return nil, fmt.Errorf("failed to add signatures: %w", err)
	}

	// validate bundle semantics and signatures
	if err := bundle.ValidBundle(b); err != nil {
		return nil, fmt.Errorf("bundle validation failed: %w", err)
	}

	return b, nil
}

// doPow calculates the transaction nonce and the hash.
func doPow(tx *transaction.Transaction, mwm int, powHandler *pow.Handler) error {

	tx.AttachmentTimestamp = time.Now().UnixNano() / int64(time.Millisecond)
	tx.AttachmentTimestampLowerBound = consts.LowerBoundAttachmentTimestamp
	tx.AttachmentTimestampUpperBound = consts.UpperBoundAttachmentTimestamp

	trytes, err := transaction.TransactionToTrytes(tx)
	if err != nil {
		return err
	}

	nonce, err := powHandler.DoPoW(trytes, mwm)
	if err != nil {
		return err
	}

	tx.Nonce = nonce

	hash, err := transactionHash(tx)
	if err != nil {
		return err
	}

	tx.Hash = hash

	return nil
}

// transactionHash makes a transaction hash from the given transaction.
func transactionHash(t *transaction.Transaction) (trinary.Hash, error) {
	trits, err := transaction.TransactionToTrits(t)
	if err != nil {
		return "", err
	}
	hashTrits, err := curl.Hasher().Hash(trits)
	if err != nil {
		return "", err
	}
	return trinary.MustTritsToTrytes(hashTrits), nil
}

// chainTransactionsFillSignatures fills the signature message fragments with the signature and sets the trunk to chain the txs in a bundle.
func chainTransactionsFillSignatures(b bundle.Bundle, fragments []trinary.Trytes, mwm int, powHandler *pow.Handler) error {
	// to chain transactions we start from the LastIndex and move towards index 0.
	prev := b[len(b)-1].Hash

	// we have to skip the siblingsTx, because it is already complete
	for i := len(b) - 2; i >= 0; i-- {
		tx := &b[i]

		// copy signature fragment
		tx.SignatureMessageFragment = fragments[tx.CurrentIndex]

		// chain bundle
		tx.TrunkTransaction = prev

		// perform PoW
		if err := doPow(tx, mwm, powHandler); err != nil {
			return err
		}

		prev = tx.Hash
	}
	return nil
}
