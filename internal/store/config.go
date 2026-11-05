package store

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func configKey(dir string) (cipher.AEAD, error) {
	file := filepath.Join(dir, "encryption.key")
	key, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		key = make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			return nil, err
		}
		f, e := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			key, err = os.ReadFile(file)
		} else {
			_, err = f.Write(key)
			if e = f.Sync(); err == nil {
				err = e
			}
			f.Close()
		}
	}
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, errors.New("The encryption key must contain exactly 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func (s *Store) marshalConfig(c Config) (string, error) {
	if c.InlineDockerfile != "" {
		sealed, e := s.seal(c.InlineDockerfile, "inline-dockerfile")
		if e != nil {
			return "", e
		}
		c.InlineDockerfile = sealed
	}
	c.InlineKeep = false
	c.InlineHasValue = false
	c.Variables = append([]Variable(nil), c.Variables...)
	for i := range c.Variables {
		v := &c.Variables[i]
		nonce := make([]byte, s.key.NonceSize())
		if _, e := rand.Read(nonce); e != nil {
			return "", e
		}
		v.Value = "sealed:v1:" + base64.StdEncoding.EncodeToString(s.key.Seal(nonce, nonce, []byte(v.Value), []byte(v.Name)))
		v.Keep = false
		v.KeepFrom = ""
		v.HasValue = false
	}
	c.Services = append([]ServiceConfig(nil), c.Services...)
	for i := range c.Services {
		raw, e := s.marshalConfig(Config{Variables: c.Services[i].Variables})
		if e != nil {
			return "", e
		}
		var inner Config
		if e = json.Unmarshal([]byte(raw), &inner); e != nil {
			return "", e
		}
		c.Services[i].Variables = inner.Variables
	}
	if c.ComposeYAML != "" {
		sealed, e := s.seal(c.ComposeYAML, "compose-yaml")
		if e != nil {
			return "", e
		}
		c.ComposeYAML = sealed
	}
	c.ComposeKeep = false
	c.ComposeHasValue = false
	data, e := json.Marshal(c)
	if e != nil {
		return "", e
	}
	var obj map[string]json.RawMessage
	if e = json.Unmarshal(data, &obj); e != nil {
		return "", e
	}
	if len(c.Hooks) > 0 {
		sealed, err := s.seal(string(obj["hooks"]), "hooks")
		if err != nil {
			return "", err
		}
		obj["hooks"], _ = json.Marshal(sealed)
	}
	data, e = json.Marshal(obj)
	return string(data), e
}
func (s *Store) unmarshalConfig(raw string, c *Config) error {
	var obj map[string]json.RawMessage
	if e := json.Unmarshal([]byte(raw), &obj); e != nil {
		return e
	}
	if hooks := obj["hooks"]; len(hooks) > 0 && hooks[0] == '"' {
		var sealed string
		if e := json.Unmarshal(hooks, &sealed); e != nil {
			return e
		}
		plain, e := s.unseal(sealed, "hooks")
		if e != nil {
			return e
		}
		obj["hooks"] = json.RawMessage(plain)
		data, e := json.Marshal(obj)
		if e != nil {
			return e
		}
		raw = string(data)
	}
	if e := json.Unmarshal([]byte(raw), c); e != nil {
		return e
	}
	if c.InlineDockerfile != "" {
		plain, e := s.unseal(c.InlineDockerfile, "inline-dockerfile")
		if e != nil {
			return e
		}
		c.InlineDockerfile = plain
	}
	for i := range c.Variables {
		v := &c.Variables[i]
		if !strings.HasPrefix(v.Value, "sealed:v1:") {
			continue
		}
		data, e := base64.StdEncoding.DecodeString(strings.TrimPrefix(v.Value, "sealed:v1:"))
		if e != nil || len(data) < s.key.NonceSize() {
			return errors.New("Stored variable ciphertext is invalid")
		}
		value, e := s.key.Open(nil, data[:s.key.NonceSize()], data[s.key.NonceSize():], []byte(v.Name))
		if e != nil {
			return fmt.Errorf("Variable %s could not be decrypted; restore the original encryption.key", v.Name)
		}
		v.Value = string(value)
	}
	for i := range c.Services {
		raw, e := json.Marshal(Config{Variables: c.Services[i].Variables})
		if e != nil {
			return e
		}
		var inner Config
		if e = s.unmarshalConfig(string(raw), &inner); e != nil {
			return e
		}
		c.Services[i].Variables = inner.Variables
	}
	if c.ComposeYAML != "" {
		var e error
		c.ComposeYAML, e = s.unseal(c.ComposeYAML, "compose-yaml")
		if e != nil {
			return e
		}
	}
	return nil
}
func (s *Store) seal(value, purpose string) (string, error) {
	if value == "" {
		return "", nil
	}
	nonce := make([]byte, s.key.NonceSize())
	if _, e := rand.Read(nonce); e != nil {
		return "", e
	}
	return "sealed:v1:" + base64.StdEncoding.EncodeToString(s.key.Seal(nonce, nonce, []byte(value), []byte(purpose))), nil
}
func (s *Store) unseal(value, purpose string) (string, error) {
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(value, "sealed:v1:") {
		return "", errors.New("Stored Compose data is not encrypted")
	}
	data, e := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, "sealed:v1:"))
	if e != nil || len(data) < s.key.NonceSize() {
		return "", errors.New("Stored Compose ciphertext is invalid")
	}
	plain, e := s.key.Open(nil, data[:s.key.NonceSize()], data[s.key.NonceSize():], []byte(purpose))
	if e != nil {
		return "", errors.New("Compose data could not be decrypted; restore encryption.key")
	}
	return string(plain), nil
}
func PublicConfig(c Config) Config {
	if c.InlineDockerfile != "" {
		c.InlineDockerfile = ""
		c.InlineKeep = true
		c.InlineHasValue = true
	}
	c.Services = append([]ServiceConfig(nil), c.Services...)
	for i := range c.Services {
		c.Services[i].Variables = PublicConfig(Config{Variables: c.Services[i].Variables}).Variables
	}
	if c.ComposeYAML != "" {
		c.ComposeYAML = ""
		c.ComposeKeep = true
		c.ComposeHasValue = true
	}
	c.Variables = append([]Variable(nil), c.Variables...)
	for i := range c.Variables {
		v := &c.Variables[i]
		v.HasValue = true
		if v.Secret {
			v.Value = ""
			v.Keep = true
		}
	}
	return c
}
func PublicApplication(a Application) Application {
	a.Config = PublicConfig(a.Config)
	if a.LatestDeployment != nil {
		d := PublicDeployment(*a.LatestDeployment)
		a.LatestDeployment = &d
	}
	return a
}
func PublicDeployment(d Deployment) Deployment { d.Config = PublicConfig(d.Config); return d }
func MergeVariables(next *Config, old Config) error {
	if next.CommandsKeep {
		next.Hooks = old.Hooks
		next.InstallCommand = old.InstallCommand
		next.BuildCommand = old.BuildCommand
		next.StartCommand = old.StartCommand
		next.CommandsKeep = false
	}
	if next.InlineKeep {
		next.InlineDockerfile = old.InlineDockerfile
		next.InlineKeep = false
		next.InlineHasValue = false
	}
	if next.ComposeKeep {
		next.ComposeYAML = old.ComposeYAML
		next.ComposeKeep = false
		next.ComposeHasValue = false
	}
	for i := range next.Services {
		var previous Config
		for _, svc := range old.Services {
			if svc.Name == next.Services[i].Name {
				previous.Variables = svc.Variables
			}
		}
		inner := Config{Variables: next.Services[i].Variables}
		if e := MergeVariables(&inner, previous); e != nil {
			return e
		}
		next.Services[i].Variables = inner.Variables
	}
	for i := range next.Variables {
		v := &next.Variables[i]
		if !v.Keep {
			continue
		}
		found := false
		name := v.Name
		if v.KeepFrom != "" {
			name = v.KeepFrom
		}
		for _, previous := range old.Variables {
			if previous.Name == name {
				v.Value = previous.Value
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("Variable %s has no saved value", v.Name)
		}
		v.Keep = false
		v.KeepFrom = ""
		v.HasValue = false
	}
	return nil
}
func PrimaryDomain(domains []Domain) Domain {
	for _, d := range domains {
		if d.Primary {
			return d
		}
	}
	if len(domains) > 0 {
		return domains[0]
	}
	return Domain{}
}

// SetDomains changes only routing metadata. Image identity and running containers
// are untouched; the durable routing job handles external apply and rollback.
func (s *Store) SetDomains(ctx context.Context, id string, domains []Domain) error {
	a, e := s.Application(ctx, id)
	if e != nil {
		return e
	}
	primary := PrimaryDomain(domains)
	a.Config.Domain = primary.Hostname
	a.Config.HTTPS = primary.HTTPS
	a.Config.Domains = nil
	cfg, e := s.marshalConfig(a.Config)
	if e != nil {
		return e
	}
	raw, _ := json.Marshal(domains)
	_, e = s.db.ExecContext(ctx, "UPDATE applications SET domains_json=?,config_json=? WHERE id=? AND deleted_at=0", string(raw), cfg, id)
	return e
}
func (s *Store) ReleaseDomainClaims(ctx context.Context, id string) error {
	_, e := s.db.ExecContext(ctx, "DELETE FROM domain_claims WHERE application_id=? AND hostname NOT IN (SELECT json_extract(value,'$.hostname') FROM applications,json_each(domains_json) WHERE applications.id=? AND applications.deleted_at=0)", id, id)
	return e
}
func (s *Store) Rename(ctx context.Context, id, name string) error {
	_, e := s.db.ExecContext(ctx, "UPDATE applications SET name=? WHERE id=? AND deleted_at=0", name, id)
	return e
}

func (s *Store) SetDesiredState(ctx context.Context, id, state string) error {
	_, e := s.db.ExecContext(ctx, "UPDATE applications SET desired_state=? WHERE id=? AND deleted_at=0", state, id)
	return e
}

func (s *Store) SetDomainCheck(ctx context.Context, id string, domain Domain, state, message string) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var data string
	if e = tx.QueryRowContext(ctx, "SELECT domains_json FROM applications WHERE id=? AND deleted_at=0", id).Scan(&data); e != nil {
		return e
	}
	var domains []Domain
	if e = json.Unmarshal([]byte(data), &domains); e != nil {
		return e
	}
	for i := range domains {
		d := &domains[i]
		if d.ID == domain.ID && d.Hostname == domain.Hostname && d.HTTPS == domain.HTTPS && d.Service == domain.Service {
			d.VerificationState = state
			d.Message = message
			d.CheckedAt = time.Now().Unix()
			d.CertificateExpiresAt = domain.CertificateExpiresAt
			d.CertificateIssuer = domain.CertificateIssuer
		}
	}
	raw, _ := json.Marshal(domains)
	if _, e = tx.ExecContext(ctx, "UPDATE applications SET domains_json=? WHERE id=?", string(raw), id); e != nil {
		return e
	}
	return tx.Commit()
}
