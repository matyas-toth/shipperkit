package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComposeEncryptionAndRetainedServiceVariables(t *testing.T) {
	dir := t.TempDir()
	db, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	c := Config{Method: "compose", ComposeSource: "paste", ComposeYAML: "services:\n  db:\n    image: postgres\n    environment: {POSTGRES_PASSWORD: inline-sensitive-value}\n", Services: []ServiceConfig{{Name: "db", Variables: []Variable{{Name: "OVERRIDE", Value: "service-sensitive-value", Secret: true, Scope: "runtime"}}}}}
	a := Application{ID: "compose-app", Name: "Compose", Config: c}
	d, e := db.Enqueue(ctx, a, Deployment{ID: "compose-release", Operation: "deploy"}, "", true)
	if e != nil {
		t.Fatal(e)
	}
	d.ComposePlan = `{"project":{"services":{"db":{"environment":{"POSTGRES_PASSWORD":"inline-sensitive-value","OVERRIDE":"service-sensitive-value"}}}}}`
	d.Services = []Service{{Name: "db"}}
	d.State = "succeeded"
	if e = db.SaveDeployment(ctx, d); e != nil {
		t.Fatal(e)
	}
	var cfg, plan string
	if e = db.db.QueryRow("SELECT config_json,compose_plan FROM deployments WHERE id=?", d.ID).Scan(&cfg, &plan); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(cfg+plan, "sensitive-value") || !strings.HasPrefix(plan, "sealed:v1:") {
		t.Fatal("Compose values were persisted as plaintext")
	}
	stored, e := db.Deployment(ctx, d.ID)
	if e != nil {
		t.Fatal(e)
	}
	if stored.ComposePlan != d.ComposePlan || stored.Config.ComposeYAML != c.ComposeYAML {
		t.Fatal("Compose release did not round-trip")
	}
	public := PublicDeployment(stored)
	raw, _ := json.Marshal(public)
	if strings.Contains(string(raw), "sensitive-value") || strings.Contains(string(raw), "composePlan") {
		t.Fatal("Public deployment disclosed Compose source or plan")
	}
	if !public.Config.ComposeKeep || public.Config.ComposeYAML != "" || public.Config.Services[0].Variables[0].Value != "" {
		t.Fatal("Compose retention metadata is missing")
	}
	next := public.Config
	next.Services[0].Variables = append(next.Services[0].Variables, Variable{Name: "RENAMED_PASSWORD", Keep: true, KeepFrom: "POSTGRES_PASSWORD", Secret: true, Scope: "runtime"})
	if e = MergeReleaseVariables(&next, c, stored); e != nil {
		t.Fatal(e)
	}
	if next.ComposeYAML != c.ComposeYAML || next.Services[0].Variables[1].Value != "inline-sensitive-value" {
		t.Fatal("Inherited Compose variable could not be renamed without disclosure")
	}
	db.Close()
	os.Remove(filepath.Join(dir, "encryption.key"))
	if reopened, e := Open(dir); e == nil {
		reopened.Close()
		t.Fatal("Missing Compose encryption key was replaced")
	}
}

func TestComposeInventoryRequiresAllActiveServices(t *testing.T) {
	db, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	a := Application{ID: "stack", Name: "Stack", Config: Config{Method: "compose"}}
	d, e := db.Enqueue(ctx, a, Deployment{ID: "stack-release", Operation: "deploy"}, "", true)
	if e != nil {
		t.Fatal(e)
	}
	d.Services = []Service{{Name: "web"}, {Name: "db"}, {Name: "migrate", OneShot: true}}
	d.State = "succeeded"
	if e = db.SaveDeployment(ctx, d); e != nil {
		t.Fatal(e)
	}
	if e = db.SetActive(ctx, a.ID, d.ID); e != nil {
		t.Fatal(e)
	}
	web := Container{ID: "web", Name: "Web", ApplicationID: a.ID, Managed: true, Project: "stable-project", Service: "web", State: "running"}
	database := web
	database.ID = "db"
	database.Service = "db"
	migration := web
	migration.ID = "migrate"
	migration.Service = "migrate"
	migration.State = "exited"
	migration.Status = "Exited (0) 2 minutes ago"
	if e = db.ReplaceInventory(ctx, []Container{web, database, migration}); e != nil {
		t.Fatal(e)
	}
	got, e := db.Application(ctx, a.ID)
	if e != nil || got.State != "running" {
		t.Fatal("Completed dependency degraded a healthy stack", got.State, e)
	}
	if e = db.ReplaceInventory(ctx, []Container{web, migration}); e != nil {
		t.Fatal(e)
	}
	got, e = db.Application(ctx, a.ID)
	if e != nil || got.State != "degraded" {
		t.Fatal("Missing service was reported as a healthy stack", got.State, e)
	}
}
