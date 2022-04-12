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
		    "0xa0115782522ac02944f34e242fde03b2429d4e9970294cb482c278ced7fd24ae"
		  ],
		  "payload": {
		    "type": 7,
		    "index": 3,
		    "timestamp": 1649751988,
		    "lastMilestone": "0x78fc378e77c2184e027165bf98ba7b25f3e8adcbffa59fe51efcbaf52e16a04e",
		    "parentMessageIds": [
		      "0xa0115782522ac02944f34e242fde03b2429d4e9970294cb482c278ced7fd24ae"
		    ],
		    "pastConeMerkleProof": "0x0e5751c026e543b2e8ab2eb06099daa1d1e5df47778f7787faab45cdf12fe3a8",
		    "inclusionMerkleProof": "0x0e5751c026e543b2e8ab2eb06099daa1d1e5df47778f7787faab45cdf12fe3a8",
		    "signatures": [
		      {
		        "type": 0,
		        "publicKey": "0xed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c",
		        "signature": "0x0a24928a544f60333fa8df3eb9626bdbb7ae92ba4af10a321bb124c1c7fdf239edd1bb349acada1616851c07719136322c2ed62f5742ef60ec0886de4d6a920d"
		      },
		      {
		        "type": 0,
		        "publicKey": "0xf6752f5f46a53364e2ee9c4d662d762a81efd51010282a75cd6bd03f28ef349c",
		        "signature": "0x9242864d1af533622a1006d4d79e87e0da3e175470279f6e23ec562f0b6acf05a69a379a6dcf2cbb8972e6c35f79b24055af8849a0119762ee372ce58547b000"
		      }
		    ]
		  },
		  "nonce": "4611686018427387984"
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
