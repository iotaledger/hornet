package utxo

import (
	"encoding/binary"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
	iotago "github.com/iotaledger/iota.go/v2"
)

// MilestoneDiff represents the generated and spent outputs by a milestone's confirmation.
type MilestoneDiff struct {
	kvStorable
	// The index of the milestone.
	Index milestone.Index
	// The outputs newly generated with this diff.
	Outputs Outputs
	// The outputs spent with this diff.
	Spents Spents
	// The treasury output this diff generated.
	TreasuryOutput *TreasuryOutput
	// The treasury output this diff consumed.
	SpentTreasuryOutput *TreasuryOutput
}

func milestoneDiffKeyForIndex(msIndex milestone.Index) []byte {
	m := marshalutil.New(5)
	m.WriteByte(UTXOStoreKeyPrefixMilestoneDiffs)
	m.WriteUint32(uint32(msIndex))
	return m.Bytes()
}

func (ms *MilestoneDiff) kvStorableKey() []byte {
	return milestoneDiffKeyForIndex(ms.Index)
}

func (ms *MilestoneDiff) kvStorableValue() []byte {

	m := marshalutil.New()

	m.WriteUint32(uint32(len(ms.Outputs)))
	for _, output := range ms.Outputs {
		m.WriteBytes(output.outputID[:])
	}

	m.WriteUint32(uint32(len(ms.Spents)))
	for _, spent := range ms.Spents {
		m.WriteBytes(spent.output.outputID[:])
	}

	if ms.TreasuryOutput != nil {
		m.WriteBool(true)
		m.WriteBytes(ms.TreasuryOutput.MilestoneID[:])
		m.WriteBytes(ms.SpentTreasuryOutput.MilestoneID[:])
		return m.Bytes()
	}

	m.WriteBool(false)

	return m.Bytes()
}

// note that this method relies on the data being available within other "tables".
func (ms *MilestoneDiff) kvStorableLoad(utxoManager *Manager, key []byte, value []byte) error {
	marshalUtil := marshalutil.New(value)

	outputCount, err := marshalUtil.ReadUint32()
	if err != nil {
		return err
	}

	outputs := make(Outputs, int(outputCount))
	for i := 0; i < int(outputCount); i++ {
		var outputID *iotago.UTXOInputID
		if outputID, err = parseOutputID(marshalUtil); err != nil {
			return err
		}

		output, err := utxoManager.ReadOutputByOutputIDWithoutLocking(outputID)
		if err != nil {
			return err
		}

		outputs[i] = output
	}

	spentCount, err := marshalUtil.ReadUint32()
	if err != nil {
		return err
	}

	spents := make(Spents, spentCount)
	for i := 0; i < int(spentCount); i++ {
		var outputID *iotago.UTXOInputID
		if outputID, err = parseOutputID(marshalUtil); err != nil {
			return err
		}

		spent, err := utxoManager.readSpentForOutputIDWithoutLocking(outputID)
		if err != nil {
			return err
		}

		spents[i] = spent
	}

	hasTreasury, err := marshalUtil.ReadBool()
	if err != nil {
		return err
	}

	if hasTreasury {
		treasuryOutputMilestoneID, err := marshalUtil.ReadBytes(iotago.MilestoneIDLength)
		if err != nil {
			return err
		}

		// try to read from unspent and spent
		treasuryOutput, err := utxoManager.readUnspentTreasuryOutputWithoutLocking(treasuryOutputMilestoneID)
		if err != nil {
			treasuryOutput, err = utxoManager.readSpentTreasuryOutputWithoutLocking(treasuryOutputMilestoneID)
			if err != nil {
				return err
			}
		}

		ms.TreasuryOutput = treasuryOutput

		spentTreasuryOutputMilestoneID, err := marshalUtil.ReadBytes(iotago.MilestoneIDLength)
		if err != nil {
			return err
		}

		spentTreasuryOutput, err := utxoManager.readSpentTreasuryOutputWithoutLocking(spentTreasuryOutputMilestoneID)
		if err != nil {
			return err
		}

		ms.SpentTreasuryOutput = spentTreasuryOutput
	}

	ms.Index = milestone.Index(binary.LittleEndian.Uint32(key[1:]))
	ms.Outputs = outputs
	ms.Spents = spents

	return nil
}

//- DB helpers

func storeDiff(diff *MilestoneDiff, mutations kvstore.BatchedMutations) error {
	return mutations.Set(diff.kvStorableKey(), diff.kvStorableValue())
}

func deleteDiff(msIndex milestone.Index, mutations kvstore.BatchedMutations) error {
	return mutations.Delete(milestoneDiffKeyForIndex(msIndex))
}

//- Manager

func (u *Manager) MilestoneDiffWithoutLocking(msIndex milestone.Index) (*MilestoneDiff, error) {

	key := milestoneDiffKeyForIndex(msIndex)

	value, err := u.utxoStorage.Get(key)
	if err != nil {
		return nil, err
	}

	diff := &MilestoneDiff{}
	if err := diff.kvStorableLoad(u, key, value); err != nil {
		return nil, err
	}

	return diff, nil
}

func (u *Manager) MilestoneDiff(msIndex milestone.Index) (*MilestoneDiff, error) {
	u.ReadLockLedger()
	defer u.ReadUnlockLedger()

	return u.MilestoneDiffWithoutLocking(msIndex)
}
