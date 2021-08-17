package toolset

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dustin/go-humanize"

	dbCore "github.com/gohornet/hornet/core/database"
	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/pebble"
	"github.com/iotaledger/hive.go/kvstore/rocksdb"
)

func databaseMigration(config *configuration.Configuration, args []string) error {
	printUsage := func() {
		println("Usage:")
		println(fmt.Sprintf("	%s [DB_ENGINE]", ToolDatabaseMigration))
		println()
		println("   [DB_ENGINE] - target database engine (values: pebble, rocksdb)")
	}

	// check arguments
	if len(args) == 0 {
		printUsage()
		return errors.New("DB_ENGINE missing")
	}

	if len(args) > 1 {
		printUsage()
		return fmt.Errorf("too many arguments for '%s'", ToolDatabaseMigration)
	}

	var dbEngineTarget string
	if len(args) == 1 {
		dbEngineTarget = strings.ToLower(args[0])
	}

	sourcePath := config.String(dbCore.CfgDatabasePath)
	targetPath := fmt.Sprintf("%s_migrated", sourcePath)
	_, err := os.Stat(targetPath)
	if err == nil {
		if err = os.RemoveAll(targetPath); err != nil {
			return fmt.Errorf("can't remove target database directory: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("can't check target database directory: %w", err)
	}

	var storeSource kvstore.KVStore
	var storeTarget kvstore.KVStore

	switch config.String(dbCore.CfgDatabaseEngine) {
	case "pebble":
		db, err := database.NewPebbleDB(sourcePath, nil, true)
		if err != nil {
			return fmt.Errorf("source database initialization failed: %w", err)
		}
		storeSource = pebble.New(db)

	case "bolt":
		return errors.New("bolt database can't be migrated")

	case "rocksdb":
		db, err := database.NewRocksDB(sourcePath)
		if err != nil {
			return fmt.Errorf("source database initialization failed: %w", err)
		}
		storeSource = rocksdb.New(db)

	default:
		return fmt.Errorf("unknown source database engine: %s, supported engines: pebble/rocksdb", config.String(dbCore.CfgDatabaseEngine))
	}

	defer func() { _ = storeSource.Close() }()

	switch dbEngineTarget {
	case "pebble":
		db, err := database.NewPebbleDB(targetPath, nil, true)
		if err != nil {
			return fmt.Errorf("target database initialization failed: %w", err)
		}
		storeTarget = pebble.New(db)

	case "rocksdb":
		db, err := database.NewRocksDB(targetPath)
		if err != nil {
			return fmt.Errorf("target database initialization failed: %w", err)
		}
		storeTarget = rocksdb.New(db)

	default:
		return fmt.Errorf("unknown target database engine: %s, supported engines: pebble/rocksdb", dbEngineTarget)
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
