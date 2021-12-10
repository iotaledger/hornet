package utils

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// ReadFromFile reads structured binary data from the file named by filename to data.
// A successful call returns err == nil, not err == EOF.
// ReadFromFile uses binary.Read to decode data. Data must be a pointer to a fixed-size value or a slice
// of fixed-size values.
func ReadFromFile(filename string, data interface{}) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return binary.Read(f, binary.LittleEndian, data)
}

// WriteToFile writes the binary representation of data to a file named by filename.
// If the file does not exist, WriteFile creates it with permissions perm
// (before umask); otherwise WriteFile truncates it before writing, without changing permissions.
// WriteToFile uses binary.Write to encode data. Data must be a pointer to a fixed-size value or a slice
// of fixed-size values.
func WriteToFile(filename string, data interface{}, perm os.FileMode) (err error) {
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
	}()
	return binary.Write(f, binary.LittleEndian, data)
}

// ReadJSONFromFile reads JSON data from the file named by filename to data.
// ReadJSONFromFile uses json.Unmarshal to decode data. Data must be a pointer to a fixed-size value or a slice
// of fixed-size values.
func ReadJSONFromFile(filename string, data interface{}) error {
	jsonData, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("unable to read JSON file %s: %w", filename, err)
	}
	return json.Unmarshal(jsonData, data)
}

// WriteJSONToFile writes the JSON representation of data to a file named by filename.
// If the file does not exist, WriteJSONToFile creates it with permissions perm
// (before umask); otherwise WriteJSONToFile truncates it before writing, without changing permissions.
// WriteJSONToFile uses json.MarshalIndent to encode data. Data must be a pointer to a fixed-size value or a slice
// of fixed-size values.
func WriteJSONToFile(filename string, data interface{}, perm os.FileMode) (err error) {
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}

	defer func() {
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
	}()

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("unable to marshal data to JSON: %w", err)
	}

	if _, err := f.Write(jsonData); err != nil {
		return fmt.Errorf("unable to write JSON data to %s: %w", filename, err)
	}

	if err := f.Sync(); err != nil {
		return fmt.Errorf("unable to fsync file content to %s: %w", filename, err)
	}

	return nil
}

// ReadTOMLFromFile reads TOML data from the file named by filename to data.
// ReadTOMLFromFile uses toml.Unmarshal to decode data. Data must be a pointer to a fixed-size value or a slice
// of fixed-size values.
func ReadTOMLFromFile(filename string, data interface{}) error {
	tomlData, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("unable to read TOML file %s: %w", filename, err)
	}
	return toml.Unmarshal(tomlData, data)
}

// WriteTOMLToFile writes the TOML representation of data to a file named by filename.
// If the file does not exist, WriteTOMLToFile creates it with permissions perm
// (before umask); otherwise WriteTOMLToFile truncates it before writing, without changing permissions.
// WriteTOMLToFile uses toml.Marshal to encode data. Data must be a pointer to a fixed-size value or a slice
// of fixed-size values. An additional header can be passed.
func WriteTOMLToFile(filename string, data interface{}, perm os.FileMode, header ...string) (err error) {
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}

	defer func() {
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
	}()

	tomlData, err := toml.Marshal(data)
	if err != nil {
		return fmt.Errorf("unable to marshal data to TOML: %w", err)
	}

	if len(header) > 0 {
		if _, err := f.Write([]byte(header[0] + "\n")); err != nil {
			return fmt.Errorf("unable to write header to %s: %w", filename, err)
		}
	}

	if _, err := f.Write(tomlData); err != nil {
		return fmt.Errorf("unable to write TOML data to %s: %w", filename, err)
	}

	if err := f.Sync(); err != nil {
		return fmt.Errorf("unable to fsync file content to %s: %w", filename, err)
	}

	return nil
}

// CreateTempFile creates a file descriptor with _tmp as file extension.
func CreateTempFile(filePath string) (*os.File, string, error) {
	filePathTmp := filePath + "_tmp"

	// we don't need to check the error, maybe the file doesn't exist
	_ = os.Remove(filePathTmp)

	fileDescriptor, err := os.OpenFile(filePathTmp, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, "", fmt.Errorf("unable to create temp file: %w", err)
	}
	return fileDescriptor, filePathTmp, nil
}

// CloseFileAndRename closes the file descriptor and renames the file.
func CloseFileAndRename(fileDescriptor *os.File, sourceFilePath string, targetFilePath string) error {
	if err := fileDescriptor.Close(); err != nil {
		return fmt.Errorf("unable to close file: %w", err)
	}
	if err := os.Rename(sourceFilePath, targetFilePath); err != nil {
		return fmt.Errorf("unable to rename file: %w", err)
	}
	return nil
}

// DirectoryEmpty returns whether the given directory is empty.
func DirectoryEmpty(dirPath string) (bool, error) {

	// check if the directory exists
	if _, err := os.Stat(dirPath); err != nil {
		return false, fmt.Errorf("unable to check directory (%s): %w", dirPath, err)
	}

	// check if the directory is empty
	if err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if dirPath == path {
			// skip the root folder itself
			return nil
		}

		return os.ErrExist
	}); err != nil {
		if !os.IsExist(err) {
			return false, fmt.Errorf("unable to check directory (%s): %w", dirPath, err)
		}

		// directory is not empty
		return false, nil
	}

	// directory is empty
	return true, nil
}
