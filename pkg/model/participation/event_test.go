package participation_test

import (
	"math/rand"

	"github.com/gohornet/hornet/pkg/model/participation"
	"github.com/iotaledger/hive.go/testutil"
)

// TODO: Add tests for serialization

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandString(strLen int) string {
	b := make([]byte, strLen)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func RandEventID() participation.EventID {
	eventID := participation.EventID{}
	copy(eventID[:], testutil.RandBytes(participation.EventIDLength))
	return eventID
}
