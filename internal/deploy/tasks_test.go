package deploy

import (
	"context"
	"errors"
	"shipper/internal/events"
	"shipper/internal/store"
	"strings"
	"testing"
	"time"
)

type taskFixtureRuntime struct {
	fakeRuntime
	calls       []string
	failStage   string
	interrupted bool
}

func (f *taskFixtureRuntime) RunTask(ctx context.Context, d store.Deployment, r *store.TaskRun, save func() error, log func(string)) error {
	f.calls = append(f.calls, r.Stage)
	r.ContainerID = "job-" + r.ID
	r.State = "running"
	if e := save(); e != nil {
		return e
	}
	log("token=" + d.Config.Variables[0].Value)
	if f.interrupted {
		return ErrUncertain
	}
	if r.Stage == f.failStage {
		r.ExitCode = 41
		return errors.New("migration failed")
	}
	r.ExitCode = 0
	return nil
}
func (f *taskFixtureRuntime) CleanupTask(context.Context, store.Deployment, store.TaskRun) error {
	return nil
}
func TestTaskOrderingFailureAndRecovery(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failure"}[fail], func(t *testing.T) {
			dir := t.TempDir()
			db, e := store.Open(dir)
			if e != nil {
				t.Fatal(e)
			}
			defer db.Close()
			ctx := context.Background()
			runtime := &taskFixtureRuntime{}
			if fail {
				runtime.failStage = "before_deploy"
			}
			w := NewWorker(db, runtime, events.New(), dir)
			w.Source = fixtureSource{commit: strings.Repeat("a", 40)}
			a := store.Application{ID: "tasks-app", Name: "Tasks", Config: testConfig()}
			a.Config.Variables = []store.Variable{{Name: "TOKEN", Value: "secret-task-value", Secret: true, Scope: "runtime"}}
			a.Config.Hooks = []store.Hook{{Stage: "before_deploy", Command: store.Command{Argv: []string{"migrate"}, Timeout: 30, Mode: "job"}}, {Stage: "after_deploy", Command: store.Command{Shell: "notify", Timeout: 30, Mode: "job"}}}
			d, e := db.Enqueue(ctx, a, store.Deployment{ID: "first", Operation: "deploy"}, "", true)
			if e != nil {
				t.Fatal(e)
			}
			w.process(ctx, d)
			got, e := db.Deployment(ctx, d.ID)
			if e != nil {
				t.Fatal(e)
			}
			a, _ = db.Application(ctx, a.ID)
			if fail {
				if got.State != "failed" || a.ActiveDeploymentID != "" || len(runtime.calls) != 1 {
					t.Fatal(got, runtime.calls)
				}
			} else {
				if got.State != "succeeded" || a.ActiveDeploymentID != d.ID || strings.Join(runtime.calls, ",") != "before_deploy,after_deploy" {
					t.Fatal(got, runtime.calls)
				}
				w.process(ctx, got)
				if len(runtime.calls) != 2 {
					t.Fatal("completed migration was repeated", runtime.calls)
				}
			}
			lines, _ := db.Logs(ctx, d.ID, 0)
			for _, l := range lines {
				if strings.Contains(l.Line, "secret-task-value") {
					t.Fatal("secret leaked")
				}
			}
		})
	}
}
func TestAmbiguousTaskDoesNotRetry(t *testing.T) {
	dir := t.TempDir()
	db, _ := store.Open(dir)
	defer db.Close()
	ctx := context.Background()
	a := store.Application{ID: "app", Name: "App", Config: testConfig()}
	a.Config.Variables = []store.Variable{{Name: "TOKEN", Value: "secret", Secret: true}}
	d, e := db.Enqueue(ctx, a, store.Deployment{ID: "run", Operation: "deploy"}, "", true)
	if e != nil {
		t.Fatal(e)
	}
	runtime := &taskFixtureRuntime{interrupted: true}
	w := NewWorker(db, runtime, events.New(), dir)
	c := store.Command{Argv: []string{"migrate"}, Timeout: 300, Mode: "job"}
	if !errors.Is(w.runTask(ctx, d, "before_deploy", c), ErrUncertain) {
		t.Fatal("expected uncertainty")
	}
	if w.runTask(ctx, d, "before_deploy", c) == nil {
		t.Fatal("uncertain task accepted")
	}
	if len(runtime.calls) != 1 {
		t.Fatal("retried ambiguous task")
	}
	runs, _ := db.TaskRuns(ctx, a.ID)
	if len(runs) != 1 || runs[0].State != "uncertain" {
		t.Fatal(runs)
	}
}
func TestConfigurationOperationPlan(t *testing.T) {
	base := store.Config{Method: "dockerfile", Repository: "https://github.com/example/app", Variables: []store.Variable{{Name: "TOKEN", Value: "old", Secret: true, Scope: "runtime"}}}
	next := base
	next.Variables = []store.Variable{{Name: "TOKEN", Value: "new", Secret: true, Scope: "runtime"}}
	if p := PlanChanges(base, next); p.Operation != "recreate" || len(p.Changes) == 0 {
		t.Fatal(p)
	}
	next.Variables[0].Scope = "both"
	if p := PlanChanges(base, next); p.Operation != "rebuild" {
		t.Fatal(p)
	}
	next = base
	next.Hooks = []store.Hook{{Stage: "before_deploy", Command: store.Command{Shell: "migrate"}}}
	if p := PlanChanges(base, next); p.Operation != "recreate" {
		t.Fatal(p)
	}
	next = base
	next.StartCommand = "worker"
	next.StartupPolicy = "process"
	if p := PlanChanges(base, next); p.Operation != "recreate" {
		t.Fatal(p)
	}
}
func TestCronSchedules(t *testing.T) {
	cases := []struct{ expr, zone, after, want string }{{"*/15 * * * *", "UTC", "2026-11-03T11:01:00Z", "2026-11-03T11:15:00Z"}, {"0 9 * * 1-5", "Europe/Budapest", "2026-11-06T09:00:00Z", "2026-11-09T08:00:00Z"}, {"30 2 * * *", "Europe/Budapest", "2026-03-28T02:00:00Z", "2026-03-30T00:30:00Z"}, {"0 0 1 * 0", "UTC", "2026-11-01T00:00:00Z", "2026-11-08T00:00:00Z"}}
	for _, c := range cases {
		after, _ := time.Parse(time.RFC3339, c.after)
		got, e := NextSchedule(c.expr, c.zone, after)
		if e != nil || got.Format(time.RFC3339) != c.want {
			t.Fatal(c, got, e)
		}
	}
	for _, bad := range []string{"* * *", "61 * * * *", "*/0 * * * *", "x * * * *", "0 0 31 2 *"} {
		if _, e := NextSchedule(bad, "UTC", time.Now()); e == nil {
			t.Fatal("accepted", bad)
		}
	}
}
func TestHookValidationAndNoShellArgv(t *testing.T) {
	c := store.Config{Method: "dockerfile", Hooks: []store.Hook{{Stage: "before_deploy", Command: store.Command{Argv: []string{"/migrate", "up"}}}}}
	if e := ValidateHooks(&c); e != nil {
		t.Fatal(e)
	}
	if args := CommandArgs(c.Hooks[0].Command); args[0] != "/migrate" {
		t.Fatal(args)
	}
	c.Hooks[0].Command.Mode = "exec"
	if ValidateHooks(&c) == nil {
		t.Fatal("allowed existing-container first-deploy migration")
	}
	c.Method = "compose"
	c.Hooks[0].Command.Mode = "job"
	if ValidateHooks(&c) == nil {
		t.Fatal("allowed ambiguous service")
	}
}

func TestScheduleBusyPolicyAndDowntime(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, e := store.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	a := store.Application{ID: "scheduled", Name: "Scheduled", Config: testConfig()}
	first, e := db.Enqueue(ctx, a, store.Deployment{ID: "active", Operation: "deploy"}, "", true)
	if e != nil {
		t.Fatal(e)
	}
	first.State = "succeeded"
	db.SaveDeployment(ctx, first)
	db.SetActive(ctx, a.ID, first.ID)
	a, _ = db.Application(ctx, a.ID)
	busy, e := db.Enqueue(ctx, a, store.Deployment{ID: "busy", Operation: "restart"}, "", false)
	if e != nil {
		t.Fatal(e)
	}
	at := time.Now().Unix() - 1
	task := store.Task{ID: "scheduled-task", ApplicationID: a.ID, Name: "Task", Enabled: true, Command: store.Command{Argv: []string{"job"}, Timeout: 30, Mode: "job"}, Schedule: "* * * * *", Timezone: "UTC", Overlap: "queue", Missed: "skip", NextAt: at}
	db.SaveTask(ctx, task)
	w := NewWorker(db, &fakeRuntime{}, events.New(), dir)
	w.scheduled(ctx)
	waiting, _ := db.Task(ctx, a.ID, task.ID)
	if !waiting.Waiting || waiting.NextAt != at {
		t.Fatal("busy run was not retained", waiting)
	}
	busy.State = "succeeded"
	db.SaveDeployment(ctx, busy)
	w.scheduled(ctx)
	pending, e := db.Pending(ctx)
	if e != nil || len(pending) != 1 || pending[0].TaskID != task.ID {
		t.Fatal(pending, e)
	}
	w.scheduled(ctx)
	pending, _ = db.Pending(ctx)
	if len(pending) != 1 {
		t.Fatal("schedule was duplicated")
	}
	job := pending[0]
	job.State = "succeeded"
	db.SaveDeployment(ctx, job)
	task.NextAt = time.Now().Unix() - 3600
	task.Overlap = "skip"
	task.Missed = "skip"
	db.SaveTask(ctx, task)
	w.scheduled(ctx)
	pending, _ = db.Pending(ctx)
	if len(pending) != 0 {
		t.Fatal("replayed missed run")
	}
	task.Missed = "once"
	db.SaveTask(ctx, task)
	w.scheduled(ctx)
	pending, _ = db.Pending(ctx)
	if len(pending) != 1 {
		t.Fatal("missed once did not enqueue")
	}
}
