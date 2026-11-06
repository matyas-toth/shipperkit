package store

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func sourceTar(t *testing.T, name, content string, kind byte) *bytes.Buffer {
	t.Helper()
	b := new(bytes.Buffer)
	w := tar.NewWriter(b)
	h := &tar.Header{Name: name, Typeflag: kind, Mode: 0755, Size: int64(len(content))}
	if kind != tar.TypeReg {
		h.Size = 0
		h.Linkname = "outside"
	}
	if e := w.WriteHeader(h); e != nil {
		t.Fatal(e)
	}
	if kind == tar.TypeReg {
		w.Write([]byte(content))
	}
	w.Close()
	return b
}
func TestObservationsDeduplicateAndRecoverDurably(t *testing.T) {
	dir := t.TempDir()
	db, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	o := Observation{ID: "daemon", Kind: "daemon", State: "failed", Summary: "Docker is unavailable"}
	for i := 0; i < 4; i++ {
		if e = db.Observe(ctx, o); e != nil {
			t.Fatal(e)
		}
	}
	events, e := db.OperationEvents(ctx, "", 0)
	if e != nil || len(events) != 1 {
		t.Fatal(events, e)
	}
	notification, n, e := db.NextNotification(ctx)
	if e != nil || n != 0 || notification.State != "failed" {
		t.Fatal(notification, e)
	}
	db.Close()
	db, e = Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = db.Observe(ctx, o); e != nil {
		t.Fatal(e)
	}
	events, _ = db.OperationEvents(ctx, "", 0)
	if len(events) != 1 {
		t.Fatal("Restart duplicated failure")
	}
	o.State = "healthy"
	o.Summary = "Docker is connected"
	if e = db.Observe(ctx, o); e != nil {
		t.Fatal(e)
	}
	events, _ = db.OperationEvents(ctx, "", 0)
	if len(events) != 2 {
		t.Fatal(events)
	}
	if e = db.FinishNotification(ctx, notification.Sequence, 0, true); e != nil {
		t.Fatal(e)
	}
	next, _, e := db.NextNotification(ctx)
	if e != nil || next.State != "healthy" {
		t.Fatal("Recovery was not queued", next, e)
	}
}
func TestFailedNotificationInspectionAndRetry(t *testing.T) {
	db, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	if e = db.Observe(ctx, Observation{ID: "proxy", Kind: "proxy", State: "failed", Summary: "Proxy is unavailable"}); e != nil {
		t.Fatal(e)
	}
	event, _, e := db.NextNotification(ctx)
	if e != nil {
		t.Fatal(e)
	}
	for attempt := 0; attempt < 5; attempt++ {
		if e = db.FinishNotification(ctx, event.Sequence, attempt, false, "Webhook returned HTTP 503"); e != nil {
			t.Fatal(e)
		}
	}
	items, e := db.FailedNotifications(ctx)
	if e != nil || len(items) != 1 || items[0].Error != "Webhook returned HTTP 503" || items[0].Attempts != 5 {
		t.Fatal(items, e)
	}
	if e = db.RetryNotification(ctx, event.Sequence); e != nil {
		t.Fatal(e)
	}
	// Retrying an already queued delivery must not reset an in-flight attempt.
	if e = db.RetryNotification(ctx, event.Sequence); e == nil {
		t.Fatal("Duplicate retry accepted")
	}
	next, attempts, e := db.NextNotification(ctx)
	if e != nil || next.Sequence != event.Sequence || attempts != 0 {
		t.Fatal(next, attempts, e)
	}
	if e = db.FinishNotification(ctx, next.Sequence, attempts, true); e != nil {
		t.Fatal(e)
	}
	if e = db.RetryNotification(ctx, next.Sequence); e == nil {
		t.Fatal("Delivered notification retried")
	}
	items, e = db.FailedNotifications(ctx)
	if e != nil || len(items) != 0 {
		t.Fatal(items, e)
	}
}

func TestLocalSourceIntegrityAndRecovery(t *testing.T) {
	dir := t.TempDir()
	db, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	for _, name := range []string{"../escape", "/absolute", ".env", ".ssh/id_rsa", "C:/escape"} {
		if _, e = db.AcceptSource(ctx, sourceTar(t, name, "secret", tar.TypeReg), "owner", "", ""); e == nil {
			t.Fatalf("Accepted unsafe path %s", name)
		}
	}
	if _, e = db.AcceptSource(ctx, sourceTar(t, "link", "", tar.TypeSymlink), "owner", "", ""); e == nil {
		t.Fatal("Accepted symlink")
	}
	v, e := db.AcceptSource(ctx, sourceTar(t, "index.html", "immutable source", tar.TypeReg), "owner", "", "commit dirty")
	if e != nil {
		t.Fatal(e)
	}
	target := t.TempDir()
	digest, e := db.MaterializeSource(ctx, v.ID, target)
	if e != nil || digest != v.Digest {
		t.Fatal(digest, e)
	}
	raw, e := os.ReadFile(filepath.Join(target, "repository", "index.html"))
	if e != nil || string(raw) != "immutable source" {
		t.Fatal(string(raw), e)
	}
	a := Application{ID: "local-app", Name: "Local", Config: Config{Method: "static", LocalSource: v.ID}}
	d, e := db.Enqueue(ctx, a, Deployment{ID: "local-release", Operation: "deploy"}, "", true)
	if e != nil {
		t.Fatal(e)
	}
	if d.Config.LocalSource != v.ID {
		t.Fatal(d)
	}
	removed, e := db.RemoveAbandonedSource(ctx, v.ID)
	if e != nil || removed {
		t.Fatal("Referenced source removed", e)
	}
	snapshot := filepath.Join(t.TempDir(), "snapshot.db")
	if e = db.Snapshot(ctx, snapshot); e != nil {
		t.Fatal(e)
	}
	bundle := filepath.Join(t.TempDir(), "sources.tar")
	if e = db.BundleSources(ctx, snapshot, bundle); e != nil {
		t.Fatal(e)
	}
	recovery := t.TempDir()
	data, _ := os.ReadFile(bundle)
	os.WriteFile(filepath.Join(recovery, "sources.tar"), data, 0600)
	if e = RestoreSourceBundle(recovery); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(recovery, "sources", v.ID+".tar")); e != nil {
		t.Fatal(e)
	}
	for _, pair := range [][2]string{{snapshot, filepath.Join(recovery, "shipper.db")}, {filepath.Join(dir, "encryption.key"), filepath.Join(recovery, "encryption.key")}} {
		data, e := os.ReadFile(pair[0])
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(pair[1], data, 0600); e != nil {
			t.Fatal(e)
		}
	}
	if e = PrepareRecovery(recovery); e != nil {
		t.Fatal(e)
	}
	recovered, e := Open(recovery)
	if e != nil {
		t.Fatal(e)
	}
	if e = recovered.VerifyRecoveredSources(ctx); e != nil {
		t.Fatal(e)
	}
	recovered.Close()
	source, _ := db.SourcePath(v.ID)
	os.WriteFile(source, []byte("tampered"), 0600)
	if _, e = db.MaterializeSource(ctx, v.ID, t.TempDir()); e == nil {
		t.Fatal("Tampered source accepted")
	}
}
func TestOperationTokenExpiryAndScope(t *testing.T) {
	db, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	if e = db.CreateAdmin(ctx, User{ID: "owner", Email: "owner@example.test"}, "hash", "csrf", time.Now().Add(time.Hour).Unix()); e != nil {
		t.Fatal(e)
	}
	v := OperationToken{ID: "token", Name: "Reader", Hash: "expired", UserID: "owner", Scopes: []string{"read"}, Applications: []string{"one"}, ExpiresAt: time.Now().Add(-time.Hour).Unix()}
	if e = db.SaveOperationToken(ctx, v); e != nil {
		t.Fatal(e)
	}
	if _, e = db.OperationToken(ctx, "raw"); e == nil {
		t.Fatal("Expired token accepted")
	}
	if !v.Allows("read", "one") || v.Allows("deploy", "one") || v.Allows("read", "two") || v.Allows("read", "") {
		t.Fatal("Scope isolation failed")
	}
}

func TestLogRetentionProtectsServingAndRecentOperations(t *testing.T) {
	db, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	a := Application{ID: "log-app", Name: "Logs", Config: Config{Method: "static"}}
	for i := 0; i < 14; i++ {
		id := fmt.Sprintf("log-%02d", i)
		d, e := db.Enqueue(ctx, a, Deployment{ID: id, Operation: "deploy"}, "", i == 0)
		if e != nil {
			t.Fatal(e)
		}
		d.State = "succeeded"
		d.FinishedAt = time.Now().Add(-45 * 24 * time.Hour).Unix()
		if e = db.SaveDeployment(ctx, d); e != nil {
			t.Fatal(e)
		}
		if e = db.AppendLog(ctx, id, "checking", "retained evidence"); e != nil {
			t.Fatal(e)
		}
	}
	if e = db.SetActive(ctx, a.ID, "log-00"); e != nil {
		t.Fatal(e)
	}
	cutoff := time.Now().Add(-30 * 24 * time.Hour).Unix()
	count, _, e := db.LogRetention(ctx, cutoff)
	if e != nil || count != 3 {
		t.Fatal(count, e)
	}
	if e = db.RetainLogs(ctx, cutoff); e != nil {
		t.Fatal(e)
	}
	logs, _ := db.Logs(ctx, "log-00", 0)
	if len(logs) != 1 {
		t.Fatal("Serving release logs removed")
	}
	logs, _ = db.Logs(ctx, "log-13", 0)
	if len(logs) != 1 {
		t.Fatal("Recent operation logs removed")
	}
	logs, _ = db.Logs(ctx, "log-01", 0)
	if len(logs) != 0 {
		t.Fatal("Old eligible log not removed")
	}
}
