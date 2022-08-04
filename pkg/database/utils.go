package database

import (
	"fmt"

	"github.com/iotaledger/hive.go/core/ioutils"
)

// DatabaseExists checks if the database folder exists and is not empty.
func DatabaseExists(dbPath string) (bool, error) {

	dirExists, err := ioutils.PathExists(dbPath)
	if err != nil {
		return false, fmt.Errorf("unable to check database path (%s): %w", dbPath, err)
	}
	if !dirExists {
		return false, nil
	}

	// directory exists, but maybe database doesn't exist.
	// check if the directory is empty (needed for example in docker environments)
	dirEmpty, err := ioutils.DirectoryEmpty(dbPath)
	if err != nil {
		return false, fmt.Errorf("unable to check database path (%s): %w", dbPath, err)
	}

	return !dirEmpty, nil
}
