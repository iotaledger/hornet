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
	"key": "a9b46fe743df783dedd00c954612428b34241f5913cf249d75bed3aafd65e4cd",
	"start": 0,
	"end": 777600
},
{
	"key": "365fb85e7568b9b32f7359d6cbafa9814472ad0ecbad32d77beaf5dd9e84c6ba",
	"start": 0,
	"end": 1555200
},
{
	"key": "ba6d07d1a1aea969e7e435f9f7d1b736ea9e0fcb8de400bf855dba7f2a57e947",
	"start": 552960,
	"end": 2108160
},
{
  "key": "760d88e112c0fd210cf16a3dce3443ecf7e18c456c2fb9646cabb2e13e367569",
  "start": 1333460,
  "end": 2888660
},
{
  "key": "7bac2209b576ea2235539358c7df8ca4d2f2fc35a663c760449e65eba9f8a6e7",
  "start": 2111060,
  "end": 3666260
},
{
  "key": "edd9c639a719325e465346b84133bf94740b7d476dd87fc949c0e8df516f9954",
  "start": 2888660,
  "end": 4443860
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

	milestoneMessageHex := "b77f44715e0b30140612f48b64627ac6b754e5c3d313a878aa11f6e34fc3263134649c20d03b96d5ca3af97facd880c5cd8413a75c10b8372af44d5d65cd220e31026e868c55d5b15f68cc042cdaf55d7d00d6a553782e8bcd1c563aa2535f7a786f97b67b660c5fb39c8fea1ae3a3b2bdc3bdbdb590078b4da174714ac919222064f8051293029e2cd8b69354dd7936ee50abb76d53a2a60bc97a5dc86e5b75d8b2fcfba718181ed2e10b6d13233ac0158f9277fbf2bc236769ac5193896fcd9c83f7703a05e0e4d21f02000001000000ce5a140093b65561000000000612f48b64627ac6b754e5c3d313a878aa11f6e34fc3263134649c20d03b96d5ca3af97facd880c5cd8413a75c10b8372af44d5d65cd220e31026e868c55d5b15f68cc042cdaf55d7d00d6a553782e8bcd1c563aa2535f7a786f97b67b660c5fb39c8fea1ae3a3b2bdc3bdbdb590078b4da174714ac919222064f8051293029e2cd8b69354dd7936ee50abb76d53a2a60bc97a5dc86e5b75d8b2fcfba718181ed2e10b6d13233ac0158f9277fbf2bc236769ac5193896fcd9c83f7703a05e0e4d20e5751c026e543b2e8ab2eb06099daa1d1e5df47778f7787faab45cdf12fe3a8000000000000000003365fb85e7568b9b32f7359d6cbafa9814472ad0ecbad32d77beaf5dd9e84c6ba760d88e112c0fd210cf16a3dce3443ecf7e18c456c2fb9646cabb2e13e367569ba6d07d1a1aea969e7e435f9f7d1b736ea9e0fcb8de400bf855dba7f2a57e9470000000003702f9c530fe08343d5bec6a621897d4a82d59594814e5308e7c48238a6783deb34f3691f80b340408c5c6317be5136c07387ca4f1334767523247732b317130d48c47c016b24ce2a020734f35e354dda917ec92d10700e06cf270bd18691d364f9fdc4c1e4a2257bcc7a91450f31bd83141806c451b9fd095e5c77428bc7560074defe516c10ac16572ce33f97c0d8c5f65402e267cc1e43d495a69418ee5802524fbf726009bcddc42909787f449c1146c3865ae47ec9beae892ec721b3c2088ab5f98aaff88aaf"
	milestoneMessageBytes, err := hex.DecodeString(milestoneMessageHex)
	require.NoError(te.TestInterface, err)

	// build HORNET representation of the message
	msg, err := storage.MessageFromBytes(milestoneMessageBytes, serializer.DeSeriModePerformValidation)
	require.NoError(te.TestInterface, err)

	// parse the milestone payload
	ms := msg.Milestone()
	require.NotNil(te.TestInterface, ms)

	verifiedMilestone := milestoneManager.VerifyMilestone(msg)
	require.NotNil(te.TestInterface, verifiedMilestone)
}
