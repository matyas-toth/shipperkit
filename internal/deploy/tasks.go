package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"shipper/internal/auth"
	"shipper/internal/store"
)

var ErrUncertain = errors.New("Task result is uncertain; inspect its effects before running it again")

func CommandArgs(c store.Command) []string {
	if c.Shell != "" {
		return []string{"/bin/sh", "-c", c.Shell}
	}
	return c.Argv
}
func ValidateCommand(c *store.Command) error {
	if (c.Shell == "") == (len(c.Argv) == 0) {
		return errors.New("Enter a shell command or an argument list")
	}
	if len(c.Shell) > 4096 || strings.ContainsRune(c.Shell, 0) || len(c.Argv) > 128 {
		return errors.New("Command is too large or contains null bytes")
	}
	for _, arg := range c.Argv {
		if len(arg) > 4096 || strings.ContainsRune(arg, 0) {
			return errors.New("Invalid command argument")
		}
	}
	if c.Timeout == 0 {
		c.Timeout = 300
	}
	if c.Timeout < 1 || c.Timeout > 3600 {
		return errors.New("Task timeout must be between 1 and 3600 seconds")
	}
	if c.Mode == "" {
		c.Mode = "job"
	}
	if c.Mode != "job" && c.Mode != "exec" {
		return errors.New("Choose a fresh job or a running service")
	}
	if c.Service != "" && !serviceName.MatchString(c.Service) {
		return errors.New("Choose a valid task service")
	}
	if c.Directory != "" && (!strings.HasPrefix(c.Directory, "/") || path.Clean(c.Directory) != c.Directory || strings.ContainsAny(c.Directory, "\\\x00\r\n")) {
		return errors.New("Working directory must be an absolute container path")
	}
	if len(c.User) > 128 || strings.ContainsAny(c.User, "\x00\r\n") {
		return errors.New("Invalid task user")
	}
	return nil
}
func ValidateHooks(c *store.Config) error {
	if c.StartupPolicy != "" && c.StartupPolicy != "http" && c.StartupPolicy != "process" {
		return errors.New("Choose HTTP or process readiness")
	}
	if c.Method == "static" && c.StartupPolicy == "process" {
		return errors.New("Static sites use HTTP readiness")
	}
	if len(c.Hooks) > 128 {
		return errors.New("Too many lifecycle commands")
	}
	seen := map[string]bool{}
	for i := range c.Hooks {
		h := &c.Hooks[i]
		if h.Stage != "before_deploy" && h.Stage != "before_start" && h.Stage != "after_deploy" && h.Stage != "before_stop" {
			return errors.New("Invalid lifecycle stage")
		}
		if e := ValidateCommand(&h.Command); e != nil {
			return e
		}
		key := h.Stage + ":" + h.Command.Service
		if seen[key] {
			return errors.New("Use one command per service and lifecycle stage")
		}
		seen[key] = true
		if c.Method != "compose" && h.Command.Service != "" {
			return errors.New("Single-container commands do not need a service")
		}
		if h.Stage == "before_start" && h.Command.Mode == "exec" {
			return errors.New("Before start runs in the starting container")
		}
		if h.Stage == "before_deploy" && h.Command.Mode == "exec" {
			return errors.New("Before deploy uses the candidate image")
		}
	}
	return nil
}

type TaskRuntime interface {
	RunTask(context.Context, store.Deployment, *store.TaskRun, func() error, func(string)) error
	CleanupTask(context.Context, store.Deployment, store.TaskRun) error
}

func hookStageLabel(stage string) string {
	parts := strings.Split(stage, ":")
	label := strings.ReplaceAll(parts[0], "_", " ")
	for _, part := range parts[1:] {
		if part == "all" {
			part = "All services"
		}
		label += " · " + part
	}
	return label
}

func (w *Worker) runTask(ctx context.Context, d store.Deployment, stage string, c store.Command) error {
	r, e := w.DB.EnsureTaskRun(ctx, store.TaskRun{ID: auth.Token()[:24], DeploymentID: d.ID, Stage: stage, Command: c})
	if e != nil {
		return e
	}
	if r.State == "succeeded" {
		return nil
	}
	if r.State == "failed" || r.State == "uncertain" {
		return fmt.Errorf("%s: %s", stage, r.Error)
	}
	runtime, ok := w.Runtime.(TaskRuntime)
	if !ok {
		return errors.New("Task execution is unavailable")
	}
	taskCtx, cancel := context.WithDeadline(ctx, time.Unix(r.StartedAt, 0).Add(time.Duration(r.Command.Timeout)*time.Second))
	defer cancel()
	checkpoint := func() error { return w.DB.SaveTaskRun(ctx, r) }
	w.log(d, "Running "+hookStageLabel(stage))
	e = runtime.RunTask(taskCtx, d, &r, checkpoint, func(line string) { w.log(d, line) })
	if ctx.Err() != nil {
		return ctx.Err()
	}
	r.FinishedAt = time.Now().Unix()
	r.State = "succeeded"
	if e != nil {
		r.State = "failed"
		if errors.Is(e, ErrUncertain) {
			r.State = "uncertain"
		}
		r.Error = RedactDeployment(d, e.Error())
	}
	if err := w.DB.SaveTaskRun(context.WithoutCancel(ctx), r); err != nil {
		return err
	}
	w.changed()
	// Retain ambiguous running executions for inspection. Completed job containers
	// can be removed only after the durable outcome and logs have been saved.
	if r.State != "uncertain" {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
		defer cancel()
		if err := runtime.CleanupTask(cleanup, d, r); err != nil {
			w.log(d, "Task cleanup: "+err.Error())
		}
	}
	return e
}

type hookRun struct {
	stage   string
	command store.Command
}

func serviceNames(d store.Deployment) []string {
	names := []string{}
	seen := map[string]bool{}
	for _, svc := range d.Services {
		if svc.Name != "" && !seen[svc.Name] {
			seen[svc.Name] = true
			names = append(names, svc.Name)
		}
	}
	if len(names) == 0 && ReleaseMethod(d) == "compose" {
		var plan composePlan
		if e := json.Unmarshal([]byte(d.ComposePlan), &plan); e == nil {
			services, _ := plan.Project["services"].(map[string]any)
			for name := range services {
				if !seen[name] {
					seen[name] = true
					names = append(names, name)
				}
			}
		}
	}
	sort.Strings(names)
	return names
}

func hookRuns(d store.Deployment, stage string) []hookRun {
	hooks := []store.Hook{}
	for _, h := range d.Config.Hooks {
		if h.Stage == stage {
			hooks = append(hooks, h)
		}
	}
	if ReleaseMethod(d) != "compose" {
		runs := make([]hookRun, 0, len(hooks))
		for _, h := range hooks {
			runs = append(runs, hookRun{stage: stage, command: h.Command})
		}
		return runs
	}
	runs := []hookRun{}
	for _, svc := range serviceNames(d) {
		for _, h := range hooks {
			if h.Command.Service != "" {
				continue
			}
			command := h.Command
			command.Service = svc
			runs = append(runs, hookRun{stage: stage + ":all:" + svc, command: command})
		}
		for _, h := range hooks {
			if h.Command.Service == svc {
				runs = append(runs, hookRun{stage: stage + ":" + svc, command: h.Command})
			}
		}
	}
	return runs
}

func (w *Worker) hooks(ctx context.Context, d store.Deployment, stage string, warning bool) error {
	return w.runHooks(ctx, d, hookRuns(d, stage), warning)
}

func (w *Worker) runHooks(ctx context.Context, d store.Deployment, runs []hookRun, warning bool) error {
	for _, run := range runs {
		if e := w.runTask(ctx, d, run.stage, run.command); e != nil {
			if ctx.Err() != nil {
				return e
			}
			if !warning {
				return fmt.Errorf("%s: %w", run.stage, e)
			}
			w.log(d, "Warning: "+e.Error())
		}
	}
	return nil
}
func (w *Worker) scheduled(ctx context.Context) {
	now := time.Now()
	tasks, e := w.DB.DueTasks(ctx, now.Unix())
	if e != nil {
		return
	}
	for _, t := range tasks {
		next, e := NextSchedule(t.Schedule, t.Timezone, now)
		if e != nil {
			continue
		}
		if t.Missed == "skip" && !t.Waiting && now.Unix()-t.NextAt >= 60 {
			w.DB.AdvanceTask(ctx, t.ID, t.NextAt, next.Unix())
			continue
		}
		a, e := w.DB.Application(ctx, t.ApplicationID)
		if e != nil {
			continue
		}
		if a.ActiveDeploymentID == "" || a.DesiredState == "stopped" {
			w.DB.AdvanceTask(ctx, t.ID, t.NextAt, next.Unix())
			continue
		}
		release, e := w.DB.Deployment(ctx, a.ActiveDeploymentID)
		if e != nil {
			continue
		}
		a.Config = release.Config
		_, e = w.DB.Enqueue(ctx, a, store.Deployment{ID: auth.Token()[:24], Operation: "task", Actor: "schedule", TaskID: t.ID, TaskCommand: &t.Command}, fmt.Sprintf("schedule:%s:%d", t.ID, t.NextAt), false)
		if errors.Is(e, store.ErrBusy) && t.Overlap == "queue" {
			w.DB.WaitTask(ctx, t.ID, t.NextAt)
			continue
		}
		if e == nil || errors.Is(e, store.ErrBusy) || errors.Is(e, store.ErrIdempotencyConflict) {
			w.DB.AdvanceTask(ctx, t.ID, t.NextAt, next.Unix())
			if e == nil {
				w.changed()
			}
		}
	}
}
