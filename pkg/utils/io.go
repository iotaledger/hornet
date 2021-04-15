package utils

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
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
	defer f.Close()
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

	return
}
