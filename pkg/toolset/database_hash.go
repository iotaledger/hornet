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

	coreDatabase "github.com/gohornet/hornet/core/database"
	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/snapshot"
	"github.com/iotaledger/hive.go/configuration"
)

func calculateDatabaseLedgerHash(dbStorage *storage.Storage) error {

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
	fmt.Println("calculating ledger state hash...")

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
		outputs = append(outputs, &snapshot.Output{MessageID: output.MessageID().ToArray(), OutputID: *output.OutputID(), OutputType: output.OutputType(), Address: output.Address(), Amount: output.Amount()})
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
		outputBytes, err := output.MarshalBinary()
		if err != nil {
			return fmt.Errorf("unable to serialize output %s: %w", hex.EncodeToString(output.OutputID[:]), err)
		}

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

	yesOrNo := func(value bool) string {
		if value {
			return "YES"
		}
		return "NO"
	}

	fmt.Printf(`>
	- Healthy %s
	- Tainted %s
	- Snapshot time %v
	- Network ID %d
	- Treasury %s
	- Ledger index %d
	- Snapshot index %d
	- UTXOs count %d
	- SEPs count %d
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
	printUsage := func() {
		println("Usage:")
		println(fmt.Sprintf("	%s [DATABASE_PATH]", ToolDatabaseLedgerHash))
		println()
		println("   [DATABASE_PATH] - the path to the database")
		println()
		println(fmt.Sprintf("example: %s %s", ToolDatabaseLedgerHash, "mainnetdb"))
	}

	if len(args) > 1 {
		printUsage()
		return fmt.Errorf("wrong argument count for '%s'", ToolDatabaseLedgerHash)
	}

	// check arguments
	if len(args) == 0 {
		printUsage()
		return errors.New("DATABASE_PATH is missing")
	}

	databasePath := args[0]
	if _, err := os.Stat(databasePath); err != nil || os.IsNotExist(err) {
		return fmt.Errorf("DATABASE_PATH (%s) does not exist", databasePath)
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

	return calculateDatabaseLedgerHash(dbStorage)
}
