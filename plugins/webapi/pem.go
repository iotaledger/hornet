package webapi

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
)

var (
	ErrPrivKeyInvalid   = errors.New("invalid private key")
	ErrNoPrivKeyFound   = errors.New("no private key found")
	ErrInvalidKeyLength = errors.New("invalid key length")
)

// pathExists returns whether the given file or directory exists.
func pathExists(path string) (exists bool, isDirectory bool, err error) {
	fileInfo, err := os.Stat(path)
	if err == nil {
		return true, fileInfo.IsDir(), nil
	}
	if os.IsNotExist(err) {
		return false, false, nil
	}

	return false, false, err
}

// createDirectory checks if the directory exists,
// otherwise it creates it with given permissions.
func createDirectory(dir string, perm os.FileMode) error {
	exists, isDir, err := pathExists(dir)
	if err != nil {
		return err
	}

	if exists {
		if !isDir {
			return fmt.Errorf("given path is a file instead of a directory %s", dir)
		}

		return nil
	}

	return os.MkdirAll(dir, perm)
}

// writeToFile writes the binary representation of data to a file named by filename.
// If the file does not exist, WriteFile creates it with permissions perm
// (before umask); otherwise WriteFile truncates it before writing, without changing permissions.
// writeToFile uses binary.Write to encode data. Data must be a pointer to a fixed-size value or a slice
// of fixed-size values.
func writeToFile(filename string, data interface{}, perm os.FileMode) (err error) {
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

// parseEd25519PrivateKeyFromString parses an ed25519 private key from a string.
func parseEd25519PrivateKeyFromString(key string) (ed25519.PrivateKey, error) {
	keyBytes, err := hex.DecodeString(key)
	if err != nil {
		return nil, err
	}

	if len(keyBytes) != ed25519.PrivateKeySize {
		return nil, ErrInvalidKeyLength
	}

	return ed25519.PrivateKey(keyBytes), nil
}

// parseLibp2pEd25519PrivateKeyFromString parses an Ed25519 private key from a hex encoded string.
func parseLibp2pEd25519PrivateKeyFromString(identityPrivKey string) (libp2pcrypto.PrivKey, error) {
	if identityPrivKey == "" {
		return nil, ErrNoPrivKeyFound
	}

	privKey, err := parseEd25519PrivateKeyFromString(identityPrivKey)
	if err != nil {
		return nil, errors.Join(ErrPrivKeyInvalid, errors.New("unable to parse private key"))
	}

	libp2pPrivKey, err := ed25519PrivateKeyToLibp2pPrivateKey(privKey)
	if err != nil {
		return nil, err
	}

	return libp2pPrivKey, nil
}

func ed25519PrivateKeyToLibp2pPrivateKey(privKey ed25519.PrivateKey) (libp2pcrypto.PrivKey, error) {
	libp2pPrivKey, _, err := libp2pcrypto.KeyPairFromStdKey(&privKey)
	if err != nil {

		return nil, errors.Join(err, errors.New("unable to unmarshal private key"))
	}

	return libp2pPrivKey, nil
}

func libp2pPrivateKeyToEd25519PrivateKey(libp2pPrivKey libp2pcrypto.PrivKey) (ed25519.PrivateKey, error) {
	cryptoPrivKey, err := libp2pcrypto.PrivKeyToStdKey(libp2pPrivKey)
	if err != nil {
		return nil, errors.Join(err, errors.New("unable to convert private key"))
	}

	privKey, ok := cryptoPrivKey.(*ed25519.PrivateKey)
	if !ok {
		return nil, errors.Join(err, errors.New("unable to type assert private key"))
	}

	return *privKey, nil
}

// readEd25519PrivateKeyFromPEMFile reads an Ed25519 private key from a file with PEM format.
func readEd25519PrivateKeyFromPEMFile(filepath string) (ed25519.PrivateKey, error) {

	pemPrivateBlockBytes, err := os.ReadFile(filepath)
	if err != nil {
		return nil, errors.Join(err, errors.New("unable to read private key"))
	}

	pemPrivateBlock, _ := pem.Decode(pemPrivateBlockBytes)
	if pemPrivateBlock == nil {
		return nil, errors.New("unable to decode private key")
	}

	cryptoPrivKey, err := x509.ParsePKCS8PrivateKey(pemPrivateBlock.Bytes)
	if err != nil {
		return nil, errors.Join(err, errors.New("unable to parse private key"))
	}

	privKey, ok := cryptoPrivKey.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("unable to type assert private key")
	}

	return privKey, nil
}

// writeEd25519PrivateKeyToPEMFile stores an Ed25519 private key to a file with PEM format.
func writeEd25519PrivateKeyToPEMFile(filepath string, privateKey ed25519.PrivateKey) error {

	if err := createDirectory(path.Dir(filepath), 0o700); err != nil {
		return errors.Join(err, errors.New("unable to store private key"))
	}

	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return errors.Join(err, errors.New("unable to marshal private key"))
	}

	pemPrivateBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Bytes,
	}

	var pemBuffer bytes.Buffer
	if err := pem.Encode(&pemBuffer, pemPrivateBlock); err != nil {
		return errors.Join(err, errors.New("unable to encode private key"))
	}

	if err := writeToFile(filepath, pemBuffer.Bytes(), 0660); err != nil {
		return errors.Join(err, errors.New("unable to write private key"))
	}

	return nil
}

// LoadOrCreateIdentityPrivateKey loads an existing Ed25519 based identity private key
// or creates a new one and stores it as a PEM file in the p2p store folder.
func LoadOrCreateIdentityPrivateKey(privKeyFilePath string, identityPrivKey string) (libp2pcrypto.PrivKey, bool, error) {

	privKeyFromConfig, err := parseLibp2pEd25519PrivateKeyFromString(identityPrivKey)
	if err != nil {
		if errors.Is(err, ErrPrivKeyInvalid) {
			return nil, false, errors.New("configuration contains an invalid private key")
		}

		if !errors.Is(err, ErrNoPrivKeyFound) {
			return nil, false, errors.Join(err, errors.New("unable to parse private key from config"))
		}
	}

	_, err = os.Stat(privKeyFilePath)
	switch {
	case err == nil || os.IsExist(err):
		// private key already exists, load and return it
		privKey, err := readEd25519PrivateKeyFromPEMFile(privKeyFilePath)
		if err != nil {
			return nil, false, errors.Join(err, errors.New("unable to load Ed25519 private key for peer identity"))
		}

		libp2pPrivKey, err := ed25519PrivateKeyToLibp2pPrivateKey(privKey)
		if err != nil {
			return nil, false, err
		}

		if privKeyFromConfig != nil && !privKeyFromConfig.Equals(libp2pPrivKey) {
			storedPrivKeyBytes, err := libp2pcrypto.MarshalPrivateKey(libp2pPrivKey)
			if err != nil {
				return nil, false, errors.Join(err, errors.New("unable to marshal stored Ed25519 private key for peer identity"))
			}
			configPrivKeyBytes, err := libp2pcrypto.MarshalPrivateKey(privKeyFromConfig)
			if err != nil {
				return nil, false, errors.Join(err, errors.New("unable to marshal configured Ed25519 private key for peer identity"))
			}

			return nil, false, fmt.Errorf("stored Ed25519 private key (%s) for peer identity doesn't match private key in config (%s)", hex.EncodeToString(storedPrivKeyBytes), hex.EncodeToString(configPrivKeyBytes))
		}

		return libp2pPrivKey, false, nil

	case os.IsNotExist(err):
		var libp2pPrivKey libp2pcrypto.PrivKey

		if privKeyFromConfig != nil {
			libp2pPrivKey = privKeyFromConfig
		} else {
			// private key does not exist, create a new one
			libp2pPrivKey, _, err = libp2pcrypto.GenerateKeyPair(libp2pcrypto.Ed25519, -1)
			if err != nil {
				return nil, false, errors.Join(err, errors.New("unable to generate Ed25519 private key for peer identity"))
			}
		}

		privKey, err := libp2pPrivateKeyToEd25519PrivateKey(libp2pPrivKey)
		if err != nil {
			return nil, false, err
		}

		if err := writeEd25519PrivateKeyToPEMFile(privKeyFilePath, privKey); err != nil {
			return nil, false, errors.Join(err, errors.New("unable to store private key file for peer identity"))
		}

		return libp2pPrivKey, true, nil

	default:

		return nil, false, errors.Join(err, fmt.Errorf("unable to check private key file for peer identity (%s)", privKeyFilePath))
	}
}
