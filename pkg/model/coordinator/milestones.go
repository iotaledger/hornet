package coordinator

import (
	"fmt"
	"strings"
	"time"

	"github.com/iotaledger/hive.go/batchhasher"
	"github.com/iotaledger/iota.go/bundle"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/kerl"
	"github.com/iotaledger/iota.go/merkle"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/t6b1"
	"github.com/gohornet/hornet/pkg/utils"
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
func createCheckpoint(trunkHash hornet.Hash, branchHash hornet.Hash, mwm int, powHandler *pow.Handler) (Bundle, error) {

	tag := randomTrytesWithRandomLengthPadded(5, consts.TagTrinarySize/3)

	tx := &transaction.Transaction{}
	tx.SignatureMessageFragment = randomTrytesWithRandomLengthPadded(100, consts.SignatureMessageFragmentTrinarySize/3)
	tx.Address = utils.RandomTrytesInsecure(consts.AddressTrinarySize / 3)
	tx.Value = 0
	tx.ObsoleteTag = tag
	tx.Timestamp = uint64(time.Now().Unix())
	tx.CurrentIndex = 0
	tx.LastIndex = 0
	tx.Bundle = consts.NullHashTrytes
	tx.TrunkTransaction = trunkHash.Trytes()
	tx.BranchTransaction = branchHash.Trytes()
	tx.Tag = tag
	tx.AttachmentTimestamp = 0
	tx.AttachmentTimestampLowerBound = consts.LowerBoundAttachmentTimestamp
	tx.AttachmentTimestampUpperBound = consts.UpperBoundAttachmentTimestamp
	tx.Nonce = consts.NullTagTrytes

	var err error
	b := Bundle{tx}

	// finalize bundle by adding the bundle hash
	b, err = finalizeInsecure(b)
	if err != nil {
		return nil, err
	}

	if err = doPow(tx, mwm, powHandler); err != nil {
		return nil, err
	}

	return b, err
}

// createMilestone creates a signed milestone bundle.
func createMilestone(seed trinary.Hash, index milestone.Index, securityLvl consts.SecurityLevel, trunkHash hornet.Hash, branchHash hornet.Hash, mwm int, merkleTree *merkle.MerkleTree, whiteFlagMerkleRootTreeHash []byte, powHandler *pow.Handler) (Bundle, error) {

	// get the siblings in the current Merkle tree
	leafSiblings, err := merkleTree.AuditPath(uint32(index))
	if err != nil {
		return nil, err
	}

	siblingsTrytes := strings.Join(leafSiblings, "")

	// append t6b1 encoded merkle tree root hash to the head's signature message fragment data
	siblingsTrytes += t6b1.MustBytesToTrytes(whiteFlagMerkleRootTreeHash)

	paddedSiblingsTrytes := trinary.MustPad(siblingsTrytes, consts.SignatureMessageFragmentSizeInTrytes)

	tag := tagForIndex(index)

	// a milestone consists of two transactions.
	// the last transaction (currentIndex == lastIndex) contains the siblings for the Merkle tree.
	txSiblings := &transaction.Transaction{}
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
	var b Bundle

	for txIndex := 0; txIndex < int(securityLvl); txIndex++ {
		tx := &transaction.Transaction{}
		tx.SignatureMessageFragment = consts.NullSignatureMessageFragmentTrytes
		tx.Address = merkleTree.Root
		tx.CurrentIndex = uint64(txIndex)
		tx.LastIndex = uint64(securityLvl)
		tx.Timestamp = uint64(time.Now().Unix())
		tx.ObsoleteTag = tag
		tx.Value = 0
		tx.Bundle = consts.NullHashTrytes
		tx.TrunkTransaction = consts.NullHashTrytes
		tx.BranchTransaction = trunkHash.Trytes()
		tx.Tag = tag
		tx.Nonce = consts.NullTagTrytes

		b = append(b, tx)
	}

	b = append(b, txSiblings)
	// Address + Value + ObsoleteTag + Timestamp + CurrentIndex + LastIndex
	// finalize bundle by adding the bundle hash
	b, err = finalizeInsecure(b)
	if err != nil {
		return nil, err
	}

	if err = doPow(txSiblings, mwm, powHandler); err != nil {
		return nil, err
	}

	fragments, err := merkle.SignatureFragments(seed, uint32(index), securityLvl, txSiblings.Hash)
	if err != nil {
		return nil, err
	}

	// verify milestone signature
	if valid, err := merkle.ValidateSignatureFragments(merkleTree.Root, uint32(index), leafSiblings, fragments, txSiblings.Hash); !valid {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("Merkle root does not match")
	}

	if err = chainTransactionsFillSignatures(b, fragments, mwm, powHandler); err != nil {
		return nil, err
	}

	// check all tx
	iotaGoBundle := make(bundle.Bundle, len(b))
	for i := 0; i < len(b); i++ {
		iotaGoBundle[i] = *b[i]
	}

	// validate bundle semantics and signatures
	if err := bundle.ValidBundle(iotaGoBundle); err != nil {
		fmt.Println(err.Error())
		return nil, err
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

	// set new transaction hash
	tx.Hash = transactionHash(tx)

	return nil
}

// transactionHash makes a transaction hash from the given transaction.
func transactionHash(t *transaction.Transaction) trinary.Hash {
	trits, _ := transaction.TransactionToTrits(t)
	hashTrits := batchhasher.CURLP81.Hash(trits)
	return trinary.MustTritsToTrytes(hashTrits)
}

// finalizeInsecure sets the bundle hash for all transactions in the bundle.
// we do not care about the M-Bug since we use a fixed version of the ISS.
func finalizeInsecure(bundle Bundle) (Bundle, error) {

	k := kerl.NewKerl()

	for _, tx := range bundle {
		txTrits, err := transaction.TransactionToTrits(tx)
		if err != nil {
			return nil, err
		}

		k.Absorb(txTrits[consts.AddressTrinaryOffset:consts.BundleTrinaryOffset]) // Address + Value + ObsoleteTag + Timestamp + CurrentIndex + LastIndex
	}

	bundleHashTrits, err := k.Squeeze(consts.HashTrinarySize)
	if err != nil {
		return nil, err
	}
	bundleHash := trinary.MustTritsToTrytes(bundleHashTrits)

	// set the computed bundle hash on each tx in the bundle
	for _, tx := range bundle {
		tx.Bundle = bundleHash
	}

	return bundle, nil
}

// chainTransactionsFillSignatures fills the signature message fragments with the signature and sets the trunk to chain the txs in a bundle.
func chainTransactionsFillSignatures(b Bundle, fragments []trinary.Trytes, mwm int, powHandler *pow.Handler) error {
	// to chain transactions we start from the LastIndex and move towards index 0.
	prev := b[len(b)-1].Hash

	// we have to skip the siblingsTx, because it is already complete
	for i := len(b) - 2; i >= 0; i-- {
		tx := b[i]

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
