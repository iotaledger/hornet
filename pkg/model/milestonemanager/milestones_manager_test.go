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
        "0x288bae9cf495bfa8dc10cb69f92995119952f32eb84122dd925d4b750a2bbc0d"
      ],
      "payload": {
        "type": 7,
        "index": 3,
        "timestamp": 1650373039,
        "lastMilestone": "0xea69e4d7c7d5796b8811d037022ca1ade509fb7aed61ffdfcde67a48930a56da",
        "parentMessageIds": [
          "0x288bae9cf495bfa8dc10cb69f92995119952f32eb84122dd925d4b750a2bbc0d"
        ],
        "confirmedMerkleRoot": "0x116e745362e089c3d92e68db006920582df62ca8d40f1d93c98100209e8fa707",
        "appliedMerkleRoot": "0x0e5751c026e543b2e8ab2eb06099daa1d1e5df47778f7787faab45cdf12fe3a8",
        "signatures": [
          {
            "type": 0,
            "publicKey": "0xed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c",
            "signature": "0x6ffecf4d02b5d917d39b5c66a774c11d599f9e892156a82e758a9433f83d98a43d2e850e50efe7b4cc8618c7a8f393136f378dd78fe4b819aebdd0146c304209"
          },
          {
            "type": 0,
            "publicKey": "0xf6752f5f46a53364e2ee9c4d662d762a81efd51010282a75cd6bd03f28ef349c",
            "signature": "0xb5b8e5a6d0fd66e442200096befb01c8325db7a140ab1f7e87fc7e4559aaf99856c4729c77155e7daea6c8f2c96acec11c50f6fdfb50c7720980c845828b1d01"
          }
        ]
      },
      "nonce": "13835058055282164128"
    }
	`
	jsonMsg := &iotago.Message{}
	err := json.Unmarshal([]byte(jsonString), jsonMsg)
	require.NoError(t, err)
	milestoneMessageBytes, err := jsonMsg.Serialize(serializer.DeSeriModePerformValidation, iotago.ZeroRentParas)
	require.NoError(t, err)

	// build HORNET representation of the message
	msg, err := storage.MessageFromBytes(milestoneMessageBytes, serializer.DeSeriModePerformValidation, testsuite.DeSerializationParameters)
	require.NoError(te.TestInterface, err)

	// parse the milestone payload
	ms := msg.Milestone()
	require.NotNil(te.TestInterface, ms)

	verifiedMilestone := milestoneManager.VerifyMilestone(msg)
	require.NotNil(te.TestInterface, verifiedMilestone)
}
