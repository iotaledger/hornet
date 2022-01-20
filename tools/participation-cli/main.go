package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/participation"
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	TAG_PARTICIPATION = "PARTICIPATE"
)

var (
	// TODO: Use final values
	deSeriParas = &iotago.DeSerializationParameters{
		RentStructure: &iotago.RentStructure{
			VByteCost:    0,
			VBFactorData: 1,
			VBFactorKey:  10,
		},
	}
)

type cfg struct {
	nodeAPIAddress  string
	inputPrivateKey ed25519.PrivateKey
	payloadFilePath string
}

func parseParticipationPayload(cfg *cfg) ([]byte, error) {

	participationPayload := &participation.ParticipationPayload{}
	if err := utils.ReadJSONFromFile(cfg.payloadFilePath, participationPayload); err != nil {
		return nil, err
	}

	return participationPayload.Serialize(serializer.DeSeriModePerformValidation, nil)
}

func buildTransactionPayload(ctx context.Context, client *iotago.NodeHTTPAPIClient, inputAddress *iotago.Ed25519Address, inputSigner iotago.AddressSigner, outputAddress *iotago.Ed25519Address, outputAmount uint64, taggedData *iotago.TaggedData) (*iotago.Transaction, error) {

	minDustDeposit := deSeriParas.RentStructure.MinDustDeposit(inputAddress)

	if outputAmount < minDustDeposit {
		return nil, fmt.Errorf("AMOUNT does not fulfill the dust requirement: %d, needed: %d", outputAmount, minDustDeposit)
	}

	txBuilder := iotago.NewTransactionBuilder()

	unspentOutputs, err := client.OutputIDsByEd25519Address(ctx, inputAddress, false)
	if err != nil {
		return nil, err
	}

	inputsBalance := uint64(0)
	for _, outputIDHex := range unspentOutputs.OutputIDs {
		input, err := outputIDHex.AsUTXOInput()
		if err != nil {
			return nil, err
		}

		output, err := client.OutputByID(ctx, input.ID())
		if err != nil {
			return nil, err
		}

		unspentOutput, err := output.Output()
		if err != nil {
			return nil, err
		}

		if unspentOutput.Type() != iotago.OutputExtended {
			continue
		}

		inputsBalance += unspentOutput.Deposit()
		txBuilder.AddInput(&iotago.ToBeSignedUTXOInput{Address: inputAddress, Input: input})

		if inputsBalance >= (outputAmount + minDustDeposit) {
			// no need to collect further inputs
			break
		}
	}

	if inputsBalance < outputAmount {
		return nil, fmt.Errorf("not enough balance on the inputs: %d, needed: %d", inputsBalance, outputAmount)
	}

	txBuilder.AddOutput(&iotago.ExtendedOutput{
		Amount: outputAmount,
		Conditions: iotago.UnlockConditions{
			&iotago.AddressUnlockCondition{Address: outputAddress},
		},
	})
	inputsBalance -= outputAmount

	if inputsBalance != 0 && inputsBalance < minDustDeposit {
		return nil, fmt.Errorf("remainder does not fulfill the minimum balance requirement: %d, needed: %d", inputsBalance, minDustDeposit)
	}

	if inputsBalance > 0 {
		txBuilder.AddOutput(&iotago.ExtendedOutput{
			Amount: inputsBalance,
			Conditions: iotago.UnlockConditions{
				&iotago.AddressUnlockCondition{Address: inputAddress},
			},
		})
	}

	if taggedData != nil {
		txBuilder.AddTaggedDataPayload(taggedData)
	}

	return txBuilder.Build(deSeriParas, inputSigner)
}

func sendParticipationTransaction(cfg *cfg) (*iotago.MessageID, error) {

	client := iotago.NewNodeHTTPAPIClient(cfg.nodeAPIAddress, deSeriParas)

	inputPublicKey := cfg.inputPrivateKey.Public().(ed25519.PublicKey)
	inputAddress := iotago.Ed25519AddressFromPubKey(inputPublicKey)
	inputSigner := iotago.NewInMemoryAddressSigner(iotago.NewAddressKeysForEd25519Address(&inputAddress, cfg.inputPrivateKey))

	participationPayload, err := parseParticipationPayload(cfg)
	if err != nil {
		return nil, err
	}

	clientCtx, clientCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer clientCancel()

	balanceResponse, err := client.BalanceByEd25519Address(clientCtx, &inputAddress)
	if err != nil {
		return nil, err
	}

	txPayload, err := buildTransactionPayload(clientCtx, client, &inputAddress, inputSigner, &inputAddress, balanceResponse.Balance, &iotago.TaggedData{
		Tag:  []byte(TAG_PARTICIPATION),
		Data: participationPayload,
	})
	if err != nil {
		return nil, err
	}

	nodeInfo, err := client.Info(clientCtx)
	if err != nil {
		return nil, err
	}

	remotePoWEnabled := false
	for _, feature := range nodeInfo.Features {
		if feature == "PoW" {
			remotePoWEnabled = true
			break
		}
	}

	msg := &iotago.Message{
		Payload: txPayload,
	}

	if !remotePoWEnabled {
		// do local PoW
		powManager := pow.New(nodeInfo.MinPowScore, 5*time.Second)

		getTipsFunc := func() (hornet.MessageIDs, error) {
			tipsResponse, err := client.Tips(clientCtx)
			if err != nil {
				return nil, err
			}

			tips, err := tipsResponse.Tips()
			return hornet.MessageIDsFromSliceOfArrays(tips), err
		}

		tips, err := getTipsFunc()
		if err != nil {
			return nil, err
		}

		msg.Parents = tips.ToSliceOfArrays()
		msg.NetworkID = iotago.NetworkIDFromString(nodeInfo.NetworkID)

		if err := powManager.DoPoW(clientCtx, msg, 0, getTipsFunc); err != nil {
			return nil, err
		}
	}

	msg, err = client.SubmitMessage(clientCtx, msg)
	if err != nil {
		return nil, err
	}

	msgID, err := msg.ID()
	if err != nil {
		return nil, err
	}

	return msgID, nil
}

func parseCfgFromArgs() (*cfg, error) {

	cmd := os.Args[0]
	args := os.Args[1:]

	printUsage := func() {
		println("Usage:")
		println(fmt.Sprintf("   %s [NODE_API_ADDRESS] [SENDER_PRIVATE_KEY] [PAYLOAD_FILE_PATH]", cmd))
		println()
		println("   [NODE_API_ADDRESS]   - the API address of the node to use")
		println("   [SENDER_PRIVATE_KEY] - the private key of the sender")
		println("   [PAYLOAD_FILE_PATH]  - path of the participation payload json file to send")
		println()
	}

	if len(args) != 3 {
		printUsage()
		return nil, fmt.Errorf("wrong argument count for '%s'", cmd)
	}

	nodeAPIAddress := args[0]

	inputPrivateKey, err := utils.ParseEd25519PrivateKeyFromString(args[1])
	if err != nil {
		return nil, fmt.Errorf("can't parse SENDER_PRIVATE_KEY: %w", err)
	}

	payloadFilePath := args[2]

	return &cfg{
		nodeAPIAddress:  nodeAPIAddress,
		inputPrivateKey: inputPrivateKey,
		payloadFilePath: payloadFilePath,
	}, nil
}

func main() {

	cfg, err := parseCfgFromArgs()
	if err != nil {
		panic(err)
	}

	msgID, err := sendParticipationTransaction(cfg)
	if err != nil {
		panic(err)
	}

	println(fmt.Sprintf("Participation transaction sent: MsgID: %s", hex.EncodeToString(msgID[:])))
}
