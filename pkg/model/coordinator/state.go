package coordinator

import (
	"encoding"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
)

// State stores the latest state of the coordinator.
type State struct {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler

	LatestMilestoneIndex     milestone.Index
	LatestMilestoneMessageID hornet.MessageID
	LatestMilestoneTime      time.Time
}

// MarshalBinary returns the binary representation of the coordinator state.
func (cs *State) MarshalBinary() (data []byte, err error) {

	/*
		 4 bytes uint32 			LatestMilestoneIndex
		32 bytes     			    LatestMilestoneMessageID
		 8 bytes uint64 			LatestMilestoneTime
	*/

	data = make([]byte, 4+32+8)

	binary.LittleEndian.PutUint32(data[0:4], uint32(cs.LatestMilestoneIndex))
	copy(data[4:36], cs.LatestMilestoneMessageID)
	binary.LittleEndian.PutUint64(data[36:44], uint64(cs.LatestMilestoneTime.UnixNano()))

	return data, nil
}

// UnmarshalBinary parses the binary encoded representation of the coordinator state.
func (cs *State) UnmarshalBinary(data []byte) error {

	/*
		 4 bytes uint32 			LatestMilestoneIndex
		32 bytes     			    LatestMilestoneMessageID
		 8 bytes uint64 			LatestMilestoneTime
	*/

	if len(data) < 44 {
		return fmt.Errorf("not enough bytes to unmarshal state, expected: 44, got: %d", len(data))
	}

	cs.LatestMilestoneIndex = milestone.Index(binary.LittleEndian.Uint32(data[0:4]))
	cs.LatestMilestoneMessageID = hornet.MessageIDFromSlice(data[4:36])
	cs.LatestMilestoneTime = time.Unix(0, int64(binary.LittleEndian.Uint64(data[36:44])))

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
