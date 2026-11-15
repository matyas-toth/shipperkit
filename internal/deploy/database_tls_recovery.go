package deploy

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const maxDatabaseTrustBytes = 16 << 20

var databaseTrustID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,100}$`)

type databaseTrust struct {
	Application string `json:"application"`
	Certificate []byte `json:"certificate"`
	Key         []byte `json:"key"`
}

func privateTrustFile(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, errors.New("Invalid database trust file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if int64(len(raw)) > limit {
		return nil, errors.New("Database trust file exceeds its limit")
	}
	return raw, err
}

func validateDatabaseTrust(v databaseTrust) error {
	if !databaseTrustID.MatchString(v.Application) || len(v.Certificate) > 64<<10 || len(v.Key) > 64<<10 {
		return errors.New("Invalid database trust identity")
	}
	cert, rest := pem.Decode(v.Certificate)
	if cert == nil || cert.Type != "CERTIFICATE" || len(bytes.TrimSpace(rest)) != 0 {
		return errors.New("Invalid database CA certificate")
	}
	ca, err := x509.ParseCertificate(cert.Bytes)
	if err != nil || !ca.IsCA || ca.CheckSignatureFrom(ca) != nil {
		return errors.New("Invalid database CA certificate")
	}
	key, rest := pem.Decode(v.Key)
	if key == nil || key.Type != "EC PRIVATE KEY" || len(bytes.TrimSpace(rest)) != 0 {
		return errors.New("Invalid database CA key")
	}
	private, err := x509.ParseECPrivateKey(key.Bytes)
	if err != nil {
		return errors.New("Invalid database CA key")
	}
	public, err := x509.MarshalPKIXPublicKey(&private.PublicKey)
	if err != nil || !bytes.Equal(public, ca.RawSubjectPublicKeyInfo) {
		return errors.New("Database CA key does not match its certificate")
	}
	return nil
}

func bundleDatabaseTrust(data, destination string) error {
	root := filepath.Join(data, "database-tls")
	info, err := os.Lstat(root)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	identities := []databaseTrust{}
	total := 0
	if err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("Invalid database trust directory")
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			return err
		}
		if len(entries) > 4096 {
			return errors.New("Too many database trust identities")
		}
		for _, entry := range entries {
			if !entry.IsDir() || !databaseTrustID.MatchString(entry.Name()) {
				return errors.New("Invalid database trust directory")
			}
			v := databaseTrust{Application: entry.Name()}
			v.Certificate, err = privateTrustFile(filepath.Join(root, entry.Name(), "ca.crt"), 64<<10)
			if err != nil {
				return err
			}
			v.Key, err = privateTrustFile(filepath.Join(root, entry.Name(), "ca.key"), 64<<10)
			if err != nil {
				return err
			}
			if err = validateDatabaseTrust(v); err != nil {
				return err
			}
			total += len(v.Certificate) + len(v.Key)
			if total > maxDatabaseTrustBytes {
				return errors.New("Database trust identities exceed the recovery limit")
			}
			identities = append(identities, v)
		}
	}
	raw, err := json.Marshal(identities)
	if err != nil {
		return err
	}
	if len(raw) > maxDatabaseTrustBytes {
		return errors.New("Database trust identities exceed the recovery limit")
	}
	return os.WriteFile(destination, raw, 0600)
}

func restoreDatabaseTrust(stage string) error {
	file := filepath.Join(stage, "database-tls.json")
	raw, err := privateTrustFile(file, maxDatabaseTrustBytes)
	if os.IsNotExist(err) {
		return nil
	} // Archives from earlier versions have no database trust bundle.
	if err != nil {
		return err
	}
	var identities []databaseTrust
	if err = json.Unmarshal(raw, &identities); err != nil {
		return err
	}
	if len(identities) > 4096 {
		return errors.New("Too many database trust identities")
	}
	seen := map[string]bool{}
	for _, v := range identities {
		id := strings.ToLower(v.Application)
		if seen[id] {
			return errors.New("Duplicate database trust identity")
		}
		seen[id] = true
		if err = validateDatabaseTrust(v); err != nil {
			return err
		}
	}
	for _, v := range identities {
		dir := filepath.Join(stage, "database-tls", v.Application)
		if err = os.MkdirAll(dir, 0700); err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(dir, "ca.crt"), v.Certificate, 0600); err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(dir, "ca.key"), v.Key, 0600); err != nil {
			return err
		}
	}
	return os.Remove(file)
}

func copyRecoveryDirectory(source, target string) error {
	if err := os.Mkdir(target, 0700); err != nil {
		return err
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		from, to := filepath.Join(source, entry.Name()), filepath.Join(target, entry.Name())
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("Recovery files cannot be symbolic links")
		}
		if entry.IsDir() {
			err = copyRecoveryDirectory(from, to)
		} else {
			err = copyPrivate(from, to)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
