package whiteflag

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gohornet/hornet/integration-tests/wfmock/pkg/config"
	httpapi "github.com/gohornet/hornet/integration-tests/wfmock/pkg/http"
	"github.com/iotaledger/iota.go/address"
	legacyapi "github.com/iotaledger/iota.go/api"
	"github.com/iotaledger/iota.go/bundle"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/encoding/b1t6"
	"github.com/iotaledger/iota.go/merkle"
	"github.com/iotaledger/iota.go/pow"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"
)

var (
	powFunc pow.ProofOfWorkFunc
	data    *whiteFlagData
)

type whiteFlagData struct {
	latestMilestoneHash  trinary.Hash
	latestMilestoneIndex uint32
	coordinatorAddress   trinary.Hash

	milestones []whiteFlagMilestone
}

type whiteFlagMilestone struct {
	milestoneBundle          []trinary.Trytes
	includedMigrationBundles [][]trinary.Trytes
}

func init() {
	cfg := config.GetConfig()

	powFunc = getPOWFunc()

	log.Println("creating migration bundles...")
	includedBundles, err := createIncludedBundles(cfg.WhiteFlag, cfg.Coordinator.MWM)
	if err != nil {
		log.Fatalf("failed to create bundles: %s", err)
	}
	log.Printf("created bundles for %d milestone indices\n", len(includedBundles))

	log.Println("creating milestones...")
	data, err = createMilestones(cfg.Coordinator, includedBundles)
	if err != nil {
		log.Fatalf("failed to create milestones: %s", err)
	}
	log.Printf("mocked coordinator: {address=%s, depth=%d, MWM=%d, latestMSIndex=%d}\n",
		data.coordinatorAddress, cfg.Coordinator.TreeDepth, cfg.Coordinator.MWM, data.latestMilestoneIndex)

	// register the API commands
	httpapi.RegisterHandler(strings.ToLower(GetNodeInfoCommand), getNodeInfo)
	httpapi.RegisterHandler(strings.ToLower(GetWhiteFlagConfirmationCommand), getWhiteFlagConfirmation)

	log.Println("white flag API initialized")
}

func getPOWFunc() pow.ProofOfWorkFunc {
	name, powFunc := pow.GetFastestProofOfWorkImpl()
	log.Printf("using '%s' PoW", name)
	return powFunc
}

func createIncludedBundles(cfg config.WhiteFlagConfig, mwm int) (map[uint32][][]trinary.Trytes, error) {
	iotaAPI := new(legacyapi.API)

	includedBundles := make(map[uint32][][]trinary.Trytes)
	for msIndex, migrations := range cfg.Migrations {
		var bundles [][]trinary.Trytes
		for _, migration := range migrations {
			addr, err := address.GenerateAddress(cfg.Seed, migration.Index, migration.Security, true)
			if err != nil {
				return nil, fmt.Errorf("failed to generate address: %w", err)
			}
			inputs := []legacyapi.Input{{
				Balance:  migration.Balance,
				Address:  addr,
				KeyIndex: migration.Index,
				Security: migration.Security,
			}}
			migrationAddress, err := generateMigrationAddress(migration.Ed25519Address)
			if err != nil {
				return nil, fmt.Errorf("failed to generate migration address from config: %w", err)
			}
			transfers := []bundle.Transfer{{
				Address: migrationAddress,
				Value:   migration.Balance,
			}}

			rawTrytes, err := iotaAPI.PrepareTransfers(cfg.Seed, transfers, legacyapi.PrepareTransfersOptions{
				Inputs: inputs,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create bundle: %w", err)
			}
			// PrepareTransfers returns the transaction trytes in the reversed order, so we must convert and reverse
			bndl, _ := transaction.AsTransactionObjects(rawTrytes, nil)
			for i, j := 0, len(bndl)-1; i < j; i, j = i+1, j-1 {
				bndl[i], bndl[j] = bndl[j], bndl[i]
			}
			if err := finalizeBundle(bndl, mwm); err != nil {
				return nil, fmt.Errorf("failed to finalize the bundle: %w", err)
			}
			bundles = append(bundles, transaction.MustTransactionsToTrytes(bndl))
		}
		includedBundles[msIndex] = bundles
	}
	return includedBundles, nil
}

func generateMigrationAddress(bytes []byte) (trinary.Hash, error) {
	if len(bytes) != 32 {
		return "", consts.ErrInvalidAddress
	}
	var addr [32]byte
	copy(addr[:], bytes)
	return address.GenerateMigrationAddress(addr, true)
}

func createMilestones(cfg config.CoordinatorConfig, includedBundles map[uint32][][]trinary.Trytes) (*whiteFlagData, error) {
	var latestMSIndex uint32
	for msIndex := range includedBundles {
		if msIndex > latestMSIndex {
			latestMSIndex = msIndex
		}
	}

	merkleTree, err := merkle.CreateMerkleTree(cfg.Seed, cfg.Security, cfg.TreeDepth)
	if err != nil {
		return nil, fmt.Errorf("failed to compute coordinator Merkle tree: %w", err)
	}

	var latestMSHash trinary.Hash
	confirmations := make([]whiteFlagMilestone, latestMSIndex+1)
	for index := uint32(1); index <= latestMSIndex; index++ {
		var msBundle []trinary.Trytes
		latestMSHash, msBundle, err = createMilestone(cfg, merkleTree, index, includedBundles[index])
		if err != nil {
			return nil, fmt.Errorf("failed to created milestone: %w", err)
		}

		confirmations[index] = whiteFlagMilestone{
			milestoneBundle:          msBundle,
			includedMigrationBundles: includedBundles[index],
		}
	}

	context := &whiteFlagData{
		latestMilestoneHash:  latestMSHash,
		latestMilestoneIndex: latestMSIndex,
		coordinatorAddress:   merkleTree.Root,
		milestones:           confirmations,
	}
	return context, nil
}

func createMilestone(cfg config.CoordinatorConfig, merkleTree *merkle.MerkleTree, index uint32, includedBundles [][]trinary.Trytes) (trinary.Hash, []trinary.Trytes, error) {
	leafSiblings, err := merkleTree.AuditPath(index)
	if err != nil {
		return "", nil, fmt.Errorf("failed to compute Merkle audit path: %w", err)
	}
	siblingsTrytes := strings.Join(leafSiblings, "")

	// append the b1t6 encoded Merkle tree hash to the signature message fragment
	whiteFlagHash, err := computeWhiteFlagMerkleTreeHash(includedBundles)
	if err != nil {
		return "", nil, fmt.Errorf("failed to compute white flag Merkle tree hash: %w", err)
	}
	siblingsTrytes += b1t6.EncodeToTrytes(whiteFlagHash)

	tag := trinary.IntToTrytes(int64(index), consts.TagTrinarySize/consts.TritsPerTryte)

	bndl := make(bundle.Bundle, cfg.Security+1)
	for i := range bndl {
		tx := &bndl[i]

		tx.SignatureMessageFragment = consts.NullSignatureMessageFragmentTrytes
		tx.Address = merkleTree.Root
		tx.Value = 0
		tx.ObsoleteTag = tag
		tx.Timestamp = uint64(time.Now().Unix())
		tx.CurrentIndex = uint64(i)
		tx.LastIndex = uint64(cfg.Security)
		tx.TrunkTransaction = consts.NullHashTrytes
		tx.BranchTransaction = consts.NullHashTrytes
		tx.Tag = tag
		tx.Nonce = consts.NullTagTrytes
	}

	txSiblings := &bndl[cfg.Security]
	txSiblings.SignatureMessageFragment = trinary.MustPad(siblingsTrytes, consts.SignatureMessageFragmentSizeInTrytes)

	// finalize bundle by adding the bundle hash
	bndl, err = bundle.FinalizeInsecure(bndl)
	if err != nil {
		return "", nil, fmt.Errorf("failed to finalize the bundle: %w", err)
	}

	// do PoW for the sibling transaction so that we can compute its final hash
	if err := doPow(txSiblings, cfg.MWM); err != nil {
		return "", nil, fmt.Errorf("failed to do PoW: %w", err)
	}

	fragments, err := merkle.SignatureFragments(cfg.Seed, index, cfg.Security, txSiblings.Hash)
	if err != nil {
		return "", nil, fmt.Errorf("signing failed: %w", err)
	}
	bundle.AddTrytes(bndl, fragments, 0)

	// do PoW for the remaining transactions
	for i := len(bndl) - 2; i >= 0; i-- {
		bndl[i].TrunkTransaction = bndl[i+1].Hash
		if err = doPow(&bndl[i], cfg.MWM); err != nil {
			return "", nil, fmt.Errorf("failed to do PoW for transaction %d: %w", bndl[i].CurrentIndex, err)
		}
	}

	return bundle.TailTransactionHash(bndl), transaction.MustTransactionsToTrytes(bndl), nil
}

func computeWhiteFlagMerkleTreeHash(includedBundles [][]trinary.Trytes) ([]byte, error) {
	includedHashes := make([]trinary.Trytes, len(includedBundles))
	for i := range includedBundles {
		bndl, err := transaction.AsTransactionObjects(includedBundles[i], nil)
		if err != nil {
			return nil, fmt.Errorf("failed to parse bundle: %w", err)
		}
		includedHashes[i] = bundle.TailTransactionHash(bndl)
	}
	return DefaultHasher.Hash(includedHashes), nil
}

// doPow calculates the transaction nonce and the hash.
func doPow(tx *transaction.Transaction, mwm int) error {
	tx.AttachmentTimestamp = time.Now().UnixNano() / int64(time.Millisecond)
	tx.AttachmentTimestampLowerBound = consts.LowerBoundAttachmentTimestamp
	tx.AttachmentTimestampUpperBound = consts.UpperBoundAttachmentTimestamp

	nonce, err := powFunc(transaction.MustTransactionToTrytes(tx), mwm)
	if err != nil {
		return err
	}
	tx.Nonce = nonce
	tx.Hash = transaction.TransactionHash(tx)

	return nil
}

func finalizeBundle(bndl bundle.Bundle, mwm int) error {
	trunk := consts.NullHashTrytes
	for i := len(bndl) - 1; i >= 0; i-- {
		bndl[i].TrunkTransaction = trunk
		if err := doPow(&bndl[i], mwm); err != nil {
			return fmt.Errorf("failed to do PoW for tx %d: %w", bndl[i].CurrentIndex, err)
		}
		trunk = bndl[i].Hash
	}
	return nil
}
