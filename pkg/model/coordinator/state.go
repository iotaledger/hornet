package coordinator

import (
	"encoding"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
)

// State stores the latest state of the coordinator.
type State struct {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler

	LatestMilestoneIndex milestone.Index
	LatestMilestoneHash  hornet.Hash
	LatestMilestoneTime  int64

	// LatestMilestoneTransactions are the transaction hashes of the latest milestone
	LatestMilestoneTransactions hornet.Hashes
}

// MarshalBinary returns the binary representation of the coordinator state.
func (cs *State) MarshalBinary() (data []byte, err error) {

	/*
		 4 bytes uint32 			LatestMilestoneIndex
		49 bytes     			    LatestMilestoneHash
		 8 bytes uint64 			LatestMilestoneTime
		49 bytes                    LatestMilestoneTransactions	(x latestMilestoneTransactionsCount)
	*/

	data = make([]byte, 4+49+8+(49*len(cs.LatestMilestoneTransactions)))

	binary.LittleEndian.PutUint32(data[0:4], uint32(cs.LatestMilestoneIndex))
	copy(data[4:53], cs.LatestMilestoneHash)
	binary.LittleEndian.PutUint64(data[53:61], uint64(cs.LatestMilestoneTime))

	offset := 61
	for _, txHash := range cs.LatestMilestoneTransactions {
		copy(data[offset:offset+49], txHash)
		offset += 49
	}

	return data, nil
}

// UnmarshalBinary parses the binary encoded representation of the coordinator state.
func (cs *State) UnmarshalBinary(data []byte) error {

	/*
		 4 bytes uint32 			LatestMilestoneIndex
		49 bytes     			    LatestMilestoneHash
		 8 bytes uint64 			LatestMilestoneTime
		49 bytes                    LatestMilestoneTransactions	(x latestMilestoneTransactionsCount)
	*/

	if len(data) < 61 {
		return fmt.Errorf("not enough bytes to unmarshal state, expected: 61, got: %d", len(data))
	}

	cs.LatestMilestoneIndex = milestone.Index(binary.LittleEndian.Uint32(data[0:4]))
	cs.LatestMilestoneHash = hornet.Hash(data[4:53])
	cs.LatestMilestoneTime = int64(binary.LittleEndian.Uint64(data[53:61]))
	cs.LatestMilestoneTransactions = make(hornet.Hashes, 0)

	latestMilestoneTransactionsCount := (len(data) - 61) / 49

	offset := 61
	for i := 0; i < latestMilestoneTransactionsCount; i++ {
		cs.LatestMilestoneTransactions = append(cs.LatestMilestoneTransactions, hornet.Hash(data[offset:offset+49]))
		offset += 49
	}

	return nil
}

// loadStateFile loads the binary state file and unmarshals it.
func loadStateFile(filePath string) (*State, error) {

	stateFile, err := os.OpenFile(filePath, os.O_RDONLY, 0666)
	if err != nil {
		return nil, err
	}
	defer stateFile.Close()

	data, err := ioutil.ReadAll(stateFile)
	if err != nil {
		return nil, err
	}

	result := &State{}
	if err := result.UnmarshalBinary(data); err != nil {
		return nil, err
	}

	return result, nil
}

// storeStateFile stores the state file for the coordinator in binary format.
func (cs *State) storeStateFile(filePath string) error {

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
