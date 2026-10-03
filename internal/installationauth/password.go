// Package installationauth contains installation-owner credential primitives.
package installationauth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	PasswordHashScheme = "argon2id-v1"
	argonMemoryKiB     = 64 * 1024
	argonIterations    = 3
	argonParallelism   = 4
	argonSaltBytes     = 16
	argonKeyBytes      = 32
	maxPasswordBytes   = 1024
)

var ErrInvalidPassword = errors.New("installation owner password is invalid")

var passwordWorkGate = make(chan struct{}, 2)

func derivePasswordKey(password, salt []byte) []byte {
	passwordWorkGate <- struct{}{}
	defer func() { <-passwordWorkGate }()
	return argon2.IDKey(password, salt, argonIterations, argonMemoryKiB, argonParallelism, argonKeyBytes)
}

// HashPassword uses Argon2id with the 64 MiB, three-pass RFC 9106 parameter
// set. The encoded parameters are fixed so stored data cannot request
// unbounded work when it is verified.
func HashPassword(password []byte) (string, error) {
	if len(password) < 1 || len(password) > maxPasswordBytes {
		return "", ErrInvalidPassword
	}
	salt := make([]byte, argonSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	derived := derivePasswordKey(password, salt)
	return "$argon2id$v=19$m=65536,t=3,p=4$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(derived), nil
}

// VerifyPassword accepts only this implementation's bounded Argon2id format.
// Parameter upgrades must be explicit and reviewed before older hashes are
// accepted with different resource costs.
func VerifyPassword(encoded string, password []byte) bool {
	if len(password) < 1 || len(password) > maxPasswordBytes {
		return false
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" || parts[3] != "m=65536,t=3,p=4" {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != argonSaltBytes {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expected) != argonKeyBytes {
		return false
	}
	derived := derivePasswordKey(password, salt)
	return subtle.ConstantTimeCompare(derived, expected) == 1
}
