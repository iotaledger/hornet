package basicauth

import (
	"crypto/sha256"
	"fmt"
)

// VerifyPassword checks whether the given password and salt compute to the expected value
func VerifyPassword(pw string, salt string, expected string) bool {
	return fmt.Sprintf("%x", sha256.Sum256(append([]byte(pw), []byte(salt)...))) == expected
}
