//nolint:forcetypeassert,varnamelen,revive,exhaustruct // we don't care about these linters in test cases
package milestonemanager_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/core/crypto"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hornet/v2/core/protocfg"
	"github.com/iotaledger/hornet/v2/pkg/model/milestonemanager"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/testsuite"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/keymanager"
)

const (
	ProtocolVersion         = 2
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

	te := testsuite.SetupTestEnvironment(testInterface, &iotago.Ed25519Address{}, 0, ProtocolVersion, BelowMaxDepth, MinPoWScore, false)

	getKeyManager := func() *keymanager.KeyManager {
		var coordinatorPublicKeyRanges protocfg.ConfigPublicKeyRanges

		err := json.Unmarshal([]byte(coordinatorPublicKeyRangesJSON), &coordinatorPublicKeyRanges)
		require.NoError(te.TestInterface, err)

		keyManager := keymanager.New()
		for _, keyRange := range coordinatorPublicKeyRanges {
			pubKey, err := crypto.ParseEd25519PublicKeyFromString(keyRange.Key)
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
	  "parents": [
	    "0x35fefbb0927ce9da0f4c6f5567ec9d390ec4f5dc88a610ecbd2aa6cb08628f5e"
	  ],
	  "payload": {
	    "type": 7,
	    "index": 3,
	    "timestamp": 1651838930,
	    "protocolVersion": 2,
	    "previousMilestoneId": "0xd3732082d3aed87e6fc29c006c290a2dc708a3d3a7f1d30f5ed54ab6a511138b",
	    "parents": [
	      "0x35fefbb0927ce9da0f4c6f5567ec9d390ec4f5dc88a610ecbd2aa6cb08628f5e"
	    ],
	    "inclusionMerkleRoot": "0xf4e43e9b04c116777a25a5f216855edf7ef6b4235685d15e51d6ed53a2c1c06d",
	    "appliedMerkleRoot": "0x0e5751c026e543b2e8ab2eb06099daa1d1e5df47778f7787faab45cdf12fe3a8",
	    "signatures": [
	      {
	        "type": 0,
	        "publicKey": "0xed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c",
	        "signature": "0xad41bb0fd27b03ffd4fd8cdbf1cdcb8d3b3ab27b304f2779663a26c7a98283e9417cbdc207a0d040d1298e30365250148a597d65d46c7da7400e115cec80870e"
	      },
	      {
	        "type": 0,
	        "publicKey": "0xf6752f5f46a53364e2ee9c4d662d762a81efd51010282a75cd6bd03f28ef349c",
	        "signature": "0xa2d80950f636c5e21d73780d7240c0b12a5224037d97ea2e32b3dcea4aa70f9b9de71c1361912a126d15c4581a2d15953a0056126b92e94ce36cd1fdddbffa09"
	      }
	    ]
	  },
	  "nonce": "0"
	}
	`
	jsonBlock := &iotago.Block{}
	err := json.Unmarshal([]byte(jsonString), jsonBlock)
	require.NoError(t, err)
	milestoneBlockBytes, err := jsonBlock.Serialize(serializer.DeSeriModePerformValidation, te.ProtocolParameters())
	require.NoError(t, err)

	// build HORNET representation of the block
	block, err := storage.BlockFromBytes(milestoneBlockBytes, serializer.DeSeriModePerformValidation, te.ProtocolParameters())
	require.NoError(te.TestInterface, err)

	// parse the milestone payload
	milestonePayload := block.Milestone()
	require.NotNil(te.TestInterface, milestonePayload)

	verifiedMilestone := milestoneManager.VerifyMilestoneBlock(block.Block())
	require.NotNil(te.TestInterface, verifiedMilestone)
}
