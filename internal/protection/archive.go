// Package protection implements portable, authenticated recovery archives.
package protection

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"filippo.io/age"
)

const MaxBytes int64 = 2 << 30

type File struct {
	Name   string `json:"name"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}
type Manifest struct {
	Version       int    `json:"version"`
	Kind          string `json:"kind"`
	Instance      string `json:"instance"`
	Application   string `json:"application,omitempty"`
	Service       string `json:"service,omitempty"`
	Engine        string `json:"engine,omitempty"`
	EngineVersion string `json:"engineVersion,omitempty"`
	Database      string `json:"database,omitempty"`
	Volume        string `json:"volume,omitempty"`
	CreatedAt     int64  `json:"createdAt"`
	Consistency   string `json:"consistency"`
	Files         []File `json:"files"`
}

func Identity() (string, string, error) {
	k, e := age.GenerateX25519Identity()
	if e != nil {
		return "", "", e
	}
	return k.String(), k.Recipient().String(), nil
}
func Recipient(identity string) (string, string, error) {
	k, e := age.ParseX25519Identity(strings.TrimSpace(identity))
	if e != nil {
		return "", "", e
	}
	return k.String(), k.Recipient().String(), nil
}
func digest(file string) (int64, string, error) {
	f, e := os.Open(file)
	if e != nil {
		return 0, "", e
	}
	defer f.Close()
	h := sha256.New()
	n, e := io.Copy(h, io.LimitReader(f, MaxBytes+(16<<20)+1))
	if n > MaxBytes+(16<<20) {
		e = errors.New("Backup exceeds the 2 GiB limit")
	}
	return n, hex.EncodeToString(h.Sum(nil)), e
}
func validName(n string) bool {
	return n != "" && n != "manifest.json" && n == filepath.Base(n) && !strings.ContainsAny(n, "/\\:\x00") && n != "." && n != ".."
}

// Seal writes only complete archives. The caller atomically publishes the file after success.
func Seal(dst io.Writer, recipient string, m Manifest, files map[string]string) (Manifest, error) {
	r, e := age.ParseX25519Recipient(recipient)
	if e != nil {
		return m, e
	}
	m.Version = 1
	m.Files = nil
	var total int64
	for name, path := range files {
		if !validName(name) {
			return m, errors.New("Invalid archive filename")
		}
		n, h, e := digest(path)
		if e != nil {
			return m, e
		}
		total += n
		if total > MaxBytes {
			return m, errors.New("Backup exceeds the 2 GiB limit")
		}
		m.Files = append(m.Files, File{name, n, h})
	}
	raw, e := json.Marshal(m)
	if e != nil {
		return m, e
	}
	enc, e := age.Encrypt(dst, r)
	if e != nil {
		return m, e
	}
	tw := tar.NewWriter(enc)
	write := func(name string, n int64, src io.Reader) error {
		if e := tw.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: n, Typeflag: tar.TypeReg}); e != nil {
			return e
		}
		_, e := io.CopyN(tw, src, n)
		return e
	}
	if e = write("manifest.json", int64(len(raw)), strings.NewReader(string(raw))); e != nil {
		return m, e
	}
	for _, v := range m.Files {
		f, e := os.Open(files[v.Name])
		if e != nil {
			return m, e
		}
		e = write(v.Name, v.Bytes, f)
		f.Close()
		if e != nil {
			return m, e
		}
	}
	if e = tw.Close(); e != nil {
		return m, e
	}
	return m, enc.Close()
}

// Open authenticates every byte and verifies the complete inventory before returning.
// dir must be a private, empty staging directory. No archive path controls a destination.
func Open(src io.Reader, identity, dir string) (Manifest, error) {
	var m Manifest
	k, e := age.ParseX25519Identity(strings.TrimSpace(identity))
	if e != nil {
		return m, e
	}
	dec, e := age.Decrypt(io.LimitReader(src, MaxBytes+(16<<20)), k)
	if e != nil {
		return m, e
	}
	tr := tar.NewReader(dec)
	h, e := tr.Next()
	if e != nil || h.Name != "manifest.json" || h.Size > 65536 || h.Typeflag != tar.TypeReg {
		return m, errors.New("Invalid recovery manifest")
	}
	if e = json.NewDecoder(io.LimitReader(tr, 65536)).Decode(&m); e != nil {
		return m, e
	}
	if m.Version != 1 || len(m.Files) == 0 || len(m.Files) > 16 {
		return m, errors.New("Unsupported recovery archive")
	}
	expected := map[string]File{}
	var total int64
	for _, v := range m.Files {
		if !validName(v.Name) || v.Bytes < 0 || v.Bytes > MaxBytes || len(v.SHA256) != 64 {
			return m, errors.New("Invalid recovery inventory")
		}
		if _, ok := expected[v.Name]; ok {
			return m, errors.New("Duplicate recovery file")
		}
		expected[v.Name] = v
		total += v.Bytes
	}
	if total > MaxBytes {
		return m, errors.New("Recovery exceeds the 2 GiB limit")
	}
	for {
		h, e = tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return m, e
		}
		v, ok := expected[h.Name]
		if !ok || h.Typeflag != tar.TypeReg || h.Size != v.Bytes {
			return m, errors.New("Unexpected recovery entry")
		}
		f, e := os.OpenFile(filepath.Join(dir, v.Name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return m, e
		}
		hash := sha256.New()
		n, e := io.Copy(io.MultiWriter(f, hash), tr)
		ce := f.Close()
		if e != nil {
			return m, e
		}
		if ce != nil {
			return m, ce
		}
		if n != v.Bytes || hex.EncodeToString(hash.Sum(nil)) != v.SHA256 {
			return m, fmt.Errorf("Recovery checksum failed: %s", v.Name)
		}
		delete(expected, v.Name)
	}
	if len(expected) > 0 {
		return m, errors.New("Recovery archive is incomplete")
	}
	// tar EOF alone does not authenticate the age trailer or reject appended data.
	n, e := io.Copy(io.Discard, io.LimitReader(dec, 1025))
	if e != nil {
		return m, e
	}
	if n != 0 {
		return m, errors.New("Unexpected recovery trailer")
	}
	return m, nil
}
func Checksum(path string) (int64, string, error) { return digest(path) }
func ReadBounded(r io.Reader, limit int64) ([]byte, error) {
	b, e := io.ReadAll(io.LimitReader(r, limit+1))
	if e != nil {
		return nil, e
	}
	if int64(len(b)) > limit {
		return nil, errors.New("Input exceeds its size limit")
	}
	return b, nil
}
