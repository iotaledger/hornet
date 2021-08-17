package utils

import (
	"os"
	"path/filepath"
)

// FolderSize returns the size of a folder.
func FolderSize(target string) (int64, error) {

	var size int64

	err := filepath.Walk(target, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			size += info.Size()
		}

		return err
	})

	return size, err
}
