package migrator

import (
	"bytes"
	"crypto"
	"fmt"
	"math"

	"github.com/pkg/errors"

	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/iota.go/address"
	"github.com/iotaledger/iota.go/api"
	"github.com/iotaledger/iota.go/bundle"
	legacy "github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/encoding/b1t6"
	"github.com/iotaledger/iota.go/encoding/t5b1"
	"github.com/iotaledger/iota.go/merkle"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/merklehasher"

	// import implementation.
	_ "golang.org/x/crypto/blake2b"
)

// ErrEmptyBundle is returned when a bundle contains no transaction.
var ErrEmptyBundle = errors.New("empty bundle")

var hasher = merklehasher.NewHasher(crypto.BLAKE2b_512)

// LegacyAPI defines the calls of the legacy API that are used.
type LegacyAPI interface {
	GetNodeInfo() (*api.GetNodeInfoResponse, error)
	GetWhiteFlagConfirmation(msIndex iotago.MilestoneIndex) (*api.WhiteFlagConfirmation, error)
}

// Validator takes care of fetching and validating white-flag confirmation data from legacy nodes
// and wrapping them into receipts.
type Validator struct {
	api LegacyAPI

	coordinatorAddress         trinary.Hash
	coordinatorMerkleTreeDepth int
}

// NewValidator creates a new Validator.
func NewValidator(api LegacyAPI, coordinatorAddress trinary.Hash, coordinatorMerkleTreeDepth int) *Validator {
	return &Validator{
		api:                        api,
		coordinatorAddress:         coordinatorAddress,
		coordinatorMerkleTreeDepth: coordinatorMerkleTreeDepth,
	}
}

// QueryMigratedFunds queries the legacy network for the white-flag confirmation data for the given milestone
// index, verifies the signatures of the milestone and included bundles and then compiles a slice of migrated fund entries.
func (m *Validator) QueryMigratedFunds(msIndex iotago.MilestoneIndex) ([]*iotago.MigratedFundsEntry, error) {
	confirmation, err := m.api.GetWhiteFlagConfirmation(msIndex)
	if err != nil {
		return nil, common.SoftError(fmt.Errorf("API call failed: %w", err))
	}

	included, err := m.validateConfirmation(confirmation, msIndex)
	if err != nil {
		return nil, common.CriticalError(fmt.Errorf("invalid confirmation data: %w", err))
	}

	migrated := make([]*iotago.MigratedFundsEntry, 0, len(included))
	for i := range included {
		output := included[i][0]
		// don't need to check the error here because if it wouldn't be a migration address,
		// it wouldn't have passed 'validateConfirmation' in the first place
		edAddr, _ := address.ParseMigrationAddress(output.Address)
		entry := &iotago.MigratedFundsEntry{
			Address: (*iotago.Ed25519Address)(&edAddr),
			Deposit: uint64(output.Value),
		}
		copy(entry.TailTransactionHash[:], t5b1.EncodeTrytes(bundle.TailTransactionHash(included[i])))

		migrated = append(migrated, entry)
	}

	return migrated, nil
}

// QueryNextMigratedFunds queries the next existing migrations starting from milestone index startIndex.
// It returns the migrations as well as milestone index that confirmed those migrations.
// If there are currently no more migrations, it returns the latest milestone index that was checked.
func (m *Validator) QueryNextMigratedFunds(startIndex iotago.MilestoneIndex) (iotago.MilestoneIndex, []*iotago.MigratedFundsEntry, error) {
	latestIndex, err := m.queryLatestMilestoneIndex()
	if err != nil {
		return 0, nil, common.SoftError(fmt.Errorf("failed to get node info: %w", err))
	}

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

// queryLatestMilestoneIndex uses getNodeInfo API call to query the index of the latest milestone.
func (m *Validator) queryLatestMilestoneIndex() (iotago.MilestoneIndex, error) {
	info, err := m.api.GetNodeInfo()
	if err != nil {
		return 0, err
	}

	index := info.LatestSolidSubtangleMilestoneIndex
	// do some sanity checks
	if index < 0 || index >= math.MaxUint32 {
		return 0, fmt.Errorf("invalid milestone index in response: %d", index)
	}

	return iotago.MilestoneIndex(index), nil
}

// validateMilestoneBundle performs syntactic validation of the milestone and checks whether it has the correct index.
func (m *Validator) validateMilestoneBundle(ms bundle.Bundle, msIndex iotago.MilestoneIndex) error {
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
	msIndex := iotago.MilestoneIndex(trinary.TrytesToInt(head.Tag))

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
	if len(rawTrytes) == 0 {
		return nil, ErrEmptyBundle
	}

	txs, err := transaction.AsTransactionObjects(rawTrytes, nil)
	if err != nil {
		return nil, err
	}

	// validate the bundle, but also accept non-migration milestone bundles
	if err := bundle.ValidBundle(txs, false); err != nil {
		return nil, err
	}

	return txs, nil
}

func (m *Validator) validateConfirmation(confirmation *api.WhiteFlagConfirmation, msIndex iotago.MilestoneIndex) ([]bundle.Bundle, error) {
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

	includedTails := make([][]byte, len(confirmation.IncludedBundles))
	includedBundles := make([]bundle.Bundle, len(confirmation.IncludedBundles))
	for i, rawTrytes := range confirmation.IncludedBundles {
		bndl, err := asBundle(rawTrytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse included bundle %d: %w", i, err)
		}

		if err := bundle.ValidBundle(bndl, true); err != nil {
			return nil, fmt.Errorf("invalid included bundle %d: %w", i, err)
		}
		includedBundles[i] = bndl

		tailBytes, err := trinaryHash(bundle.TailTransactionHash(bndl)).MarshalBinary()
		if err != nil {
			return nil, err
		}
		includedTails[i] = tailBytes
	}

	msMerkleHash, err := m.whiteFlagMerkleTreeHash(ms)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Merkle tree hash: %w", err)
	}

	merkleHash := hasher.Hash(includedTails)
	if !bytes.Equal(msMerkleHash, merkleHash) {
		return nil, fmt.Errorf("invalid MerkleTreeHash %s", merkleHash)
	}

	return includedBundles, nil
}
