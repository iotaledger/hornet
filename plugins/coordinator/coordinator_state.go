package coordinator

import (
	"encoding"
	"encoding/binary"
	"io/ioutil"
	"os"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/model/milestone"
)

type CoordinatorState struct {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler

	latestMilestoneIndex        milestone.Index
	latestMilestoneHash         trinary.Hash
	latestMilestoneTime         int64
	latestMilestoneTransactions []trinary.Hash
}

func (cs *CoordinatorState) MarshalBinary() (data []byte, err error) {

	data = make([]byte, 4+8+49*(1+len(cs.latestMilestoneTransactions)))

	binary.LittleEndian.PutUint32(data[0:4], uint32(cs.latestMilestoneIndex))
	copy(data[4:53], trinary.MustTrytesToBytes(cs.latestMilestoneHash))
	binary.LittleEndian.PutUint64(data[53:61], uint64(cs.latestMilestoneTime))

	offset := 61
	for _, txHash := range cs.latestMilestoneTransactions {
		copy(data[offset:offset+49], trinary.MustTrytesToBytes(txHash))
		offset += 49
	}

	return data, nil
}

func (cs *CoordinatorState) UnmarshalBinary(data []byte) error {

	/*
		 4 bytes uint32 			latestMilestoneIndex
		49 bytes     			    latestMilestoneHash
		 8 bytes uint64 			latestMilestoneTime
		49 bytes                    latestMilestoneTransactions	(x latestMilestoneTransactionsCount)
	*/

	cs.latestMilestoneIndex = milestone.Index(binary.LittleEndian.Uint32(data[0:4]))
	cs.latestMilestoneHash = trinary.MustBytesToTrytes(data[4:53], 81)
	cs.latestMilestoneTime = int64(binary.LittleEndian.Uint64(data[53:61]))
	cs.latestMilestoneTransactions = make([]trinary.Hash, 0)

	latestMilestoneTransactionsCount := (len(data) - 61) / 49

	offset := 61
	for i := 0; i < latestMilestoneTransactionsCount; i++ {
		cs.latestMilestoneTransactions = append(cs.latestMilestoneTransactions, trinary.MustBytesToTrytes(data[offset:offset+49], 81))
		offset += 49
	}

	return nil
}

func loadStateFile(filePath string) (*CoordinatorState, error) {

	stateFile, err := os.OpenFile(filePath, os.O_RDONLY, 0666)
	if err != nil {
		return nil, err
	}
	defer stateFile.Close()

	data, err := ioutil.ReadAll(stateFile)
	if err != nil {
		return nil, err
	}

	result := &CoordinatorState{}
	if err := result.UnmarshalBinary(data); err != nil {
		return nil, err
	}

	return result, nil
}

func (cs *CoordinatorState) storeStateFile(filePath string) error {

	data, err := cs.MarshalBinary()
	if err != nil {
		return err
	}

	stateFile, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0660)
	if err != nil {
		return err
	}
	defer stateFile.Close()

	if _, err := stateFile.Write(data); err != nil {
		return err
	}

	if err := stateFile.Sync(); err != nil {
		return err
	}

	return nil
}
