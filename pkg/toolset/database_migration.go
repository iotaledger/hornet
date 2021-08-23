package toolset

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/kvstore"
)

func databaseMigration(_ *configuration.Configuration, args []string) error {
	printUsage := func() {
		println("Usage:")
		println(fmt.Sprintf("	%s [SOURCE_DATABASE_PATH] [TARGET_DATABASE_PATH] [TARGET_DATABASE_ENGINE]", ToolDatabaseMigration))
		println()
		println("   [SOURCE_DATABASE_PATH]   - the path to the source database")
		println("   [TARGET_DATABASE_PATH]   - the path to the target database")
		println("   [TARGET_DATABASE_ENGINE] - the engine of the target database (values: pebble, rocksdb)")
		println()
		println(fmt.Sprintf("example: %s %s %s %s", ToolDatabaseMigration, "mainnetdb", "mainnetdb_new", "rocksdb"))
	}

	// check arguments
	if len(args) != 3 {
		printUsage()
		return fmt.Errorf("wrong argument count for '%s'", ToolDatabaseMigration)
	}

	sourcePath := args[0]
	if _, err := os.Stat(sourcePath); err != nil || os.IsNotExist(err) {
		return fmt.Errorf("SOURCE_DATABASE_PATH (%s) does not exist", sourcePath)
	}

	targetPath := args[1]
	if _, err := os.Stat(targetPath); err == nil || !os.IsNotExist(err) {
		return fmt.Errorf("TARGET_DATABASE_PATH (%s) already exist", targetPath)
	}

	dbEngineTarget := strings.ToLower(args[2])
	engineTarget, err := database.DatabaseEngine(dbEngineTarget)
	if err != nil {
		return err
	}

	storeSource, err := database.StoreWithDefaultSettings(sourcePath, false)
	if err != nil {
		return fmt.Errorf("source database initialization failed: %w", err)
	}
	defer func() { _ = storeSource.Close() }()

	storeTarget, err := database.StoreWithDefaultSettings(targetPath, true, engineTarget)
	if err != nil {
		return fmt.Errorf("target database initialization failed: %w", err)
	}
	defer func() { _ = storeTarget.Close() }()

	copyBytes := func(source []byte) []byte {
		cpy := make([]byte, len(source))
		copy(cpy, source)
		return cpy
	}

	ts := time.Now()
	lastStatusTime := time.Now()

	sourcePathAbs, err := filepath.Abs(sourcePath)
	if err != nil {
		sourcePathAbs = sourcePath
	}
	targetPathAbs, err := filepath.Abs(targetPath)
	if err != nil {
		targetPathAbs = targetPath
	}

	fmt.Printf("Migrating database... (source: \"%s\", target: \"%s\")\n", sourcePathAbs, targetPathAbs)

	var errDB error
	if err := storeSource.Iterate(kvstore.EmptyPrefix, func(key []byte, value kvstore.Value) bool {
		dstKey := copyBytes(key)
		dstValue := copyBytes(value)

		if errDB = storeTarget.Set(dstKey, dstValue); errDB != nil {
			return false
		}

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			sourceSizeBytes, _ := utils.FolderSize(sourcePath)
			targetSizeBytes, _ := utils.FolderSize(targetPath)

			percentage, remaining := utils.EstimateRemainingTime(ts, targetSizeBytes, sourceSizeBytes)
			fmt.Printf("Source database size: %s, target database size: %s, estimated percentage: %0.2f%%. %v elapsed, %v left...)\n", humanize.Bytes(uint64(sourceSizeBytes)), humanize.Bytes(uint64(targetSizeBytes)), percentage, time.Since(ts).Truncate(time.Second), remaining.Truncate(time.Second))
		}

		return true
	}); err != nil {
		return fmt.Errorf("source database iteration failed: %w", err)
	}

	if errDB != nil {
		return fmt.Errorf("target database set failed: %w", err)
	}

	if err := storeTarget.Flush(); err != nil {
		return fmt.Errorf("target database flush failed: %w", err)
	}

	fmt.Printf("Migration successful! took: %v\n", time.Since(ts).Truncate(time.Second))

	return nil
}
