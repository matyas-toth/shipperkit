package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTaskEncryptionSnapshotsAndDesiredConfig(t *testing.T) {
	dir := t.TempDir()
	db, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	a := Application{ID: "app", Name: "App", Config: Config{Repository: "https://github.com/example/app", Hooks: []Hook{{Stage: "before_deploy", Command: Command{Shell: "a-private-command"}}}}}
	d, e := db.Enqueue(ctx, a, Deployment{ID: "release", Operation: "deploy"}, "", true)
	if e != nil {
		t.Fatal(e)
	}
	d.State = "succeeded"
	db.SaveDeployment(ctx, d)
	db.SetActive(ctx, a.ID, d.ID)
	a, _ = db.Application(ctx, a.ID)
	a.Config.BuildCommand = "pending build"
	db.SaveConfig(ctx, a.ID, a.Config)
	task := Task{ID: "task", ApplicationID: a.ID, Name: "Migration", Command: Command{Shell: "a-private-command", Timeout: 30, Mode: "job"}, Enabled: true, Schedule: "* * * * *", Timezone: "UTC", Overlap: "skip", Missed: "once", NextAt: time.Now().Unix()}
	if e = db.SaveTask(ctx, task); e != nil {
		t.Fatal(e)
	}
	active := a
	active.Config = d.Config
	run, e := db.Enqueue(ctx, active, Deployment{ID: "operation", Operation: "task", TaskID: task.ID, TaskCommand: &task.Command, Actor: "owner@example.test"}, "manual-task", false)
	if e != nil {
		t.Fatal(e)
	}
	got, _ := db.Application(ctx, a.ID)
	if got.Config.BuildCommand != "pending build" {
		t.Fatal("task overwrote pending configuration")
	}
	snapshot, _ := db.Deployment(ctx, run.ID)
	if snapshot.TaskCommand.Shell != task.Command.Shell || snapshot.Actor != "owner@example.test" {
		t.Fatal(snapshot)
	}
	var raw string
	for _, query := range []string{"SELECT command FROM tasks", "SELECT task_command FROM deployments WHERE id='operation'", "SELECT config_json FROM applications"} {
		if e = db.db.QueryRow(query).Scan(&raw); e != nil {
			t.Fatal(e)
		}
		if strings.Contains(raw, "a-private-command") {
			t.Fatal("plaintext command at rest", query)
		}
	}
	r, e := db.EnsureTaskRun(ctx, TaskRun{ID: "attempt", DeploymentID: run.ID, Stage: "task", Command: task.Command})
	if e != nil {
		t.Fatal(e)
	}
	r.ContainerID = "container"
	r.State = "running"
	db.SaveTaskRun(ctx, r)
	r2, e := db.EnsureTaskRun(ctx, TaskRun{ID: "different", DeploymentID: run.ID, Stage: "task", Command: Command{Shell: "changed"}})
	if e != nil || r2.ID != r.ID || r2.Command.Shell != task.Command.Shell || r2.ContainerID != "container" {
		t.Fatal("lost attempt", r2, e)
	}
	due, e := db.DueTasks(ctx, time.Now().Unix())
	if e != nil || len(due) != 1 {
		t.Fatal(due, e)
	}
	db.Close()
	if e = os.Remove(filepath.Join(dir, "encryption.key")); e != nil {
		t.Fatal(e)
	}
	if reopened, e := Open(dir); e == nil {
		reopened.Close()
		t.Fatal("accepted lost task encryption key")
	}
}
