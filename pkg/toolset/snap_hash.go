package toolset

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"time"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/snapshot"
	"github.com/iotaledger/hive.go/configuration"
)

func snapshotHash(_ *configuration.Configuration, args []string) error {
	printUsage := func() {
		println("Usage:")
		println(fmt.Sprintf("	%s [FULL_SNAPSHOT_PATH] [DELTA_SNAPSHOT_PATH]", ToolSnapHash))
		println()
		println("	[FULL_SNAPSHOT_PATH]  - the path to the full snapshot file")
		println("	[DELTA_SNAPSHOT_PATH] - the path to the delta snapshot file (optional)")
		println()
		println(fmt.Sprintf("example: %s %s", ToolSnapHash, "./snapshot.bin"))
	}

	if len(args) > 2 {
		printUsage()
		return fmt.Errorf("wrong argument count for '%s'", ToolSnapHash)
	}

	// check arguments
	if len(args) == 0 {
		printUsage()
		return errors.New("FULL_SNAPSHOT_PATH missing")
	}

	fullPath := args[0]
	deltaPath := ""

	if len(args) == 2 {
		deltaPath = args[1]
	}

	targetEngine, err := database.DatabaseEngine(database.EnginePebble)
	if err != nil {
		return err
	}

	tempDir, err := ioutil.TempDir("", "snapHash")
	if err != nil {
		return fmt.Errorf("can't create temp dir: %w", err)
	}

	store, err := database.StoreWithDefaultSettings(tempDir, true, targetEngine)
	if err != nil {
		return fmt.Errorf("database initialization failed: %w", err)
	}

	// clean up temp db
	defer func() {
		store.Shutdown()
		_ = store.Close()
		_ = os.RemoveAll(tempDir)
	}()

	ts := time.Now()
	fmt.Println("calculating ledger state hash...")

	dbStorage, err := storage.New(store)
	if err != nil {
		return err
	}

	_, _, err = snapshot.LoadSnapshotFilesToStorage(context.Background(), dbStorage, fullPath, deltaPath)
	if err != nil {
		return err
	}

	ledgerIndex, err := dbStorage.UTXOManager().ReadLedgerIndex()
	if err != nil {
		return err
	}

	snapshotInfo := dbStorage.SnapshotInfo()
	if snapshotInfo == nil {
		return errors.New("no snapshot info found")
	}

	var solidEntryPoints hornet.LexicalOrderedMessageIDs
	dbStorage.ForEachSolidEntryPointWithoutLocking(func(sep *storage.SolidEntryPoint) bool {
		solidEntryPoints = append(solidEntryPoints, sep.MessageID)
		return true
	})
	// sort the solid entry points lexicographically by their MessageID
	sort.Sort(solidEntryPoints)

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
	snapshotHashSum := lsHash.Sum(nil)

	fmt.Printf(`> 
	- Snapshot time %v
	- Network ID %d
	- Treasury %s
	- Ledger index %d
	- Snapshot index %d
	- UTXOs count %d
	- SEPs count %d
	- Ledger state hash: %s`+"\n\n",
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
		hex.EncodeToString(snapshotHashSum),
	)

	fmt.Printf("successfully calculated ledger state hash, took %v\n", time.Since(ts).Truncate(time.Millisecond))

	return nil
}
