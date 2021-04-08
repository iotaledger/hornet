package basicauth

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"log"

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

type BasicAuth struct {
	username     string
	passwordHash []byte
	passwordSalt []byte
}

func NewBasicAuth(username string, passwordHashHex string, passwordSaltHex string) *BasicAuth {
	if len(username) == 0 {
		log.Fatal("username must not be empty")
	}

	if len(passwordHashHex) != 64 {
		log.Fatal("password hash must be 64 (hex encoded scrypt hash) in length")
	}

	if len(passwordSaltHex) != 64 {
		log.Fatal("password salt must be 64 (hex encoded) in length")
	}

	var err error
	passwordHash, err := hex.DecodeString(passwordHashHex)
	if err != nil {
		log.Fatal("password hash must be hex encoded")
	}

	passwordSalt, err := hex.DecodeString(passwordSaltHex)
	if err != nil {
		log.Fatal("password salt must be hex encoded")
	}

	return &BasicAuth{
		username:     username,
		passwordHash: passwordHash,
		passwordSalt: passwordSalt,
	}
}

func (b *BasicAuth) VerifyUsernameAndPassword(username string, password string) bool {
	if username != b.username {
		return false
	}

	valid, _ := VerifyPassword([]byte(password), b.passwordSalt, b.passwordHash)
	return valid
}
