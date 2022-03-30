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

	//milestoneString := "{\n  \"protocolVersion\": 2,\n  \"parentMessageIds\": [\n    \"0x2a2133f1b898dfdbd27e21db98f588eb661e671487121f273fe496ddc227df1f\",\n    \"0x6382e7fa4814f4524ff40b989b30713165e0ac5674b6ab1a48ac4e290aca0386\",\n    \"0x86524d7366db1e444004400477746935ef6edabf634f0d7118b7ab263062eeeb\",\n    \"0xa1a82ba3fddd0be9a34e8bc6ecaf2b6018e2ec5b9e1711d267e713f1f1375d46\",\n    \"0xbb51eff82fb9b517c71fa5f9ee948a8056efb79b5aa4c25815193527ee69ea0e\",\n    \"0xc6114dd90d0bb33b915527109ab4b2eb7ecef37d78e36877d15839d6814790e8\"\n  ],\n  \"payload\": {\n    \"type\": 7,\n    \"index\": 3,\n    \"timestamp\": 1648650914,\n    \"parentMessageIds\": [\n      \"0x2a2133f1b898dfdbd27e21db98f588eb661e671487121f273fe496ddc227df1f\",\n      \"0x6382e7fa4814f4524ff40b989b30713165e0ac5674b6ab1a48ac4e290aca0386\",\n      \"0x86524d7366db1e444004400477746935ef6edabf634f0d7118b7ab263062eeeb\",\n      \"0xa1a82ba3fddd0be9a34e8bc6ecaf2b6018e2ec5b9e1711d267e713f1f1375d46\",\n      \"0xbb51eff82fb9b517c71fa5f9ee948a8056efb79b5aa4c25815193527ee69ea0e\",\n      \"0xc6114dd90d0bb33b915527109ab4b2eb7ecef37d78e36877d15839d6814790e8\"\n    ],\n    \"inclusionMerkleProof\": \"0x0e5751c026e543b2e8ab2eb06099daa1d1e5df47778f7787faab45cdf12fe3a8\",\n    \"nextPoWScore\": 0,\n    \"nextPoWScoreMilestoneIndex\": 0,\n    \"signatures\": [\n      {\n        \"type\": 0,\n        \"publicKey\": \"0xed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c\",\n        \"signature\": \"0x3b3ca9291d51abf25528f5f261c73c8ee7fbae43f5ea0ea53bb9208e3c5c2f319197a991d675d02dcfd2336dd5ee02796fcf2ba62f819a42089ea7832a6e4c0f\"\n      },\n      {\n        \"type\": 0,\n        \"publicKey\": \"0xf6752f5f46a53364e2ee9c4d662d762a81efd51010282a75cd6bd03f28ef349c\",\n        \"signature\": \"0x4cf81b35c398fb82dddeedb8d1fce2466efbec990a0be522c134817970abf80046ef8d874a9a281798ea7bcf1f759d73d630a2bd6c975ff9e9c876a520d64a02\"\n      }\n    ]\n  },\n  \"nonce\": \"15860\"\n}"
	milestoneMessageHex := "02062a2133f1b898dfdbd27e21db98f588eb661e671487121f273fe496ddc227df1f6382e7fa4814f4524ff40b989b30713165e0ac5674b6ab1a48ac4e290aca038686524d7366db1e444004400477746935ef6edabf634f0d7118b7ab263062eeeba1a82ba3fddd0be9a34e8bc6ecaf2b6018e2ec5b9e1711d267e713f1f1375d46bb51eff82fb9b517c71fa5f9ee948a8056efb79b5aa4c25815193527ee69ea0ec6114dd90d0bb33b915527109ab4b2eb7ecef37d78e36877d15839d6814790e8c00100000700000003000000a26a446200000000062a2133f1b898dfdbd27e21db98f588eb661e671487121f273fe496ddc227df1f6382e7fa4814f4524ff40b989b30713165e0ac5674b6ab1a48ac4e290aca038686524d7366db1e444004400477746935ef6edabf634f0d7118b7ab263062eeeba1a82ba3fddd0be9a34e8bc6ecaf2b6018e2ec5b9e1711d267e713f1f1375d46bb51eff82fb9b517c71fa5f9ee948a8056efb79b5aa4c25815193527ee69ea0ec6114dd90d0bb33b915527109ab4b2eb7ecef37d78e36877d15839d6814790e80e5751c026e543b2e8ab2eb06099daa1d1e5df47778f7787faab45cdf12fe3a80000000000000000000000000200ed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c3b3ca9291d51abf25528f5f261c73c8ee7fbae43f5ea0ea53bb9208e3c5c2f319197a991d675d02dcfd2336dd5ee02796fcf2ba62f819a42089ea7832a6e4c0f00f6752f5f46a53364e2ee9c4d662d762a81efd51010282a75cd6bd03f28ef349c4cf81b35c398fb82dddeedb8d1fce2466efbec990a0be522c134817970abf80046ef8d874a9a281798ea7bcf1f759d73d630a2bd6c975ff9e9c876a520d64a02f43d000000000000"
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
