package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProtectionSecretsLocksAndSnapshot(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	a := Application{ID: "backup-app", Name: "Backed up", Config: Config{Method: "image", Image: "nginx:alpine"}}
	d, e := db.Enqueue(ctx, a, Deployment{ID: "first", Operation: "deploy"}, "", true)
	if e != nil {
		t.Fatal(e)
	}
	d.State = "succeeded"
	if e = db.SaveDeployment(ctx, d); e != nil {
		t.Fatal(e)
	}
	c := BackupSpec{Kind: "database", Engine: "postgres", User: "postgres", Database: "db", Password: "secret-backup-password", Keep: 2}
	p := BackupPlan{ID: "p", ApplicationID: a.ID, Name: "Data", Config: c, Enabled: true, Timezone: "UTC"}
	if e = db.SaveBackupPlan(ctx, p); e != nil {
		t.Fatal(e)
	}
	v, e := db.QueueBackup(ctx, BackupRun{PlanID: p.ID, ApplicationID: a.ID, Operation: "backup", Config: c})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Enqueue(ctx, a, Deployment{ID: "concurrent", Operation: "restart"}, "", false); e != ErrBusy {
		t.Fatal("deployment ignored backup lock", e)
	}
	v.State = "succeeded"
	db.SaveBackupRun(ctx, v)
	d, e = db.Enqueue(ctx, a, Deployment{ID: "next", Operation: "restart"}, "", false)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.QueueBackup(ctx, BackupRun{ApplicationID: a.ID, Operation: "backup", Config: c}); e != ErrBusy {
		t.Fatal("backup ignored deployment lock", e)
	}
	attempt, e := db.EnsureTaskRun(ctx, TaskRun{ID: "captured-attempt", DeploymentID: d.ID, Stage: "before_deploy", Command: Command{Argv: []string{"true"}}})
	if e != nil {
		t.Fatal(e)
	}
	attempt.State = "running"
	if e = db.SaveTaskRun(ctx, attempt); e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(t.TempDir(), "shipper.db")
	if e = db.Snapshot(ctx, file); e != nil {
		t.Fatal(e)
	}
	raw, _ := os.ReadFile(file)
	if strings.Contains(string(raw), c.Password) {
		t.Fatal("backup credentials stored in plaintext")
	}
	if e = PrepareRecovery(filepath.Dir(file)); e != nil {
		t.Fatal(e)
	}
	db2, e := Open(filepath.Dir(file))
	if e == nil {
		db2.Close()
		t.Fatal("encrypted database opened without original key")
	}
	key, _ := os.ReadFile(filepath.Join(dir, "encryption.key"))
	os.WriteFile(filepath.Join(filepath.Dir(file), "encryption.key"), key, 0600)
	db2, e = Open(filepath.Dir(file))
	if e != nil {
		t.Fatal(e)
	}
	defer db2.Close()
	plans, e := db2.BackupPlans(ctx, a.ID)
	if e != nil || plans[0].Enabled || plans[0].Config.Password != c.Password {
		t.Fatal("recovery did not retain secrets and pause schedules", plans, e)
	}
	got, e := db2.Deployment(ctx, d.ID)
	if e != nil || got.State != "interrupted" || got.FinishedAt == 0 {
		t.Fatal("captured work was not interrupted", got, e)
	}
	restoredAttempt, e := db2.TaskRun(ctx, a.ID, attempt.ID)
	if e != nil || restoredAttempt.State != "interrupted" || restoredAttempt.FinishedAt == 0 {
		t.Fatal("captured task attempt did not become terminal", restoredAttempt, e)
	}
}
