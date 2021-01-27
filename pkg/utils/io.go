package utils

import (
	"encoding/binary"
	"os"
)

func ReadFromFile(filename string, data interface{}) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	return binary.Read(f, binary.LittleEndian, data)
}

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
