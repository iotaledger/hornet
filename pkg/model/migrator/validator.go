package migrator

import (
	"bytes"
	"crypto"
	"encoding"
	"fmt"

	"github.com/gohornet/hornet/pkg/whiteflag"
	"github.com/iotaledger/iota.go/address"
	"github.com/iotaledger/iota.go/api"
	"github.com/iotaledger/iota.go/bundle"
	legacy "github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/encoding/b1t6"
	"github.com/iotaledger/iota.go/encoding/t5b1"
	"github.com/iotaledger/iota.go/merkle"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/iotaledger/iota.go/v2"

	_ "golang.org/x/crypto/blake2b" // import implementation
)

var hasher = whiteflag.NewHasher(crypto.BLAKE2b_512)

// Validator takes care of fetching and validating white-flag confirmation data from legacy nodes
// and wrapping them into receipts.
type Validator struct {
	api *api.API

	coordinatorAddress         trinary.Hash
	coordinatorMerkleTreeDepth int
}

// NewValidator creates a new Validator.
func NewValidator(api *api.API, coordinatorAddress trinary.Hash, coordinatorMerkleTreeDepth int) *Validator {
	return &Validator{
		api:                        api,
		coordinatorAddress:         coordinatorAddress,
		coordinatorMerkleTreeDepth: coordinatorMerkleTreeDepth,
	}
}

// QueryMigratedFunds queries the legacy network for the white-flag confirmation data for the given milestone
// index, verifies the signatures of the milestone and included bundles and then compiles a slice of migrated fund entries.
func (m *Validator) QueryMigratedFunds(milestoneIndex uint32) ([]*iota.MigratedFundsEntry, error) {
	confirmation, err := m.api.GetWhiteFlagConfirmation(milestoneIndex)
	if err != nil {
		return nil, fmt.Errorf("API call failed: %w", err)
	}

	included, err := m.validateConfirmation(confirmation, milestoneIndex)
	if err != nil {
		return nil, fmt.Errorf("invalid confirmation data: %w", err)
	}

	migrated := make([]*iota.MigratedFundsEntry, 0, len(included))
	for i := range included {
		output := included[i][0]
		edAddr, _ := address.ParseMigrationAddress(output.Address)
		entry := &iota.MigratedFundsEntry{
			Address: (*iota.Ed25519Address)(&edAddr),
			Deposit: uint64(output.Value),
		}
		copy(entry.TailTransactionHash[:], t5b1.EncodeTrytes(bundle.TailTransactionHash(included[i])))

		migrated = append(migrated, entry)
	}

	return migrated, nil
}

// validateMilestoneBundle performs syntactic validation of the milestone and checks whether it has the correct index.
func (m *Validator) validateMilestoneBundle(ms bundle.Bundle, msIndex uint32) error {
	// since in a milestone bundle only the (complete) head is signed, there is no need to validate other transactions
	head := ms[len(ms)-1]
	tag := trinary.IntToTrytes(int64(msIndex), legacy.TagTrinarySize/legacy.TritsPerTryte)
	lastIndex := uint64(len(ms) - 1)

	// the address must match the configured address
	if head.Address != m.coordinatorAddress {
		return legacy.ErrInvalidAddress
	}
	// the milestone must be a 0-value transaction
	if head.Value != 0 {
		return legacy.ErrInvalidValue
	}
	// the tag must match the milestone index
	if head.ObsoleteTag != tag || head.Tag != tag {
		return legacy.ErrInvalidTag
	}
	// the head transaction must indeed be the last transaction in the bundle
	if head.CurrentIndex != lastIndex || head.LastIndex != lastIndex {
		return legacy.ErrInvalidIndex
	}
	return nil
}

// validateMilestoneSignature validates the signature of the given milestone bundle.
func (m *Validator) validateMilestoneSignature(ms bundle.Bundle) error {
	head := ms[len(ms)-1]
	msData := head.SignatureMessageFragment
	msIndex := uint32(trinary.TrytesToInt(head.Tag))

	var auditPath []trinary.Trytes
	for i := 0; i < m.coordinatorMerkleTreeDepth; i++ {
		auditPath = append(auditPath, msData[i*legacy.HashTrytesSize:(i+1)*legacy.HashTrytesSize])
	}

	var fragments []trinary.Trytes
	for i := 0; i < len(ms)-1; i++ {
		fragments = append(fragments, ms[i].SignatureMessageFragment)
	}

	valid, err := merkle.ValidateSignatureFragments(m.coordinatorAddress, msIndex, auditPath, fragments, head.Hash)
	if err != nil {
		return fmt.Errorf("failed to validate signature: %w", err)
	}
	if !valid {
		return legacy.ErrInvalidSignature
	}
	return nil
}

// whiteFlagMerkleTreeHash returns the Merkle tree root of the included state-mutating transactions.
func (m *Validator) whiteFlagMerkleTreeHash(ms bundle.Bundle) ([]byte, error) {
	head := ms[len(ms)-1]
	data := head.SignatureMessageFragment[m.coordinatorMerkleTreeDepth*legacy.HashTrytesSize:]
	trytesLen := b1t6.EncodedLen(hasher.Size()) / legacy.TritsPerTryte
	hash, err := b1t6.DecodeTrytes(data[:trytesLen])
	if err != nil {
		return nil, err
	}
	return hash, nil
}

type trinaryHash trinary.Hash

func (h trinaryHash) MarshalBinary() ([]byte, error) {
	return t5b1.EncodeTrytes(trinary.Trytes(h)), nil
}

func asBundle(rawTrytes []trinary.Trytes) (bundle.Bundle, error) {
	txs, err := transaction.AsTransactionObjects(rawTrytes, nil)
	if err != nil {
		return nil, err
	}
	bundles := bundle.GroupTransactionsIntoBundles(txs)
	if l := len(bundles); l != 1 {
		return nil, fmt.Errorf("transactions from %d bundles instead of one", l)
	}
	return bundles[0], nil
}

func (m *Validator) validateConfirmation(confirmation *api.WhiteFlagConfirmation, msIndex uint32) ([]bundle.Bundle, error) {
	ms, err := asBundle(confirmation.MilestoneBundle)
	if err != nil {
		return nil, fmt.Errorf("failed to parse milestone bundle: %w", err)
	}
	if err := m.validateMilestoneBundle(ms, msIndex); err != nil {
		return nil, fmt.Errorf("invalid milestone bundle: %w", err)
	}
	if err := m.validateMilestoneSignature(ms); err != nil {
		return nil, fmt.Errorf("invalid milestone signature: %w", err)
	}

	var includedTails []encoding.BinaryMarshaler
	var includedBundles []bundle.Bundle
	for i, rawTrytes := range confirmation.IncludedBundles {
		bndl, err := asBundle(rawTrytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse included bundle %d: %w", i, err)
		}
		if err := bundle.ValidBundle(bndl, true); err != nil {
			return nil, fmt.Errorf("invalid included bundle %d: %w", i, err)
		}
		includedBundles = append(includedBundles, bndl)
		includedTails = append(includedTails, trinaryHash(bundle.TailTransactionHash(bndl)))
	}

	msMerkleHash, err := m.whiteFlagMerkleTreeHash(ms)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Merkle tree hash: %w", err)
	}
	merkleHash, err := hasher.Hash(includedTails)
	if err != nil {
		return nil, fmt.Errorf("failed to compute Merkle tree hash: %w", err)
	}
	if !bytes.Equal(msMerkleHash, merkleHash) {
		return nil, fmt.Errorf("invalid MerkleTreeHash %s", merkleHash)
	}
	return includedBundles, nil
}

func (m *Validator) nextMigrations(startIndex uint32) (uint32, []*iota.MigratedFundsEntry, error) {
	info, err := m.api.GetNodeInfo()
	if err != nil {
		return 0, nil, fmt.Errorf("failed to get node info: %w", err)
	}

	latestIndex := uint32(info.LatestMilestoneIndex)
	for index := startIndex; index <= latestIndex; index++ {
		migrated, err := m.QueryMigratedFunds(index)
		if err != nil {
			return 0, nil, fmt.Errorf("failed to query migration funds: %w", err)
		}
		if len(migrated) > 0 {
			return index, migrated, nil
		}
	}
	return latestIndex, nil, nil
}
