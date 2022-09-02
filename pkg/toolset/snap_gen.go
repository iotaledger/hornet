package toolset

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/core/configuration"
	"github.com/iotaledger/hive.go/core/ioutils"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/hornet/v2/pkg/snapshot"
	iotago "github.com/iotaledger/iota.go/v3"
)

type AddressWithBalance struct {
	Address iotago.Address
	Balance uint64
}

type jsonGenesisAddresses struct {
	Balances map[string]uint64 `json:"balances"`
	Rewards  map[string]uint64 `json:"rewards"`
}

type GenesisAddresses struct {
	Balances []*AddressWithBalance
}

func (g *GenesisAddresses) MarshalJSON() ([]byte, error) {
	return nil, nil
}

func (g *GenesisAddresses) UnmarshalJSON(bytes []byte) error {
	j := &jsonGenesisAddresses{}
	if err := json.Unmarshal(bytes, j); err != nil {
		return err
	}

	if len(j.Balances) != 0 && len(j.Rewards) != 0 {
		return errors.New("cannot specify both balances and rewards")
	}

	parseBalance := func(bech32Address string, balance uint64) error {
		address, err := parseAddress(bech32Address)
		if err != nil {
			return err
		}

		g.Balances = append(g.Balances, &AddressWithBalance{
			Address: address,
			Balance: balance,
		})

		return nil
	}

	for bech32Address, balance := range j.Balances {
		if err := parseBalance(bech32Address, balance); err != nil {
			return err
		}
	}

	for bech32Address, balance := range j.Rewards {
		if err := parseBalance(bech32Address, balance); err != nil {
			return err
		}
	}

	return nil
}

// Sort sorts the addresses based on the address and balance in case they are equal.
func (g *GenesisAddresses) Sort() {
	sort.Slice(g.Balances, func(i int, j int) bool {
		addressI := g.Balances[i].Address
		addressJ := g.Balances[j].Address

		addressBytesI, err := addressI.Serialize(serializer.DeSeriModeNoValidation, nil)
		if err != nil {
			panic(fmt.Sprintf("serializing address %s failed: %s", addressI.String(), err.Error()))
		}

		addressBytesJ, err := addressJ.Serialize(serializer.DeSeriModeNoValidation, nil)
		if err != nil {
			panic(fmt.Sprintf("serializing address %s failed: %s", addressJ.String(), err.Error()))
		}

		result := bytes.Compare(addressBytesI, addressBytesJ)
		if result == 0 {
			balanceI := g.Balances[i].Balance
			balanceJ := g.Balances[j].Balance

			// if both addresses are equal, we sort by larger balance first
			return balanceI > balanceJ
		}

		return result < 0
	})
}

// TotalBalance calculates the total balance of all genesis addresses.
func (g *GenesisAddresses) TotalBalance() uint64 {
	total := uint64(0)
	for _, genesisAddress := range g.Balances {
		total += genesisAddress.Balance
	}

	return total
}

func parseAddress(bech32Address string) (iotago.Address, error) {
	_, address, err := iotago.ParseBech32(bech32Address)
	if err != nil {
		bech32Address = strings.TrimPrefix(bech32Address, "0x")

		if len(bech32Address) != iotago.Ed25519AddressBytesLength*2 {
			return nil, err
		}

		// try parsing as hex
		address, err = iotago.ParseEd25519AddressFromHexString("0x" + bech32Address)
		if err != nil {
			return nil, err
		}
	}

	return address, nil
}

func snapshotGen(args []string) error {

	fs := configuration.NewUnsortedFlagSet("", flag.ContinueOnError)
	protocolParametersPathFlag := fs.String(FlagToolProtocolParametersPath, "", "the path to the initial protocol parameters file")
	mintAddressFlag := fs.String(FlagToolSnapGenMintAddress, "", "the initial bech32 address all the tokens will be minted to")
	treasuryAllocationFlag := fs.Uint64(FlagToolSnapGenTreasuryAllocation, 0, "the amount of tokens to reside within the treasury, the delta from the supply will be allocated to 'mintAddress'")
	genesisAddressesPathFlag := fs.String(FlagToolGenesisAddressesPath, "", "the file path to the genesis bech32 addresses file (optional)")
	genesisAddressesFlag := fs.String(FlagToolGenesisAddresses, "", "additional genesis bech32 addresses with balances (optional, format: addr1:balance1,addr2:balance2,...)")
	outputFilePathFlag := fs.String(FlagToolOutputPath, "", "the file path to the generated snapshot file")

	fs.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolSnapGen)
		fs.PrintDefaults()
		println(fmt.Sprintf("\nexample: %s --%s %s --%s %s --%s %s --%s %s",
			ToolSnapGen,
			FlagToolProtocolParametersPath,
			"protocol_parameters.json",
			FlagToolSnapGenMintAddress,
			"[MINT_ADDRESS]",
			FlagToolSnapGenTreasuryAllocation,
			"500000000",
			FlagToolOutputPath,
			"genesis_snapshot.bin"))
	}

	if err := parseFlagSet(fs, args); err != nil {
		return err
	}

	if len(*protocolParametersPathFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolProtocolParametersPath)
	}
	if len(*outputFilePathFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolOutputPath)
	}

	protocolParametersPath := *protocolParametersPathFlag
	if _, err := os.Stat(protocolParametersPath); err != nil || os.IsNotExist(err) {
		return fmt.Errorf("'%s' (%s) does not exist", FlagToolProtocolParametersPath, protocolParametersPath)
	}

	outputFilePath := *outputFilePathFlag
	if _, err := os.Stat(outputFilePath); err == nil || !os.IsNotExist(err) {
		return fmt.Errorf("'%s' already exists", FlagToolOutputPath)
	}

	println("loading protocol parameters ...")
	// TODO: needs to be adapted for when protocol parameters struct changes
	protoParams := &iotago.ProtocolParameters{}
	if err := ioutils.ReadJSONFromFile(protocolParametersPath, protoParams); err != nil {
		return fmt.Errorf("failed to load protocol parameters: %w", err)
	}

	protoParamsBytes, err := protoParams.Serialize(serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		return fmt.Errorf("failed to serialize protocol parameters: %w", err)
	}

	treasury := *treasuryAllocationFlag

	genesisAddresses := &GenesisAddresses{}
	if len(*genesisAddressesPathFlag) > 0 {
		genesisAddressesPath := *genesisAddressesPathFlag
		if _, err := os.Stat(genesisAddressesPath); err != nil || os.IsNotExist(err) {
			return fmt.Errorf("'%s' (%s) does not exist", FlagToolGenesisAddressesPath, genesisAddressesPath)
		}

		println("loading genesis addresses from file ...")
		if err := ioutils.ReadJSONFromFile(genesisAddressesPath, genesisAddresses); err != nil {
			return fmt.Errorf("failed to load genesis addresses: %w", err)
		}
	}
	if len(*genesisAddressesFlag) > 0 {
		println("loading genesis addresses from command line ...")

		addressesWithBalances := strings.Split(*genesisAddressesFlag, ",")
		for i, addressWithBalance := range addressesWithBalances {
			addressWithBalance := strings.Split(addressWithBalance, ":")
			if len(addressWithBalance) != 2 {
				return fmt.Errorf("'%s' invalid format for address at position %d: 'addr:balance' format not found", FlagToolGenesisAddresses, i)
			}

			address, err := parseAddress(addressWithBalance[0])
			if err != nil {
				return fmt.Errorf("'%s' invalid format for address at position %d: %w", FlagToolGenesisAddresses, i, err)
			}

			balance, err := strconv.ParseUint(addressWithBalance[1], 10, 64)
			if err != nil {
				return fmt.Errorf("'%s' invalid format for balance at position %d: %w", FlagToolGenesisAddresses, i, err)
			}

			genesisAddresses.Balances = append(genesisAddresses.Balances, &AddressWithBalance{
				Address: address,
				Balance: balance,
			})
		}
	}

	// sort the addresses to have a deterministic order
	genesisAddresses.Sort()

	// create snapshot file
	var targetIndex iotago.MilestoneIndex
	fullHeader := &snapshot.FullSnapshotHeader{
		Version:                  snapshot.SupportedFormatVersion,
		Type:                     snapshot.Full,
		GenesisMilestoneIndex:    0,
		TargetMilestoneIndex:     targetIndex,
		TargetMilestoneTimestamp: 0,
		TargetMilestoneID:        iotago.MilestoneID{},
		LedgerMilestoneIndex:     targetIndex,
		TreasuryOutput: &utxo.TreasuryOutput{
			MilestoneID: iotago.MilestoneID{},
			Amount:      treasury,
		},
		ProtocolParamsMilestoneOpt: &iotago.ProtocolParamsMilestoneOpt{
			TargetMilestoneIndex: targetIndex,
			ProtocolVersion:      protoParams.Version,
			Params:               protoParamsBytes,
		},
		OutputCount:        0,
		MilestoneDiffCount: 0,
		SEPCount:           0,
	}

	// solid entry points
	// add "EmptyBlockID" as sole entry point
	nullHashAdded := false
	solidEntryPointProducerFunc := func() (iotago.BlockID, error) {
		if nullHashAdded {
			return iotago.EmptyBlockID(), snapshot.ErrNoMoreSEPToProduce
		}
		nullHashAdded = true

		return iotago.EmptyBlockID(), nil
	}

	genesisBalancesTotal := genesisAddresses.TotalBalance()

	// calculate the remaining amount for the "genesis mint address"
	balanceMintAddress := int64(protoParams.TokenSupply) - int64(treasury) - int64(genesisBalancesTotal)

	var mintAddress iotago.Address
	genesisOutputAdded := false
	switch {
	case balanceMintAddress < 0:
		return fmt.Errorf("not enough funds to create genesis snapshot")

	case balanceMintAddress == 0:
		// no genesis output needed, all balances distributed
		genesisOutputAdded = true

	default:
		// genesis mint address needs to be added
		if len(*mintAddressFlag) == 0 {
			return fmt.Errorf("'%s' not specified", FlagToolSnapGenMintAddress)
		}

		// check mint address
		mintAddress, err = parseAddress(*mintAddressFlag)
		if err != nil {
			return fmt.Errorf("failed to parse mint address: %w", err)
		}
	}

	// unspent transaction outputs
	genesisBalancesIndex := int64(0)
	outputProducerFunc := func() (*utxo.Output, error) {
		if !genesisOutputAdded {
			genesisOutputAdded = true

			// add the genesis output with the remaining balance
			return utxo.CreateOutput(
				iotago.OutputID{},
				iotago.EmptyBlockID(),
				0,
				0,
				&iotago.BasicOutput{
					Amount: uint64(balanceMintAddress),
					Conditions: iotago.UnlockConditions{
						&iotago.AddressUnlockCondition{Address: mintAddress},
					},
				}), nil
		}

		if genesisBalancesIndex < int64(len(genesisAddresses.Balances)) {
			genesisAddress := genesisAddresses.Balances[genesisBalancesIndex]
			genesisBalancesIndex++

			return utxo.CreateOutput(
				iotago.OutputIDFromTransactionIDAndIndex(TransactionIDFromIndex(genesisBalancesIndex), 0),
				iotago.EmptyBlockID(),
				0,
				0,
				&iotago.BasicOutput{
					Amount: genesisAddress.Balance,
					Conditions: iotago.UnlockConditions{
						&iotago.AddressUnlockCondition{Address: genesisAddress.Address},
					},
				}), nil
		}

		// all outputs added
		//nolint:nilnil // nil, nil is ok in this context, even if it is not go idiomatic
		return nil, nil
	}

	// milestone diffs
	milestoneDiffProducerFunc := func() (*snapshot.MilestoneDiff, error) {
		// no milestone diffs needed
		//nolint:nilnil // nil, nil is ok in this context, even if it is not go idiomatic
		return nil, nil
	}

	// build temp file path
	outputFilePathTmp := outputFilePath + "_tmp"

	// we don't need to check the error, maybe the file doesn't exist
	_ = os.Remove(outputFilePathTmp)

	fileHandle, err := os.OpenFile(outputFilePathTmp, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("unable to create snapshot file: %w", err)
	}

	if _, err := snapshot.StreamFullSnapshotDataTo(
		fileHandle,
		fullHeader,
		outputProducerFunc,
		milestoneDiffProducerFunc,
		solidEntryPointProducerFunc,
	); err != nil {
		_ = fileHandle.Close()

		return fmt.Errorf("couldn't generate snapshot file: %w", err)
	}

	if err := fileHandle.Close(); err != nil {
		return fmt.Errorf("unable to close snapshot file: %w", err)
	}

	// rename tmp file to final file name
	if err := os.Rename(outputFilePathTmp, outputFilePath); err != nil {
		return fmt.Errorf("unable to rename temp snapshot file: %w", err)
	}

	fmt.Println("Snapshot creation successful!")

	return nil
}

func TransactionIDFromIndex(index int64) iotago.TransactionID {
	txID := iotago.TransactionID{}

	indexBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(indexBytes, uint64(index))
	copy(txID[:8], indexBytes)

	return txID
}
