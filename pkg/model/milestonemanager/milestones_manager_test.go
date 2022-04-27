package milestonemanager_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/core/protocfg"
	"github.com/gohornet/hornet/pkg/keymanager"
	"github.com/gohornet/hornet/pkg/model/milestonemanager"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/testsuite"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	BelowMaxDepth           = 15
	MinPoWScore             = 1.0
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

	te := testsuite.SetupTestEnvironment(testInterface, &iotago.Ed25519Address{}, 0, BelowMaxDepth, MinPoWScore, false)

	getKeyManager := func() *keymanager.KeyManager {
		var coordinatorPublicKeyRanges protocfg.ConfigPublicKeyRanges

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

	jsonString := `
	{
	  "protocolVersion": 2,
	  "parentMessageIds": [
	    "0x3d2ff2e7fc5ed7bfefe6c04830fabdb2f7a54e24c5804094cf278be75daa5a8f"
	  ],
	  "payload": {
	    "type": 7,
	    "index": 3,
	    "timestamp": 1650991380,
	    "previousMilestoneId": "0x2db243660a17056f65a271173f98d8342f7a4fd248f345e0f2b3692b3d0b2d9e",
	    "parentMessageIds": [
	      "0x3d2ff2e7fc5ed7bfefe6c04830fabdb2f7a54e24c5804094cf278be75daa5a8f"
	    ],
	    "confirmedMerkleRoot": "0x8b73cda5ef097a44ed6a709d8154e6ef52894f35cb8138aaf8009b734109d3cd",
	    "appliedMerkleRoot": "0x0e5751c026e543b2e8ab2eb06099daa1d1e5df47778f7787faab45cdf12fe3a8",
	    "signatures": [
	      {
	        "type": 0,
	        "publicKey": "0xed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c",
	        "signature": "0x186036b0d336cf4f0bd2e57c3e15fa475a6ea2eeee3abf613f2fde2cb5587fb43b0de17863f7a039f0eaf0f27002f57cc8674b4908117d1cc2314adbc2bb9102"
	      },
	      {
	        "type": 0,
	        "publicKey": "0xf6752f5f46a53364e2ee9c4d662d762a81efd51010282a75cd6bd03f28ef349c",
	        "signature": "0x22cd0e53c17aed121b3b744a32fdcd8e4b944f65541bebebe6a25dc5eae53ad9b911dd534332e638479861bd751012b3b1991cf8cc9ee69ba2752e66d3e1a40f"
	      }
	    ]
	  },
	  "nonce": "293"
	}
	`
	jsonMsg := &iotago.Message{}
	err := json.Unmarshal([]byte(jsonString), jsonMsg)
	require.NoError(t, err)
	milestoneMessageBytes, err := jsonMsg.Serialize(serializer.DeSeriModePerformValidation, te.ProtocolParameters())
	require.NoError(t, err)

	// build HORNET representation of the message
	msg, err := storage.MessageFromBytes(milestoneMessageBytes, serializer.DeSeriModePerformValidation, te.ProtocolParameters())
	require.NoError(te.TestInterface, err)

	// parse the milestone payload
	milestonePayload := msg.Milestone()
	require.NotNil(te.TestInterface, milestonePayload)

	verifiedMilestone := milestoneManager.VerifyMilestoneMessage(msg.Message())
	require.NotNil(te.TestInterface, verifiedMilestone)
}
