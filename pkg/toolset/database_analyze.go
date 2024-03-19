package toolset

import (
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/common/hexutil"
	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/app/configuration"
	hivedb "github.com/iotaledger/hive.go/kvstore/database"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	iotago "github.com/iotaledger/iota.go/v3"
)

func databaseAnalyze(args []string) error {
	fs := configuration.NewUnsortedFlagSet("", flag.ContinueOnError)
	databasePathFlag := fs.String(FlagToolDatabasePath, "", "the path to the database")

	fs.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolDatabaseVerify)
		fs.PrintDefaults()
		println(fmt.Sprintf("\nexample: %s --%s %s --%s %s",
			ToolDatabaseVerify,
			FlagToolDatabasePath,
			DefaultValueMainnetDatabasePath))
	}

	if err := parseFlagSet(fs, args); err != nil {
		return err
	}

	if len(*databasePathFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolDatabasePath)
	}

	tangleStore, err := getTangleStorage(*databasePathFlag, "source", string(hivedb.EngineAuto), true, false, false, true)
	if err != nil {
		return err
	}
	defer func() {
		println("\nshutdown source storage ...")
		if err := tangleStore.Shutdown(); err != nil {
			panic(err)
		}
	}()

	protoParams, err := tangleStore.CurrentProtocolParameters()
	if err != nil {
		panic(err)
	}

	if err := tangleStore.UTXOManager().ForEachUnspentOutput(func(output *utxo.Output) bool {

		fmt.Println("outputID:", output.OutputID().ToHex())
		fmt.Println("outputType:", output.Output().Type().String())
		fmt.Println("amount:", output.Output().Deposit())

		if nativeTokens := output.Output().NativeTokenList(); len(nativeTokens) > 0 {
			fmt.Println("nativeTokens:")
			for _, nativeToken := range nativeTokens {
				fmt.Println(">", "ID:", nativeToken.ID.ToHex(), "amount:", nativeToken.Amount)
			}
		}

		fmt.Println("unlocks:")
		unlocks := output.Output().UnlockConditionSet()
		if address := unlocks.Address(); address != nil {
			fmt.Println(">", "address:", address.Address.Bech32(protoParams.Bech32HRP))
		}
		if sdruc := unlocks.StorageDepositReturn(); sdruc != nil {
			fmt.Println(">", "storageDepositReturn:", sdruc.ReturnAddress.Bech32(protoParams.Bech32HRP), sdruc.Amount)
		}
		if timelock := unlocks.Timelock(); timelock != nil {
			fmt.Println(">", "timelock:", timelock.UnixTime)
		}
		if expiration := unlocks.Expiration(); expiration != nil {
			fmt.Println(">", "expiration:", expiration.UnixTime)
		}
		if stateController := unlocks.StateControllerAddress(); stateController != nil {
			fmt.Println(">", "stateController:", stateController.Address.Bech32(protoParams.Bech32HRP))
		}
		if governor := unlocks.GovernorAddress(); governor != nil {
			fmt.Println(">", "governor:", governor.Address.Bech32(protoParams.Bech32HRP))
		}

		features := output.Output().FeatureSet()
		if len(features) > 0 {
			fmt.Println("features:")
			if sender := features.SenderFeature(); sender != nil {
				fmt.Println(">", "sender:", sender.Address.Bech32(protoParams.Bech32HRP))
			}
			if issuer := features.IssuerFeature(); issuer != nil {
				fmt.Println(">", "issuer:", issuer.Address.Bech32(protoParams.Bech32HRP))
			}
			if tag := features.TagFeature(); tag != nil {
				fmt.Println(">", "tag:", hexutil.Encode(tag.Tag))
			}
			if metadata := features.MetadataFeature(); metadata != nil {
				fmt.Println(">", "metadata:", hexutil.Encode(metadata.Data))
			}
		}

		if chainOutput, isChainOutput := output.Output().(iotago.ChainConstrainedOutput); isChainOutput {
			immFeatures := chainOutput.ImmutableFeatureSet()
			if len(immFeatures) > 0 {
				if immutableAlias := unlocks.ImmutableAlias(); immutableAlias != nil {
					fmt.Println(">", "immutableAlias:", immutableAlias.Address.Bech32(protoParams.Bech32HRP))
				}
				if issuer := features.IssuerFeature(); issuer != nil {
					fmt.Println(">", "issuer:", issuer.Address.Bech32(protoParams.Bech32HRP))
				}
				if metadata := features.MetadataFeature(); metadata != nil {
					fmt.Println(">", "metadata:", hexutil.Encode(metadata.Data))
				}
			}
		}

		fmt.Println("----------------------------------------------------------------------------------------------------")

		return true
	}); err != nil {
		panic(err)
	}

	return nil
}
