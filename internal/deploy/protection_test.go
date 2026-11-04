package deploy

import (
	"context"
	"testing"
	"time"

	"shipper/internal/events"
	"shipper/internal/store"
)

func TestBackupSchedulesAdvanceOnceAndDoNotReplayInterruptedRestores(t *testing.T) {
	ctx := context.Background()
	db, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	w := NewWorker(db, nil, events.New(), t.TempDir())
	p := store.BackupPlan{ID: "scheduled", Name: "Instance", Enabled: true, Schedule: "0 3 * * *", Timezone: "Europe/Budapest", NextAt: time.Now().Add(-48 * time.Hour).Unix(), Config: store.BackupSpec{Kind: "instance", Keep: 2}}
	if e = db.SaveBackupPlan(ctx, p); e != nil {
		t.Fatal(e)
	}
	disabled := p
	disabled.ID = "disabled"
	disabled.Enabled = false
	if e = db.SaveBackupPlan(ctx, disabled); e != nil {
		t.Fatal(e)
	}
	w.ScheduleBackups(ctx)
	w.ScheduleBackups(ctx)
	runs, e := db.BackupRuns(ctx, "")
	if e != nil || len(runs) != 1 {
		t.Fatal("missed occurrences were duplicated", len(runs), e)
	}
	plans, e := db.BackupPlans(ctx, "")
	if e != nil || plans[0].NextAt <= time.Now().Unix() {
		t.Fatal("schedule did not advance", plans, e)
	}
	v := runs[0]
	v.State = "succeeded"
	if e = db.SaveBackupRun(ctx, v); e != nil {
		t.Fatal(e)
	}
	v, e = db.QueueBackup(ctx, store.BackupRun{Operation: "restore", Config: p.Config, SourceID: v.ID})
	if e != nil {
		t.Fatal(e)
	}
	v.State = "running"
	if e = db.SaveBackupRun(ctx, v); e != nil {
		t.Fatal(e)
	}
	if e = db.InterruptBackups(ctx); e != nil {
		t.Fatal(e)
	}
	v, e = db.BackupRun(ctx, v.ID)
	if e != nil || v.State != "interrupted" {
		t.Fatal("interrupted operation was not fenced", v, e)
	}
	if _, e = db.PendingBackup(ctx); e == nil {
		t.Fatal("an uncertain operation was automatically replayed")
	}
}

func TestBackupConnectionDoesNotAcceptLibpqConnectionStrings(t *testing.T) {
	for _, name := range []string{"postgres://evil/db", "host=evil password=value", "--all-databases"} {
		spec := store.BackupSpec{Kind: "database", Engine: "postgres", Database: name, User: "postgres"}
		if e := ValidateBackupSpec(&spec); e == nil {
			t.Fatal("database escaped the selected loopback connection", name)
		}
	}
	spec := store.BackupSpec{Kind: "database", Engine: "postgres", Database: "app-prod", User: "postgres"}
	if e := ValidateBackupSpec(&spec); e != nil {
		t.Fatal("ordinary hyphenated database rejected", e)
	}
}
