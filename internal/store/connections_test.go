package store

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestConnectionsEncryptionCancellationAndWebhookDurability(t *testing.T) {
	db, e := Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	c := Connection{ID: "source", Name: "GitHub", Kind: "github_token", Host: "github.com", Secret: ConnectionSecret{Token: "private-source-token", WebhookSecret: "private-hook-secret"}}
	if e = db.SaveConnection(ctx, c); e != nil {
		t.Fatal(e)
	}
	var raw string
	db.db.QueryRow("SELECT secret FROM connections").Scan(&raw)
	if strings.Contains(raw, "private-") {
		t.Fatal("plaintext credentials")
	}
	got, e := db.Connection(ctx, c.ID)
	if e != nil || got.Secret.Token != c.Secret.Token {
		t.Fatal(got, e)
	}
	a := Application{ID: "app", Name: "App", Config: Config{SourceID: c.ID, Repository: "https://github.com/a/b"}}
	d, e := db.Enqueue(ctx, a, Deployment{ID: "old", Operation: "deploy", Commit: strings.Repeat("a", 40)}, "", true)
	if e != nil {
		t.Fatal(e)
	}
	loaded, _ := db.Deployment(ctx, d.ID)
	if loaded.Commit != d.Commit {
		t.Fatal("pinned revision was lost")
	}
	if db.DeleteConnection(ctx, c.ID) == nil {
		t.Fatal("deleted active source")
	}
	if e = db.CancelDeployment(ctx, a.ID, d.ID); e != nil {
		t.Fatal(e)
	}
	pending, _ := db.Pending(ctx)
	if len(pending) != 0 {
		t.Fatal("cancelled work stayed pending")
	}
	d, e = db.Enqueue(ctx, a, Deployment{ID: "next", Operation: "deploy"}, "", false)
	if e != nil {
		t.Fatal(e)
	}
	d.State = "building"
	if e = db.SaveDeployment(ctx, d); e != nil {
		t.Fatal(e)
	}
	if e = db.CancelDeployment(ctx, a.ID, d.ID); e != nil {
		t.Fatal(e)
	}
	d.State = "before_deploy"
	if !errors.Is(db.SaveDeployment(ctx, d), ErrCancelled) {
		t.Fatal("cancelled build entered migration")
	}
	d.State = "cancelled"
	db.SaveDeployment(ctx, d)
	intent := DeployIntent{ApplicationID: a.ID, Revision: strings.Repeat("b", 40), Actor: "github"}
	if accepted, e := db.AcceptWebhook(ctx, c.ID, "delivery", []DeployIntent{intent}); e != nil || !accepted {
		t.Fatal(e)
	}
	if accepted, e := db.AcceptWebhook(ctx, c.ID, "delivery", []DeployIntent{intent}); e != nil || accepted {
		t.Fatal("delivery was replayed", e)
	}
	jobs, _ := db.DeployIntents(ctx)
	if len(jobs) != 1 || jobs[0].Revision != intent.Revision {
		t.Fatal(jobs)
	}
}
