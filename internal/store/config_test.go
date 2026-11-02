package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncryptedConfigurationMaskingAndKeyRecovery(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	a := Application{ID: "encrypted", Name: "Encrypted", Config: Config{Repository: "https://github.com/example/app", Domain: "encrypted.localhost", Variables: []Variable{{Name: "TOKEN", Value: "a-secret-value-with-entropy", Secret: true, Scope: "both"}, {Name: "PUBLIC", Value: "visible-value", Scope: "runtime"}}}}
	d, err := db.Enqueue(ctx, a, Deployment{ID: "encrypted-job", Operation: "deploy"}, "same-request", true)
	if err != nil {
		t.Fatal(err)
	}
	var raw string
	if err = db.db.QueryRow("SELECT config_json FROM applications WHERE id=?", a.ID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "a-secret-value-with-entropy") || strings.Contains(raw, "visible-value") || !strings.Contains(raw, "sealed:v1:") {
		t.Fatal("configuration was not encrypted")
	}
	current, err := db.Application(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	public := PublicApplication(current)
	if public.Config.Variables[0].Value != "" || !public.Config.Variables[0].Keep || public.Config.Variables[1].Value != "visible-value" {
		t.Fatal("public configuration masking failed")
	}
	if current.Config.Variables[0].Value != "a-secret-value-with-entropy" {
		t.Fatal("masking mutated saved configuration")
	}
	renamed := PublicConfig(current.Config)
	renamed.Variables[0].Name = "RENAMED_TOKEN"
	renamed.Variables[0].KeepFrom = "TOKEN"
	if err = MergeVariables(&renamed, current.Config); err != nil || renamed.Variables[0].Value != current.Config.Variables[0].Value {
		t.Fatal("masked rename lost its value", err)
	}
	merged := public.Config
	if err = MergeVariables(&merged, current.Config); err != nil || merged.Variables[0].Value != current.Config.Variables[0].Value {
		t.Fatal("masked round trip failed", err)
	}
	if _, err = db.Enqueue(ctx, a, Deployment{ID: "changed", Operation: "deploy", NoCache: true}, "same-request", true); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatal("cache option missing from request identity", err)
	}
	if PublicDeployment(d).Config.Variables[0].Value != "" {
		t.Fatal("deployment response exposed a secret")
	}
	db.Close()
	db, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	again, err := db.Deployment(ctx, d.ID)
	if err != nil || again.Config.Variables[0].Value != a.Config.Variables[0].Value {
		t.Fatal("encrypted restart failed", err)
	}
	db.Close()
	keyPath := filepath.Join(dir, "encryption.key")
	key, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(keyPath); err != nil {
		t.Fatal(err)
	}
	if opened, err := Open(dir); err == nil {
		opened.Close()
		t.Fatal("missing key silently replaced")
	}
	if err = os.WriteFile(keyPath, key, 0600); err != nil {
		t.Fatal(err)
	}
	db, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
}

func TestDomainClaimsFollowAppliedRoutes(t *testing.T) {
	db, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	a := Application{ID: "first", Name: "First", Config: Config{Domain: "first.localhost"}}
	first, e := db.Enqueue(ctx, a, Deployment{ID: "initial", Operation: "deploy"}, "", true)
	if e != nil {
		t.Fatal(e)
	}
	first.State = "succeeded"
	db.SaveDeployment(ctx, first)
	a, _ = db.Application(ctx, a.ID)
	a.Config.Domains = []Domain{{ID: "alias", Hostname: "alias.localhost", Primary: true}}
	change, e := db.Enqueue(ctx, a, Deployment{ID: "routes", Operation: "domains"}, "", false)
	if e != nil {
		t.Fatal(e)
	}
	b := Application{ID: "second", Name: "Second", Config: Config{Domain: "alias.localhost"}}
	if _, e = db.Enqueue(ctx, b, Deployment{ID: "conflict", Operation: "deploy"}, "", true); !errors.Is(e, ErrDomainInUse) {
		t.Fatal("pending hostname was not reserved", e)
	}
	if e = db.SetDomains(ctx, a.ID, change.Config.Domains); e != nil {
		t.Fatal(e)
	}
	if e = db.ReleaseDomainClaims(ctx, a.ID); e != nil {
		t.Fatal(e)
	}
	b.Config.Domain = "first.localhost"
	if _, e = db.Enqueue(ctx, b, Deployment{ID: "released", Operation: "deploy"}, "", true); e != nil {
		t.Fatal("old hostname was not released", e)
	}
}
