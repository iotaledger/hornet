package utils

import (
	"fmt"
	"os"
)

// LoadStringFromEnvironment loads a string from the given environment variable.
func LoadStringFromEnvironment(name string) (string, error) {

	str, exists := os.LookupEnv(name)
	if !exists {
		return "", fmt.Errorf("environment variable '%s' not set", name)
	}

	if len(str) == 0 {
		return "", fmt.Errorf("environment variable '%s' not set", name)
	}

	return str, nil
}
