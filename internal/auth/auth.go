// Package auth provides password and token primitives.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

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
	token, err := ReadSetupClaim(dir)
	if err == nil {
		return token, nil
	}
	if errors.Is(err, ErrClaimExpired) {
		return RotateSetupClaim(dir)
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	token = Token()
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

var ErrClaimExpired = errors.New("the setup token expired")

func ReadSetupClaim(dir string) (string, error) {
	file, err := os.Open(filepath.Join(dir, "setup-token"))
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() > 128 {
		return "", errors.New("invalid setup-token file")
	}
	if time.Since(info.ModTime()) >= 30*time.Minute {
		return "", ErrClaimExpired
	}
	data := make([]byte, info.Size())
	if _, err = io.ReadFull(file, data); err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(data))
	if len(token) != 64 {
		return "", errors.New("invalid setup-token file")
	}
	return token, nil
}

func RotateSetupClaim(dir string) (string, error) {
	file, err := os.CreateTemp(dir, "setup-token-")
	if err != nil {
		return "", err
	}
	defer os.Remove(file.Name())
	token := Token()
	_, err = file.WriteString(token + "\n")
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		return "", errors.Join(err, closeErr)
	}
	return token, os.Rename(file.Name(), filepath.Join(dir, "setup-token"))
}
