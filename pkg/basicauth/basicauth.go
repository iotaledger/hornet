package basicauth

import (
	"bytes"
	"crypto/rand"

	"golang.org/x/crypto/scrypt"
)

// SaltGenerator generates a crypto-secure random salt.
func SaltGenerator(length int) ([]byte, error) {
	salt := make([]byte, length)

	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}

	return salt, nil
}

// GetPasswordKey calculates the key based on password and salt.
func GetPasswordKey(password []byte, salt []byte) ([]byte, error) {

	dk, err := scrypt.Key(password, salt, 1<<15, 8, 1, 32)
	if err != nil {
		return nil, err
	}

	return dk, err
}

// VerifyPassword verifies if the password is correct.
func VerifyPassword(password []byte, salt []byte, storedPasswordKey []byte) (bool, error) {

	dk, err := GetPasswordKey(password, salt)
	if err != nil {
		return false, err
	}

	return bytes.Equal(dk, storedPasswordKey), nil
}
