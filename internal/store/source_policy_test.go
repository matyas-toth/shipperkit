package store

import (
	"archive/tar"
	"bytes"
	"context"
	"testing"
)

func TestLocalSourceRejectsCredentialFilesAtTheAPIBoundary(t *testing.T) {
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, name := range []string{".npmrc", "nested/.netrc", ".pypirc", ".yarnrc.yml", ".docker/config.json", ".git-credentials"} {
		var buf bytes.Buffer
		w := tar.NewWriter(&buf)
		w.WriteHeader(&tar.Header{Name: name, Size: 6, Mode: 0600})
		w.Write([]byte("secret"))
		w.Close()
		if _, err := db.AcceptSource(context.Background(), bytes.NewReader(buf.Bytes()), "owner", "", "test"); err == nil {
			t.Fatalf("accepted credential file %s", name)
		}
	}
}
