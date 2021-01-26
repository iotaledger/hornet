package utxo

import (
	"encoding/binary"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
	iotago "github.com/iotaledger/iota.go"
)

type MilestoneDiff struct {
	kvStorable

	Index   milestone.Index
	Outputs Outputs
	Spents  Spents
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

	m := marshalutil.New(4 + len(ms.Outputs)*34 + len(ms.Spents)*67)

	m.WriteUint32(uint32(len(ms.Outputs)))
	for _, output := range ms.Outputs {
		m.WriteBytes(output.outputID[:])
	}

	m.WriteUint32(uint32(len(ms.Spents)))
	for _, spent := range ms.Spents {
		m.WriteBytes(spent.output.addressBytes())
		m.WriteBytes(spent.output.outputID[:])
	}

	return m.Bytes()
}

func (ms *MilestoneDiff) kvStorableLoad(utxoManager *Manager, key []byte, value []byte) error {
	marshalUtil := marshalutil.New(value)

	var outputs Outputs
	var spents Spents

	outputCount, err := marshalUtil.ReadUint32()
	if err != nil {
		return err
	}

	for i := 0; i < int(outputCount); i++ {
		var outputID *iotago.UTXOInputID
		if outputID, err = parseOutputID(marshalUtil); err != nil {
			return err
		}

		output, err := utxoManager.ReadOutputByOutputIDWithoutLocking(outputID)
		if err != nil {
			return err
		}

		outputs = append(outputs, output)
	}

	spentCount, err := marshalUtil.ReadUint32()
	if err != nil {
		return err
	}

	for i := 0; i < int(spentCount); i++ {
		if _, err := parseAddress(marshalUtil); err != nil {
			return err
		}

		var outputID *iotago.UTXOInputID
		if outputID, err = parseOutputID(marshalUtil); err != nil {
			return err
		}

		spent, err := utxoManager.readSpentForOutputIDWithoutLocking(outputID)
		if err != nil {
			return err
		}

		spents = append(spents, spent)
	}

	ms.Index = milestone.Index(binary.LittleEndian.Uint32(key))
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

func (u *Manager) GetMilestoneDiffWithoutLocking(msIndex milestone.Index) (*MilestoneDiff, error) {

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

func (u *Manager) GetMilestoneDiff(msIndex milestone.Index) (*MilestoneDiff, error) {
	u.ReadLockLedger()
	defer u.ReadUnlockLedger()

	return u.GetMilestoneDiffWithoutLocking(msIndex)
}
