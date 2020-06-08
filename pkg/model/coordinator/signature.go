package coordinator

import (
	"fmt"

	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/kerl"
	"github.com/iotaledger/iota.go/mam/v1/merkle"
	legacySigning "github.com/iotaledger/iota.go/mam/v1/signing"
	"github.com/iotaledger/iota.go/signing"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/model/milestone"
)

// Address generates an address deterministically, according to the given seed, milestone index and security level.
// a modified key derivation function is used to avoid the M-bug.
func Address(seed trinary.Hash, index milestone.Index, securityLvl int) (trinary.Hash, error) {

	subSeedTrits, err := signing.Subseed(seed, uint64(index), kerl.NewKerl())
	if err != nil {
		return "", err
	}

	keyTrits, err := legacySigning.Key(subSeedTrits, consts.SecurityLevel(securityLvl), kerl.NewKerl())
	if err != nil {
		return "", err
	}

	digestsTrits, err := signing.Digests(keyTrits, kerl.NewKerl())
	if err != nil {
		return "", err
	}

	addressTrits, err := signing.Address(digestsTrits, kerl.NewKerl())
	if err != nil {
		return "", err
	}

	address, err := trinary.TritsToTrytes(addressTrits)
	if err != nil {
		return "", err
	}

	return address, nil
}

// signature signs the normalized hash of a given hash, according to the given seed, milestone index and security level.
func signature(seed trinary.Hash, index milestone.Index, securityLvl int, hashToSign trinary.Trytes) (trinary.Trytes, error) {

	subSeedTrits, err := signing.Subseed(seed, uint64(index), kerl.NewKerl())
	if err != nil {
		return "", err
	}

	keyTrits, err := legacySigning.Key(subSeedTrits, consts.SecurityLevel(securityLvl), kerl.NewKerl())
	if err != nil {
		return "", err
	}

	// milestones sign the normalized hash of the sibling transaction.
	normalizedBundleHashTrits := signing.NormalizedBundleHash(hashToSign)

	signatureTrits := make(trinary.Trits, securityLvl*consts.KeyFragmentLength)
	for i := 0; i < securityLvl; i++ {
		fragmentTrits, err := signing.SignatureFragment(normalizedBundleHashTrits[i*consts.KeySegmentsPerFragment:(i+1)*consts.KeySegmentsPerFragment], keyTrits[i*consts.KeyFragmentLength:(i+1)*consts.KeyFragmentLength])
		if err != nil {
			return "", err
		}
		copy(signatureTrits[i*consts.KeyFragmentLength:(i+1)*consts.KeyFragmentLength], fragmentTrits)
	}

	signature, err := trinary.TritsToTrytes(signatureTrits)
	if err != nil {
		return "", err
	}

	return signature, nil
}

// validateSignature checks if the milestone has the correct signature.
func validateSignature(root trinary.Hash, milestoneIndex milestone.Index, securityLvl int, hashToSign trinary.Hash, signature trinary.Trytes, siblingsTrytes trinary.Hash) error {

	normalizedBundleHashFragments := make([]trinary.Trits, securityLvl)

	// milestones sign the normalized hash of the sibling transaction.
	normalizeBundleHash := signing.NormalizedBundleHash(hashToSign)

	for i := 0; i < securityLvl; i++ {
		normalizedBundleHashFragments[i] = normalizeBundleHash[i*consts.KeySegmentsPerFragment : (i+1)*consts.KeySegmentsPerFragment]
	}

	signatureMessageFragmentTrits, err := trinary.TrytesToTrits(signature)
	if err != nil {
		return err
	}

	digests := make(trinary.Trits, securityLvl*consts.HashTrinarySize)
	for i := 0; i < securityLvl; i++ {
		digest, err := signing.Digest(normalizedBundleHashFragments[i%consts.MaxSecurityLevel], signatureMessageFragmentTrits[i*consts.KeyFragmentLength:(i+1)*consts.KeyFragmentLength], kerl.NewKerl())
		if err != nil {
			return err
		}

		copy(digests[i*consts.HashTrinarySize:], digest)
	}

	addressTrits, err := signing.Address(digests, kerl.NewKerl())
	if err != nil {
		return err
	}

	siblingsTrits, err := trinary.TrytesToTrits(siblingsTrytes)
	if err != nil {
		return err
	}

	// validate Merkle path
	merkleRoot, err := merkle.MerkleRoot(
		addressTrits,
		siblingsTrits,
		uint64(len(siblingsTrits)/consts.HashTrinarySize),
		uint64(milestoneIndex),
		kerl.NewKerl(),
	)
	if err != nil {
		return err
	}

	merkleAddress, err := trinary.TritsToTrytes(merkleRoot)
	if err != nil {
		return err
	}

	if merkleAddress != root {
		return fmt.Errorf("merkle root does not match: %v != %v", merkleAddress, root)
	}

	return nil
}
