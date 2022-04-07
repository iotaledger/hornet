package milestonemanager_test

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/keymanager"
	"github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/pkg/model/milestonemanager"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/testsuite"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	BelowMaxDepth           = 15
	MinPowScore             = 1.0
	MilestonePublicKeyCount = 2
)

var (
	coordinatorPublicKeyRangesJSON = `
[{
  "key": "baff3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c",
  "start": 0,
  "end": 2
},
{
  "key": "ed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c",
  "start": 0,
  "end": 25
},
{
  "key": "f6752f5f46a53364e2ee9c4d662d762a81efd51010282a75cd6bd03f28ef349c",
  "start": 0,
  "end": 50
},
{
  "key": "ff752f5f46a53364e2ee9c4d662d762a81efd51010282a75cd6bd03f28ef349c",
  "start": 0,
  "end": 80
},
{
  "key": "b0752f5f46a53364e2ee9c4d662d762a81efd51010282a75cd6bd03f28ef349c",
  "start": 20,
  "end": 80
},
{
  "key": "c2752f5f46a53364e2ee9c4d662d762a81efd51010282a75cd6bd03f28ef349c",
  "start": 80,
  "end": 0
}]`
)

func initTest(testInterface testing.TB) (*testsuite.TestEnvironment, *milestonemanager.MilestoneManager) {

	te := testsuite.SetupTestEnvironment(testInterface, &iotago.Ed25519Address{}, 0, BelowMaxDepth, MinPowScore, false)

	getKeyManager := func() *keymanager.KeyManager {
		var coordinatorPublicKeyRanges coordinator.PublicKeyRanges

		err := json.Unmarshal([]byte(coordinatorPublicKeyRangesJSON), &coordinatorPublicKeyRanges)
		require.NoError(te.TestInterface, err)

		keyManager := keymanager.New()
		for _, keyRange := range coordinatorPublicKeyRanges {
			pubKey, err := utils.ParseEd25519PublicKeyFromString(keyRange.Key)
			require.NoError(te.TestInterface, err, "can't load public key ranges")
			keyManager.AddKeyRange(pubKey, keyRange.StartIndex, keyRange.EndIndex)
		}

		return keyManager
	}

	milestoneManager := milestonemanager.New(te.Storage(), te.SyncManager(), getKeyManager(), MilestonePublicKeyCount)

	return te, milestoneManager
}

func TestMilestoneManager_KeyManager(t *testing.T) {
	te, milestoneManager := initTest(t)
	defer te.CleanupTestEnvironment(true)

	/*
		{
		  "protocolVersion": 2,
		  "parentMessageIds": [
		    "0x4500be17d84dde04179a9619e202ba618da21debf6b7d07bbdcd22b9f7c41abf"
		  ],
		  "payload": {
		    "type": 7,
		    "index": 3,
		    "timestamp": 1649079954,
		    "parentMessageIds": [
		      "0x4500be17d84dde04179a9619e202ba618da21debf6b7d07bbdcd22b9f7c41abf"
		    ],
		    "inclusionMerkleProof": "0x0e5751c026e543b2e8ab2eb06099daa1d1e5df47778f7787faab45cdf12fe3a8",
		    "nextPoWScore": 0,
		    "nextPoWScoreMilestoneIndex": 0,
		    "signatures": [
		      {
		        "type": 0,
		        "publicKey": "0xed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c",
		        "signature": "0x4d690e7b869670cdb384c15987acf6cc0ade58232a8bbbe88d52ba12443fbc8bc90d2469d4973c643f8d0dce4afd2dced4c305add4c2c42a02aaa237b08d990a"
		      },
		      {
		        "type": 0,
		        "publicKey": "0xf6752f5f46a53364e2ee9c4d662d762a81efd51010282a75cd6bd03f28ef349c",
		        "signature": "0x12007c19da4aaa90da5aa2ae6351de7c8c77cd5bdca02b1ae5a3462ef0cce7aa7458c570f5d5cc0b0a573253e6d61d542dd53dbd569988673f4f9d30a435c30a"
		      }
		    ]
		  },
		  "nonce": "9223372036854786502"
		}
	*/

	milestoneMessageHex := "02014500be17d84dde04179a9619e202ba618da21debf6b7d07bbdcd22b9f7c41abf22010000070000000300000092f64a6200000000014500be17d84dde04179a9619e202ba618da21debf6b7d07bbdcd22b9f7c41abf0e5751c026e543b2e8ab2eb06099daa1d1e5df47778f7787faab45cdf12fe3a800000000000000000000000000000200ed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c4d690e7b869670cdb384c15987acf6cc0ade58232a8bbbe88d52ba12443fbc8bc90d2469d4973c643f8d0dce4afd2dced4c305add4c2c42a02aaa237b08d990a00f6752f5f46a53364e2ee9c4d662d762a81efd51010282a75cd6bd03f28ef349c12007c19da4aaa90da5aa2ae6351de7c8c77cd5bdca02b1ae5a3462ef0cce7aa7458c570f5d5cc0b0a573253e6d61d542dd53dbd569988673f4f9d30a435c30ac629000000000080"
	milestoneMessageBytes, err := hex.DecodeString(milestoneMessageHex)
	require.NoError(te.TestInterface, err)

	// build HORNET representation of the message
	msg, err := storage.MessageFromBytes(milestoneMessageBytes, serializer.DeSeriModePerformValidation, testsuite.DeSerializationParameters)
	require.NoError(te.TestInterface, err)

	// parse the milestone payload
	ms := msg.Milestone()
	require.NotNil(te.TestInterface, ms)

	verifiedMilestone := milestoneManager.VerifyMilestone(msg)
	require.NotNil(te.TestInterface, verifiedMilestone)
}
