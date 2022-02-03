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
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/snapshot"
	"github.com/iotaledger/hive.go/configuration"
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

	// read out treasury tx
	treasuryOutput, err := dbStorage.UTXOManager().UnspentTreasuryOutputWithoutLocking()
	if err != nil {
		return err
	}

	var outputs snapshot.LexicalOrderedOutputs
	if err := dbStorage.UTXOManager().ForEachUnspentOutput(func(output *utxo.Output) bool {
		outputs = append(outputs, output)
		return true
	}); err != nil {
		return err
	}
	// sort the outputs lexicographically by their OutputID
	sort.Sort(outputs)

	var solidEntryPoints hornet.LexicalOrderedMessageIDs
	dbStorage.ForEachSolidEntryPointWithoutLocking(func(sep *storage.SolidEntryPoint) bool {
		solidEntryPoints = append(solidEntryPoints, sep.MessageID)
		return true
	})
	// sort the solid entry points lexicographically by their MessageID
	sort.Sort(solidEntryPoints)

	// compute the sha256 of the ledger state
	lsHash := sha256.New()

	// write current ledger index
	if err := binary.Write(lsHash, binary.LittleEndian, ledgerIndex); err != nil {
		return fmt.Errorf("unable to serialize ledger index: %w", err)
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

	// write all unspent outputs in lexicographical order
	for _, output := range outputs {
		outputBytes := output.SnapshotBytes()
		if err = binary.Write(lsHash, binary.LittleEndian, outputBytes); err != nil {
			return fmt.Errorf("unable to calculate snapshot hash: %w", err)
		}
	}

	// calculate sha256 hash of the current ledger state
	snapshotHashSumWithoutSEPs := lsHash.Sum(nil)

	// write all solid entry points in lexicographical order
	for _, solidEntryPoint := range solidEntryPoints {
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
				MilestoneID: hex.EncodeToString(treasuryOutput.MilestoneID[:]),
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
			UTXOsCount:             len(outputs),
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
			return fmt.Sprintf("milestone ID %s, tokens %d", hex.EncodeToString(treasuryOutput.MilestoneID[:]), treasuryOutput.Amount)
		}(),
		ledgerIndex,
		snapshotInfo.SnapshotIndex,
		len(outputs),
		len(solidEntryPoints),
		hex.EncodeToString(snapshotHashSumWithoutSEPs),
		hex.EncodeToString(snapshotHashSumWithSEPs),
	)

	fmt.Printf("successfully calculated ledger state hash, took %v\n", time.Since(ts).Truncate(time.Millisecond))

	return nil
}

func databaseLedgerHash(_ *configuration.Configuration, args []string) error {
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	databasePathFlag := fs.String(FlagToolDatabasePath, "mainnetdb", "the path to the database")
	outputJSONFlag := fs.Bool(FlagToolOutputJSON, false, FlagToolDescriptionOutputJSON)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolDatabaseLedgerHash)
		fs.PrintDefaults()
		println(fmt.Sprintf("\nexample: %s --%s %s",
			ToolDatabaseLedgerHash,
			FlagToolDatabasePath,
			"mainnetdb"))
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
		tangleStore.Shutdown()
		_ = tangleStore.Close()
	}()

	utxoStore, err := database.StoreWithDefaultSettings(filepath.Join(databasePath, coreDatabase.UTXODatabaseDirectoryName), false)
	if err != nil {
		return fmt.Errorf("%s database initialization failed: %w", coreDatabase.UTXODatabaseDirectoryName, err)
	}

	// clean up store
	defer func() {
		utxoStore.Shutdown()
		_ = utxoStore.Close()
	}()

	dbStorage, err := storage.New(tangleStore, utxoStore)
	if err != nil {
		return err
	}

	return calculateDatabaseLedgerHash(dbStorage, *outputJSONFlag)
}
