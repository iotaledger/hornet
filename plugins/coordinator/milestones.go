package coordinator

import (
	"fmt"
	"strings"
	"time"

	"github.com/iotaledger/hive.go/batchhasher"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/kerl"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/model/milestone"
)

// Bundle represents grouped together transactions for creating a transfer.
type Bundle = []*transaction.Transaction

// siblings calculates a list of siblings
func siblings(leafIndex milestone.Index, merkleTree *MerkleTree) []trinary.Hash {
	var siblings []trinary.Hash

	for currentLayerIndex := merkleTree.Depth; currentLayerIndex > 0; currentLayerIndex-- {
		layer := merkleTree.Layers[currentLayerIndex]

		if leafIndex%2 == 0 {
			// even
			siblings = append(siblings, layer.Hashes[leafIndex+1])
		} else {
			// odd
			siblings = append(siblings, layer.Hashes[leafIndex-1])
		}

		leafIndex /= 2
	}

	return siblings
}

func getTagForIndex(index milestone.Index) trinary.Trytes {
	return trinary.IntToTrytes(int64(index), 27)
}

func createMilestone(trunkHash trinary.Hash, branchHash trinary.Hash, index milestone.Index, merkleTree *MerkleTree) (Bundle, error) {

	// Get the siblings in the current merkle tree
	leafSiblings := siblings(index, merkleTree)

	siblingsTrytes := strings.Join(leafSiblings, "")
	paddedSiblingsTrytes := trinary.MustPad(siblingsTrytes, consts.KeyFragmentLength/consts.TrinaryRadix)

	tag := getTagForIndex(index)

	// A milestone consists of two transactions.
	// The last transaction (currentIndex == lastIndex) contains the siblings for the merkle tree.
	txSiblings := &transaction.Transaction{}
	txSiblings.SignatureMessageFragment = paddedSiblingsTrytes
	txSiblings.Address = consts.NullHashTrytes
	txSiblings.CurrentIndex = uint64(securityLvl)
	txSiblings.LastIndex = uint64(securityLvl)
	txSiblings.Timestamp = uint64(time.Now().Unix())
	txSiblings.ObsoleteTag = consts.NullTagTrytes
	txSiblings.Value = 0
	txSiblings.Bundle = consts.NullHashTrytes
	txSiblings.TrunkTransaction = trunkHash
	txSiblings.BranchTransaction = branchHash
	txSiblings.Tag = consts.NullTagTrytes
	txSiblings.Nonce = consts.NullTagTrytes

	// The other transactions contain a signature that signs the siblings and thereby ensures the integrity.
	var b Bundle

	for txIndex := 0; txIndex < securityLvl; txIndex++ {
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
		tx.BranchTransaction = trunkHash
		tx.Tag = tag
		tx.Nonce = consts.NullTagTrytes

		b = append(b, tx)
	}

	b = append(b, txSiblings)

	// finalize bundle by adding the bundle hash
	b, err := finalizeInsecure(b)
	if err != nil {
		return nil, fmt.Errorf("Bundle.Finalize: %v", err.Error())
	}

	if err = doPow(txSiblings, mwm); err != nil {
		return nil, fmt.Errorf("doPow: %v", err.Error())
	}

	signature, err := GetSignature(seed, index, securityLvl, txSiblings.Hash)
	if err != nil {
		return nil, err
	}

	if err = validateSignature(merkleTree.Root, index, securityLvl, txSiblings.Hash, signature, siblingsTrytes); err != nil {
		return nil, err
	}

	if err = chainTransactionsFillSignatures(b, signature, mwm); err != nil {
		return nil, err
	}

	return b, nil
}

func doPow(tx *transaction.Transaction, mwm int) error {

	tx.AttachmentTimestamp = time.Now().UnixNano() / int64(time.Millisecond)
	tx.AttachmentTimestampLowerBound = consts.LowerBoundAttachmentTimestamp
	tx.AttachmentTimestampUpperBound = consts.UpperBoundAttachmentTimestamp

	trytes, err := transaction.TransactionToTrytes(tx)
	if err != nil {
		return err
	}

	nonce, err := powFunc(trytes, mwm)
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

// We don't need to care about the M-Bug here => we use a fixed version of the ISS
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

func chainTransactionsFillSignatures(b Bundle, signature trinary.Trytes, mwm int) error {
	// to chain transactions we start from the LastIndex and move towards index 0.
	prev := b[len(b)-1].Hash

	// we have to skip the siblingsTx, because it is already complete
	for i := len(b) - 2; i >= 0; i-- {
		tx := b[i]

		// copy signature fragment
		tx.SignatureMessageFragment = signature[tx.CurrentIndex*consts.SignatureMessageFragmentSizeInTrytes : (tx.CurrentIndex+1)*consts.SignatureMessageFragmentSizeInTrytes]

		// chain bundle
		tx.TrunkTransaction = prev

		// perform PoW
		if err := doPow(tx, mwm); err != nil {
			return fmt.Errorf("doPow: %v", err.Error())
		}

		prev = tx.Hash
	}
	return nil
}
