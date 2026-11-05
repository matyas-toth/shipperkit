package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"shipper/internal/auth"
	"shipper/internal/cli"
	"shipper/internal/deploy"
	"shipper/internal/events"
	"shipper/internal/httpapi"
	"shipper/internal/inventory"
	"shipper/internal/store"
)

func TestLocalPlanApplyAndLostResponseReplay(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, e := store.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = db.CreateAdmin(ctx, store.User{ID: "owner", Email: "owner@example.test"}, "hash", "csrf", time.Now().Add(time.Hour).Unix()); e != nil {
		t.Fatal(e)
	}
	raw := auth.Token()
	if e = db.SaveOperationToken(ctx, store.OperationToken{ID: "cli", Name: "CLI", Hash: auth.Hash(raw), UserID: "owner", Scopes: []string{"read", "deploy"}, ExpiresAt: time.Now().Add(time.Hour).Unix()}); e != nil {
		t.Fatal(e)
	}
	hub := events.New()
	worker := deploy.NewWorker(db, nil, hub, dir)
	server := httptest.NewServer(httpapi.New(db, inventory.New(nil, db, hub), hub, httpapi.Config{Worker: worker, Assets: http.NotFoundHandler()}))
	defer server.Close()
	t.Setenv("SHIPPER_CLIENT_DIR", t.TempDir())
	t.Setenv("SHIPPER_TOKEN", raw)
	run := func(args ...string) map[string]any {
		t.Helper()
		var out bytes.Buffer
		if e := cli.Run(append(args, "--json"), &out); e != nil {
			t.Fatalf("%v: %v", args, e)
		}
		var result map[string]any
		if e = json.Unmarshal(out.Bytes(), &result); e != nil {
			t.Fatal(out.String(), e)
		}
		return result
	}
	run("connect", server.URL)
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "index.html"), []byte("reviewed content"), 0600)
	os.WriteFile(filepath.Join(root, ".env"), []byte("PRIVATE=not-uploaded"), 0600)
	plan := run("plan", root, "--name", "CLI fixture", "--method", "static")
	if int(plan["files"].(float64)) != 1 {
		t.Fatal("Secret file was included", plan)
	}
	os.WriteFile(filepath.Join(root, "index.html"), []byte("changed after review"), 0600)
	applied := run("deploy", "--plan", plan["planId"].(string))
	id := applied["applicationId"].(string)
	operation := applied["operationId"].(string)
	d, e := db.Deployment(ctx, operation)
	if e != nil {
		t.Fatal(e)
	}
	target := t.TempDir()
	if _, e = db.MaterializeSource(ctx, d.Config.LocalSource, target); e != nil {
		t.Fatal(e)
	}
	content, _ := os.ReadFile(filepath.Join(target, "repository", "index.html"))
	if string(content) != "reviewed content" {
		t.Fatal("Apply used changed working tree", string(content))
	}
	complete := func(id string) {
		t.Helper()
		d, _ := db.Deployment(ctx, id)
		d.State = "succeeded"
		d.ImageID = "sha256:fixture"
		d.FinishedAt = time.Now().Unix()
		if e = db.SaveDeployment(ctx, d); e != nil {
			t.Fatal(e)
		}
		if e = db.SetActive(ctx, d.ApplicationID, id); e != nil {
			t.Fatal(e)
		}
	}
	complete(operation)
	again := run("deploy", "--plan", plan["planId"].(string))
	if again["operationId"] != operation {
		t.Fatal("New application replay created duplicate work")
	}
	next := run("plan", root)
	applied = run("deploy", "--plan", next["planId"].(string))
	operation = applied["operationId"].(string)
	complete(operation)
	again = run("deploy", "--plan", next["planId"].(string))
	if again["operationId"] != operation {
		t.Fatal("Reviewed update replay created duplicate work")
	}
	report := run("doctor", id)
	if report["servingRelease"] == nil {
		t.Fatal(report)
	}
}
