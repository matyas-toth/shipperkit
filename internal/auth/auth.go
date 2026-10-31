// Package auth provides password and token primitives.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func Token() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func Hash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func Equal(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(Hash(a)), []byte(Hash(b))) == 1
}

func Password(password string) (string, error) {
	if len(password) < 12 || len(password) > 72 {
		return "", errors.New("Use a password between 12 and 72 bytes")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(hash), err
}
func Verify(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// SetupToken is a filesystem capability, never returned by an HTTP endpoint.
func SetupToken(dir, override string) (string, error) {
	if override != "" {
		if len(override) < 32 {
			return "", errors.New("SHIPPER_SETUP_TOKEN must contain at least 32 characters")
		}
		return override, nil
	}
	file := filepath.Join(dir, "setup-token")
	existing, err := os.ReadFile(file)
	if err == nil {
		token := strings.TrimSpace(string(existing))
		if len(token) != 64 {
			return "", errors.New("invalid setup-token file")
		}
		return token, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	token := Token()
	handle, err := os.OpenFile(file, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if os.IsExist(err) {
		return SetupToken(dir, override)
	}
	if err != nil {
		return "", err
	}
	_, writeErr := handle.WriteString(token + "\n")
	closeErr := handle.Close()
	if writeErr != nil {
		return "", writeErr
	}
	return token, closeErr
}
