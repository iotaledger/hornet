package coordinator

import (
	"strings"
	"time"

	"github.com/iotaledger/hive.go/batchhasher"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/kerl"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/plugins/gossip"
	"github.com/gohornet/hornet/plugins/tipselection"
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

// getTagForIndex creates a tag for a specific index
func getTagForIndex(index milestone.Index) trinary.Trytes {
	return trinary.IntToTrytes(int64(index), 27)
}

// createMilestone create a signed milestone bundle
func createMilestone(trunkHash trinary.Hash, branchHash trinary.Hash, index milestone.Index, merkleTree *MerkleTree) (Bundle, error) {

	// get the siblings in the current merkle tree
	leafSiblings := siblings(index, merkleTree)

	siblingsTrytes := strings.Join(leafSiblings, "")
	paddedSiblingsTrytes := trinary.MustPad(siblingsTrytes, consts.KeyFragmentLength/consts.TrinaryRadix)

	tag := getTagForIndex(index)

	// a milestone consists of two transactions.
	// the last transaction (currentIndex == lastIndex) contains the siblings for the merkle tree.
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

	// the other transactions contain a signature that signs the siblings and thereby ensures the integrity.
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
		return nil, err
	}

	if err = doPow(txSiblings, mwm); err != nil {
		return nil, err
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

// doPow calculates the transaction nonce and the hash
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

// finalizeInsecure sets the bundle hash for all transactions in the bundle
// we do not care about the M-Bug since we use a fixed version of the ISS
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

// chainTransactionsFillSignatures fills the signature message fragments with the signature and sets the trunk to chain the txs in a bundle
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
			return err
		}

		prev = tx.Hash
	}
	return nil
}

// issueCheckpoint sends a secret checkpoint transaction to the network
// we do that to prevent parasite chain attacks
// only honest tipselection will reference our checkpoints, so the milestone will reference honest tips
func issueCheckpoint(lastCheckpointHash *trinary.Hash) (trinary.Hash, error) {

	tips, _, err := tipselection.SelectTips(0, lastCheckpointHash)
	if err != nil {
		return "", err
	}

	tx := &transaction.Transaction{}
	tx.SignatureMessageFragment = consts.NullSignatureMessageFragmentTrytes
	tx.Address = consts.NullHashTrytes
	tx.Value = 0
	tx.ObsoleteTag = consts.NullTagTrytes
	tx.Timestamp = uint64(time.Now().Unix())
	tx.CurrentIndex = 0
	tx.LastIndex = 0
	tx.Bundle = consts.NullHashTrytes
	tx.TrunkTransaction = tips[0]
	tx.BranchTransaction = tips[1]
	tx.Tag = consts.NullTagTrytes
	tx.AttachmentTimestamp = 0
	tx.AttachmentTimestampLowerBound = consts.LowerBoundAttachmentTimestamp
	tx.AttachmentTimestampUpperBound = consts.UpperBoundAttachmentTimestamp
	tx.Nonce = consts.NullTagTrytes

	b := Bundle{tx}

	// finalize bundle by adding the bundle hash
	b, err = finalizeInsecure(b)
	if err != nil {
		return "", err
	}

	if err = doPow(tx, mwm); err != nil {
		return "", err
	}

	for _, tx := range b {
		txTrits, _ := transaction.TransactionToTrits(tx)
		if err := gossip.Processor().CompressAndEmit(tx, txTrits); err != nil {
			return "", err
		}
	}

	return tx.Hash, nil
}

// createAndSendMilestone create a milestone, sends it to the network and stores a new coordinator state file
func createAndSendMilestone(trunkHash trinary.Hash, branchHash trinary.Hash, newMilestoneIndex milestone.Index, merkleTree *MerkleTree) (trinary.Hash, error) {

	b, err := createMilestone(trunkHash, branchHash, newMilestoneIndex, coordinatorMerkleTree)
	if err != nil {
		return "", err
	}

	txHashes := []trinary.Hash{}
	for _, tx := range b {
		txTrits, err := transaction.TransactionToTrits(tx)
		if err != nil {
			log.Panic(err)
		}

		if err := gossip.Processor().CompressAndEmit(tx, txTrits); err != nil {
			log.Panic(err)
		}
		txHashes = append(txHashes, tx.Hash)
	}

	tailTx := b[0]
	coordinatorState.latestMilestoneHash = tailTx.Hash
	coordinatorState.latestMilestoneIndex = newMilestoneIndex
	coordinatorState.latestMilestoneTime = int64(tailTx.Timestamp)
	coordinatorState.latestMilestoneTransactions = txHashes

	if err := coordinatorState.storeStateFile(stateFilePath); err != nil {
		log.Panic(err)
	}

	log.Infof("Milestone created (%d): %v", coordinatorState.latestMilestoneIndex, coordinatorState.latestMilestoneHash)
	return tailTx.Hash, nil
}

// issueNextCheckpointOrMilestone creates the next checkpoint or milestone
// if the network was not bootstrapped yet, it creates the first milestone
func issueNextCheckpointOrMilestone() {

	milestoneLock.Lock()
	defer milestoneLock.Unlock()

	if !bootstrapped {
		// create first milestone to bootstrap the network
		msHash, err := createAndSendMilestone(consts.NullHashTrytes, consts.NullHashTrytes, coordinatorState.latestMilestoneIndex, coordinatorMerkleTree)
		if err != nil {
			log.Warn(err)
			return
		}
		lastCheckpointHash = &msHash
		bootstrapped = true
		return
	}

	if lastCheckpointCount < checkpointTransactions {
		// issue a checkpoint
		checkpointHash, err := issueCheckpoint(lastCheckpointHash)
		if err != nil {
			log.Warn(err)
			return
		}

		lastCheckpointCount++
		log.Infof("Issued checkpoint (%d): %v", lastCheckpointCount, checkpointHash)
		lastCheckpointHash = &checkpointHash
		return
	}

	// issue new milestone
	tips, _, err := tipselection.SelectTips(0, lastCheckpointHash)
	if err != nil {
		log.Warn(err)
		return
	}

	msHash, err := createAndSendMilestone(tips[0], tips[1], coordinatorState.latestMilestoneIndex+1, coordinatorMerkleTree)
	if err != nil {
		log.Warn(err)
		return
	}

	// always reference the last milestone directly to speed up syncing (or indirectly via checkpoints)
	lastCheckpointHash = &msHash
	lastCheckpointCount = 0
}
