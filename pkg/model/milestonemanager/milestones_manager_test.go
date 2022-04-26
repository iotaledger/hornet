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
	    "0x1c77435ecd0f9c1fb9ad4179c6f12c9daf1e38051fe9b5bdf5325578b2d508aa"
	  ],
	  "payload": {
	    "type": 7,
	    "index": 3,
	    "timestamp": 1650451900,
	    "lastMilestoneId": "0x35339a1bbb8ab044ae8490442fa0d511efa0994f59746ebbee642ac799a4d1f1",
	    "parentMessageIds": [
	      "0x1c77435ecd0f9c1fb9ad4179c6f12c9daf1e38051fe9b5bdf5325578b2d508aa"
	    ],
	    "confirmedMerkleRoot": "0xc29fd0895481e630fa6dea5f06cfb07048ec641cf9e3d39e528b0df09f0af7ea",
	    "appliedMerkleRoot": "0x0e5751c026e543b2e8ab2eb06099daa1d1e5df47778f7787faab45cdf12fe3a8",
	    "signatures": [
	      {
	        "type": 0,
	        "publicKey": "0xed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c",
	        "signature": "0xbe83584cc1223327156a667f8f2e0813db850e1d2acdacecd76439bd8da0a18f1c21a8dd6982340890d48cb53100bd39fe70c38a9a13199bd6baed4ee09e3d02"
	      },
	      {
	        "type": 0,
	        "publicKey": "0xf6752f5f46a53364e2ee9c4d662d762a81efd51010282a75cd6bd03f28ef349c",
	        "signature": "0x4ac5c7b70c1ec21e702847baad4717c20f0fbc4f8c252ef86ef01cf4a65b2a536c77c30b8dd6a062c47947037745676c7a6fb274cd2151f3e1063a18274b5308"
	      }
	    ]
	  },
	  "nonce": "13835058055282164240"
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
	ms := msg.Milestone()
	require.NotNil(te.TestInterface, ms)

	verifiedMilestone := milestoneManager.VerifyMilestone(msg)
	require.NotNil(te.TestInterface, verifiedMilestone)
}
