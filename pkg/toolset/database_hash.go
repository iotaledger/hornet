package toolset

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"

	coreDatabase "github.com/gohornet/hornet/core/database"
	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/iotaledger/hive.go/configuration"
	iotago "github.com/iotaledger/iota.go/v3"
)

func calculateDatabaseLedgerHash(dbStorage *storage.Storage, outputJSON bool) error {

	correctVersion, err := dbStorage.CheckCorrectDatabasesVersion()
	if err != nil {
		return err
	}

	if !correctVersion {
		return fmt.Errorf("database version outdated")
	}

	corrupted, err := dbStorage.AreDatabasesCorrupted()
	if err != nil {
		return err
	}

	tainted, err := dbStorage.AreDatabasesTainted()
	if err != nil {
		return err
	}

	ts := time.Now()

	if !outputJSON {
		fmt.Println("calculating ledger state hash...")
	}

	ledgerIndex, err := dbStorage.UTXOManager().ReadLedgerIndex()
	if err != nil {
		return err
	}

	snapshotInfo := dbStorage.SnapshotInfo()
	if snapshotInfo == nil {
		return errors.New("no snapshot info found")
	}

	// compute the sha256 of the ledger state
	lsHash := sha256.New()

	// write current ledger index
	if err := binary.Write(lsHash, binary.LittleEndian, ledgerIndex); err != nil {
		return fmt.Errorf("unable to serialize ledger index: %w", err)
	}

	// read out treasury tx
	treasuryOutput, err := dbStorage.UTXOManager().UnspentTreasuryOutputWithoutLocking()
	if err != nil {
		return err
	}

	if treasuryOutput != nil {
		// write current treasury output
		if _, err := lsHash.Write(treasuryOutput.MilestoneID[:]); err != nil {
			return fmt.Errorf("unable to serialize treasury output milestone hash: %w", err)
		}
		if err := binary.Write(lsHash, binary.LittleEndian, treasuryOutput.Amount); err != nil {
			return fmt.Errorf("unable to serialize treasury output amount: %w", err)
		}
	}

	// get all UTXOs and sort them by outputID
	var outputIDs utxo.LexicalOrderedOutputIDs
	outputIDs, err = dbStorage.UTXOManager().UnspentOutputsIDs()
	if err != nil {
		return err
	}

	// sort the OutputIDs lexicographically by their ID
	sort.Sort(outputIDs)

	// write all unspent outputs in lexicographical order
	for _, outputID := range outputIDs {
		output, err := dbStorage.UTXOManager().ReadOutputByOutputID(outputID)
		if err != nil {
			return err
		}

		outputBytes := output.SnapshotBytes()
		if err = binary.Write(lsHash, binary.LittleEndian, outputBytes); err != nil {
			return err
		}
	}

	// calculate sha256 hash of the current ledger state
	snapshotHashSumWithoutSEPs := lsHash.Sum(nil)

	var solidEntryPoints iotago.BlockIDs
	dbStorage.ForEachSolidEntryPointWithoutLocking(func(sep *storage.SolidEntryPoint) bool {
		solidEntryPoints = append(solidEntryPoints, sep.BlockID)
		return true
	})

	// write all solid entry points in lexicographical order
	for _, solidEntryPoint := range solidEntryPoints.RemoveDupsAndSort() {
		sepBytes, err := solidEntryPoint.MarshalBinary()
		if err != nil {
			return fmt.Errorf("unable to serialize solid entry point %s: %w", solidEntryPoint.ToHex(), err)
		}

		if err := binary.Write(lsHash, binary.LittleEndian, sepBytes); err != nil {
			return fmt.Errorf("unable to calculate snapshot hash: %w", err)
		}
	}

	snapshotHashSumWithSEPs := lsHash.Sum(nil)

	if outputJSON {

		type treasuryStruct struct {
			MilestoneID string `json:"milestoneID"`
			Tokens      uint64 `json:"tokens"`
		}

		var treasury *treasuryStruct
		if treasuryOutput != nil {
			treasury = &treasuryStruct{
				MilestoneID: iotago.EncodeHex(treasuryOutput.MilestoneID[:]),
				Tokens:      treasuryOutput.Amount,
			}
		}

		result := struct {
			Healthy                bool            `json:"healthy"`
			Tainted                bool            `json:"tainted"`
			SnapshotTime           time.Time       `json:"snapshotTime"`
			NetworkID              uint64          `json:"networkID"`
			Treasury               *treasuryStruct `json:"treasury"`
			LedgerIndex            milestone.Index `json:"ledgerIndex"`
			SnapshotIndex          milestone.Index `json:"snapshotIndex"`
			UTXOsCount             int             `json:"UTXOsCount"`
			SEPsCount              int             `json:"SEPsCount"`
			LedgerStateHash        string          `json:"ledgerStateHash"`
			LedgerStateHashWithSEP string          `json:"ledgerStateHashWithSEP"`
		}{
			Healthy:                !corrupted,
			Tainted:                tainted,
			SnapshotTime:           snapshotInfo.Timestamp,
			NetworkID:              snapshotInfo.NetworkID,
			Treasury:               treasury,
			LedgerIndex:            ledgerIndex,
			SnapshotIndex:          snapshotInfo.SnapshotIndex,
			UTXOsCount:             len(outputIDs),
			SEPsCount:              len(solidEntryPoints),
			LedgerStateHash:        hex.EncodeToString(snapshotHashSumWithoutSEPs),
			LedgerStateHashWithSEP: hex.EncodeToString(snapshotHashSumWithSEPs),
		}

		return printJSON(result)
	}

	fmt.Printf(`    >
        - Healthy:        %s
        - Tainted:        %s
        - Snapshot time:  %v
        - Network ID:     %d
        - Treasury:       %s
        - Ledger index:   %d
        - Snapshot index: %d
        - UTXOs count:    %d
        - SEPs count:     %d
        - Ledger state hash (w/o  solid entry points): %s
        - Ledger state hash (with solid entry points): %s`+"\n\n",
		yesOrNo(!corrupted),
		yesOrNo(tainted),
		snapshotInfo.Timestamp,
		snapshotInfo.NetworkID,
		func() string {
			if treasuryOutput == nil {
				return "no treasury output found"
			}
			return fmt.Sprintf("milestone ID %s, tokens %d", iotago.EncodeHex(treasuryOutput.MilestoneID[:]), treasuryOutput.Amount)
		}(),
		ledgerIndex,
		snapshotInfo.SnapshotIndex,
		len(outputIDs),
		len(solidEntryPoints),
		hex.EncodeToString(snapshotHashSumWithoutSEPs),
		hex.EncodeToString(snapshotHashSumWithSEPs),
	)

	fmt.Printf("successfully calculated ledger state hash, took %v\n", time.Since(ts).Truncate(time.Millisecond))

	return nil
}

func databaseLedgerHash(args []string) error {

	fs := configuration.NewUnsortedFlagSet("", flag.ContinueOnError)
	databasePathFlag := fs.String(FlagToolDatabasePath, DefaultValueMainnetDatabasePath, "the path to the database")
	outputJSONFlag := fs.Bool(FlagToolOutputJSON, false, FlagToolDescriptionOutputJSON)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolDatabaseLedgerHash)
		fs.PrintDefaults()
		println(fmt.Sprintf("\nexample: %s --%s %s",
			ToolDatabaseLedgerHash,
			FlagToolDatabasePath,
			DefaultValueMainnetDatabasePath))
	}

	if err := parseFlagSet(fs, args); err != nil {
		return err
	}

	if len(*databasePathFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolDatabasePath)
	}

	databasePath := *databasePathFlag
	if _, err := os.Stat(databasePath); err != nil || os.IsNotExist(err) {
		return fmt.Errorf("'%s' (%s) does not exist", FlagToolDatabasePath, databasePath)
	}

	tangleStore, err := database.StoreWithDefaultSettings(filepath.Join(databasePath, coreDatabase.TangleDatabaseDirectoryName), false)
	if err != nil {
		return fmt.Errorf("%s database initialization failed: %w", coreDatabase.TangleDatabaseDirectoryName, err)
	}

	// clean up store
	defer func() {
		_ = tangleStore.Close()
	}()

	utxoStore, err := database.StoreWithDefaultSettings(filepath.Join(databasePath, coreDatabase.UTXODatabaseDirectoryName), false)
	if err != nil {
		return fmt.Errorf("%s database initialization failed: %w", coreDatabase.UTXODatabaseDirectoryName, err)
	}

	// clean up store
	defer func() {
		_ = utxoStore.Close()
	}()

	dbStorage, err := storage.New(tangleStore, utxoStore)
	if err != nil {
		return err
	}

	return calculateDatabaseLedgerHash(dbStorage, *outputJSONFlag)
}
