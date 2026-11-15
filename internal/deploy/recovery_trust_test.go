package deploy

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/moby/moby/client"
	"shipper/internal/auth"
	"shipper/internal/events"
	"shipper/internal/store"
)

func TestInstanceRecoveryPreservesDatabaseTrustAndLocalFiles(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	instance, err := db.Setting(ctx, "instance_id", auth.Token())
	if err != nil {
		t.Fatal(err)
	}
	docker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		w.Write([]byte(`{"message":"No such container"}`))
	}))
	defer docker.Close()
	cli, err := client.New(client.WithHost(docker.URL), client.WithAPIVersion("1.54"))
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	runtime := &DockerRuntime{Client: cli, Instance: instance, DataDir: dir}
	original, err := runtime.databaseCertificate("audit-db", "db.example.test")
	if err != nil {
		t.Fatal(err)
	}
	var source bytes.Buffer
	tw := tar.NewWriter(&source)
	content := []byte("immutable local source")
	if err = tw.WriteHeader(&tar.Header{Name: "index.html", Mode: 0644, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	tw.Write(content)
	tw.Close()
	snapshot, err := db.AcceptSource(ctx, bytes.NewReader(source.Bytes()), "owner", "", "audit")
	if err != nil {
		t.Fatal(err)
	}
	app := store.Application{ID: auth.Token()[:24], Name: "Local", Config: store.Config{Method: "static", LocalSource: snapshot.ID, PublishDirectory: "."}}
	if _, err = db.Enqueue(ctx, app, store.Deployment{ID: auth.Token()[:24], ApplicationID: app.ID, Operation: "deploy", Config: app.Config}, "", true); err != nil {
		t.Fatal(err)
	}
	worker := NewWorker(db, runtime, events.New(), dir)
	run := store.BackupRun{ID: auth.Token()[:24], Config: store.BackupSpec{Kind: "instance", Keep: 2}}
	if err = worker.protect(ctx, &run); err != nil {
		t.Fatal(err)
	}
	identity, _, err := worker.RecoveryKey(ctx)
	if err != nil {
		t.Fatal(err)
	}
	keyfile := filepath.Join(t.TempDir(), "recovery.txt")
	if err = os.WriteFile(keyfile, []byte(identity), 0600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "recovered")
	if err = RecoverInstance(ctx, worker.BackupPath(run.ID), keyfile, target); err != nil {
		t.Fatal(err)
	}
	recovered := &DockerRuntime{DataDir: target}
	replacement, err := recovered.databaseCertificate("audit-db", "db.example.test")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original["ca.crt"], replacement["ca.crt"]) {
		t.Fatal("database CA changed on recovery")
	}
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(original["ca.crt"])
	block, _ := pem.Decode(replacement["server.crt"])
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = cert.Verify(x509.VerifyOptions{Roots: roots, DNSName: "db.example.test"}); err != nil {
		t.Fatal("original client trust no longer works", err)
	}
	restored, err := store.Open(target)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if err = restored.VerifyRecoveredSources(ctx); err != nil {
		t.Fatal("local source was lost during final installation", err)
	}
}

func TestDatabaseTrustRecoveryRejectsInvalidIdentities(t *testing.T) {
	runtime := &DockerRuntime{DataDir: t.TempDir()}
	files, err := runtime.databaseCertificate("valid", "")
	if err != nil {
		t.Fatal(err)
	}
	valid := databaseTrust{Application: "valid", Certificate: files["ca.crt"], Key: files["ca.key"]}
	other, err := runtime.databaseCertificate("other", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		entries []databaseTrust
	}{
		{"traversal", []databaseTrust{{Application: "../outside", Certificate: valid.Certificate, Key: valid.Key}}},
		{"duplicate", []databaseTrust{valid, valid}},
		{"case collision", []databaseTrust{valid, {Application: "VALID", Certificate: valid.Certificate, Key: valid.Key}}},
		{"mismatch", []databaseTrust{{Application: "valid", Certificate: valid.Certificate, Key: other["ca.key"]}}},
		{"invalid PEM", []databaseTrust{{Application: "valid", Certificate: []byte("invalid"), Key: valid.Key}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stage := t.TempDir()
			raw, _ := json.Marshal(tc.entries)
			os.WriteFile(filepath.Join(stage, "database-tls.json"), raw, 0600)
			if err := restoreDatabaseTrust(stage); err == nil {
				t.Fatal("invalid trust archive accepted")
			}
			if _, err := os.Stat(filepath.Join(stage, "database-tls")); !os.IsNotExist(err) {
				t.Fatal("invalid bundle partially materialized")
			}
		})
	}
}
