package protection

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"
)

func TestArchiveAuthenticatedRoundTrip(t *testing.T) {
	key, recipient, e := Identity()
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "data")
	os.WriteFile(file, []byte("private original data"), 0600)
	var archive bytes.Buffer
	_, e = Seal(&archive, recipient, Manifest{Kind: "instance", Instance: "host", Consistency: "snapshot"}, map[string]string{"shipper.db": file})
	if e != nil {
		t.Fatal(e)
	}
	original := append([]byte{}, archive.Bytes()...)
	m, e := Open(bytes.NewReader(original), key, t.TempDir())
	if e != nil || m.Instance != "host" {
		t.Fatal(m, e)
	}
	other, _, _ := Identity()
	for name, input := range map[string][]byte{"truncated": original[:len(original)-10], "corrupt": append([]byte{}, original...)} {
		if name == "corrupt" {
			input[len(input)-15] ^= 1
		}
		t.Run(name, func(t *testing.T) {
			if _, e := Open(bytes.NewReader(input), key, t.TempDir()); e == nil {
				t.Fatal("corrupt archive accepted")
			}
		})
	}
	if _, e = Open(bytes.NewReader(original), other, t.TempDir()); e == nil {
		t.Fatal("wrong recovery key accepted")
	}
	if bytes.Contains(original, []byte("private original data")) {
		t.Fatal("plaintext archive")
	}
}
func TestArchiveRejectsInventoriesAndPaths(t *testing.T) {
	key, recipient, _ := Identity()
	r, _ := age.ParseX25519Recipient(recipient)
	for _, name := range []string{"../outside", "C:\\outside", "file", "manifest.json"} {
		t.Run(name, func(t *testing.T) {
			var b bytes.Buffer
			enc, _ := age.Encrypt(&b, r)
			tw := tar.NewWriter(enc)
			m := Manifest{Version: 1, Kind: "volume", Files: []File{{Name: "file", Bytes: 1, SHA256: "invalid"}}}
			raw, _ := json.Marshal(m)
			tw.WriteHeader(&tar.Header{Name: "manifest.json", Mode: 0600, Size: int64(len(raw)), Typeflag: tar.TypeReg})
			tw.Write(raw)
			tw.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: 1, Typeflag: tar.TypeReg})
			io.WriteString(tw, "x")
			tw.Close()
			enc.Close()
			if _, e := Open(&b, key, t.TempDir()); e == nil {
				t.Fatal("unsafe archive accepted")
			}
		})
	}
}
