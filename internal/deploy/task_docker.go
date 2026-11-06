package deploy

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"shipper/internal/startup"
	"shipper/internal/store"
)

const startupPath = "/.shipper-start"

func (r *DockerRuntime) startupImage(ctx context.Context, image string) (string, error) {
	info, e := r.Client.ImageInspect(ctx, image)
	if e != nil {
		return "", e
	}
	if info.Config.Labels["io.shipper.startup"] == "1" {
		return info.ID, nil
	}
	binary, e := startup.Binary(info.Architecture)
	if e != nil {
		return "", e
	}
	key := fmt.Sprintf("%x", sha256.Sum256(append([]byte(info.ID), binary...)))
	tag := "shipper/" + r.Instance[:12] + "/startup:" + key[:24]
	if cached, err := r.Client.ImageInspect(ctx, tag); err == nil {
		if cached.Config.Labels["io.shipper.instance"] != r.Instance {
			return "", errors.New("Startup image ownership check failed")
		}
		return cached.ID, nil
	}
	helper, e := r.Client.ContainerCreate(ctx, client.ContainerCreateOptions{Config: &container.Config{Image: image, Labels: map[string]string{"io.shipper.instance": r.Instance, "io.shipper.infrastructure": "startup-helper"}}, HostConfig: &container.HostConfig{NetworkMode: "none"}})
	if e != nil {
		return "", e
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
		defer cancel()
		r.Client.ContainerRemove(cleanup, helper.ID, client.ContainerRemoveOptions{Force: true})
	}()
	var b bytes.Buffer
	tw := tar.NewWriter(&b)
	if e = tw.WriteHeader(&tar.Header{Name: ".shipper-start", Mode: 0555, Size: int64(len(binary))}); e != nil {
		return "", e
	}
	if _, e = tw.Write(binary); e != nil {
		return "", e
	}
	if e = tw.Close(); e != nil {
		return "", e
	}
	if _, e = r.Client.CopyToContainer(ctx, helper.ID, client.CopyToContainerOptions{DestinationPath: "/", Content: &b}); e != nil {
		return "", e
	}
	var cfg container.Config
	raw, e := json.Marshal(info.Config)
	if e != nil {
		return "", e
	}
	if e = json.Unmarshal(raw, &cfg); e != nil {
		return "", e
	}
	labels := map[string]string{}
	for k, v := range cfg.Labels {
		labels[k] = v
	}
	labels["io.shipper.startup"] = "1"
	labels["io.shipper.instance"] = r.Instance
	labels["io.shipper.infrastructure"] = "startup-helper"
	cfg.Labels = labels
	committed, e := r.Client.ContainerCommit(ctx, helper.ID, client.ContainerCommitOptions{Reference: tag, Config: &cfg, Comment: "Shipper startup helper"})
	return committed.ID, e
}
func wrapStartup(commands []store.Command, entry, cmd []string) ([]string, error) {
	main := append(append([]string{}, entry...), cmd...)
	if len(main) == 0 {
		return nil, errors.New("The image needs a start command")
	}
	raw, e := json.Marshal(commands)
	if e != nil {
		return nil, e
	}
	return append([]string{startupPath, base64.RawStdEncoding.EncodeToString(raw)}, main...), nil
}
func startupCommands(d store.Deployment, service string) []store.Command {
	commands := []store.Command{}
	for _, h := range d.Config.Hooks {
		if h.Stage == "before_start" && h.Command.Service == "" {
			commands = append(commands, h.Command)
		}
	}
	if service == "" {
		return commands
	}
	for _, h := range d.Config.Hooks {
		if h.Stage == "before_start" && h.Command.Service == service {
			commands = append(commands, h.Command)
		}
	}
	return commands
}

func (r *DockerRuntime) configureStartup(ctx context.Context, d store.Deployment, cfg *container.Config) error {
	if d.TaskCommand != nil {
		cfg.Entrypoint = CommandArgs(*d.TaskCommand)
		cfg.Cmd = []string{}
		cfg.User = d.TaskCommand.User
		cfg.WorkingDir = d.TaskCommand.Directory
		cfg.Healthcheck = &container.HealthConfig{Test: []string{"NONE"}}
		return nil
	}
	commands := startupCommands(d, "")
	if len(commands) == 0 {
		return nil
	}
	image, e := r.Client.ImageInspect(ctx, cfg.Image)
	if e != nil {
		return e
	}
	entry, cmd := cfg.Entrypoint, cfg.Cmd
	if entry == nil {
		entry = image.Config.Entrypoint
	}
	if cmd == nil {
		cmd = image.Config.Cmd
	}
	if image.Config.Healthcheck != nil {
		raw, _ := json.Marshal(image.Config.Healthcheck)
		var health container.HealthConfig
		if e = json.Unmarshal(raw, &health); e != nil {
			return e
		}
		startPeriod := time.Duration(0)
		for _, c := range commands {
			startPeriod = max(startPeriod, time.Duration(c.Timeout)*time.Second)
		}
		health.StartPeriod = max(health.StartPeriod, startPeriod)
		cfg.Healthcheck = &health
	}
	cfg.Entrypoint, e = wrapStartup(commands, entry, cmd)
	cfg.Cmd = []string{}
	return e
}
func taskName(d store.Deployment, run store.TaskRun) string {
	return ContainerName(d.ApplicationName, "task", run.ID)
}
func (r *DockerRuntime) RunTask(ctx context.Context, d store.Deployment, run *store.TaskRun, checkpoint func() error, log func(string)) error {
	if run.Command.Mode == "exec" {
		return r.execTask(ctx, d, run, checkpoint, log)
	}
	name := taskName(d, *run)
	info, e := r.Client.ContainerInspect(ctx, name, client.ContainerInspectOptions{})
	if errdefs.IsNotFound(e) {
		if run.ContainerID != "" || run.State == "running" {
			return ErrUncertain
		}
		if ReleaseMethod(d) == "compose" {
			if e = r.createComposeTask(ctx, d, *run, log); e != nil {
				return e
			}
		} else {
			task := d
			task.ID = run.ID
			task.ContainerID = ""
			task.TaskCommand = &run.Command
			task.Config.Hooks = nil
			id, err := r.Start(ctx, task)
			if err != nil {
				return err
			}
			run.ContainerID = id
		}
		info, e = r.Client.ContainerInspect(ctx, name, client.ContainerInspectOptions{})
	}
	if e != nil {
		if run.ContainerID != "" || run.State != "prepared" {
			return fmt.Errorf("%w: %v", ErrUncertain, e)
		}
		return e
	}
	if !owned(info.Container.Config.Labels, r.Instance, d.ApplicationID) || info.Container.Config.Labels["io.shipper.task.id"] != run.ID {
		return errors.New("Task container ownership check failed")
	}
	run.ContainerID = info.Container.ID
	if e = checkpoint(); e != nil {
		return e
	}
	if info.Container.State.Status == "created" {
		if run.State == "starting" || run.State == "running" {
			return ErrUncertain
		}
		// Persist start intent separately from creation. A recovered created
		// container with this intent may have an in-flight start request.
		run.State = "starting"
		if e = checkpoint(); e != nil {
			return e
		}
		if _, e = r.Client.ContainerStart(ctx, run.ContainerID, client.ContainerStartOptions{}); e != nil {
			check, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
			defer cancel()
			observed, err := r.Client.ContainerInspect(check, run.ContainerID, client.ContainerInspectOptions{})
			if err == nil && observed.Container.State.Error != "" && !observed.Container.State.Running {
				return e
			}
			return fmt.Errorf("%w: %v", ErrUncertain, e)
		}
	}
	run.State = "running"
	if e = checkpoint(); e != nil {
		return e
	}

	logContext, stopLogs := context.WithCancel(ctx)
	logDone := make(chan error, 1)
	go func() { logDone <- r.taskOutput(logContext, run.ContainerID, log) }()
	defer func() {
		stopLogs()
		select {
		case <-logDone:
		case <-time.After(2 * time.Second):
		}
	}()
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	for {
		info, e = r.Client.ContainerInspect(ctx, run.ContainerID, client.ContainerInspectOptions{})
		if e != nil {
			return fmt.Errorf("%w: %v", ErrUncertain, e)
		}
		if !info.Container.State.Running && info.Container.State.Status != "created" {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
	select {
	case logError := <-logDone:
		if logError != nil {
			log("Task log stream ended: " + logError.Error())
		}
		logDone <- nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(2 * time.Second):
		log("Task log stream did not finish before the collection deadline")
	}
	run.ExitCode = info.Container.State.ExitCode
	if run.ExitCode != 0 {
		return fmt.Errorf("Task exited with status %d", run.ExitCode)
	}
	return nil
}
func (r *DockerRuntime) taskOutput(ctx context.Context, id string, log func(string)) error {
	logs, e := r.Client.ContainerLogs(ctx, id, client.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Tail: "5000", Timestamps: true, Follow: true})
	if e != nil {
		return e
	}
	defer logs.Close()
	writer := &logWriter{emit: func(line string) error { log(line); return nil }}
	_, e = stdcopy.StdCopy(writer, writer, logs)
	if e == nil && writer.pending != "" {
		log(writer.pending)
	}
	return e
}
func (r *DockerRuntime) CleanupTask(ctx context.Context, d store.Deployment, run store.TaskRun) error {
	if run.Command.Mode == "exec" || run.ContainerID == "" {
		return nil
	}
	info, e := r.Client.ContainerInspect(ctx, run.ContainerID, client.ContainerInspectOptions{})
	if errdefs.IsNotFound(e) {
		return nil
	}
	if e != nil {
		return e
	}
	if !owned(info.Container.Config.Labels, r.Instance, d.ApplicationID) || info.Container.Config.Labels["io.shipper.task.id"] != run.ID {
		return errors.New("Task ownership check failed")
	}
	_, e = r.Client.ContainerRemove(ctx, run.ContainerID, client.ContainerRemoveOptions{Force: true})
	return e
}
func (r *DockerRuntime) taskService(ctx context.Context, d store.Deployment, service string) (string, error) {
	if ReleaseMethod(d) != "compose" {
		return d.ContainerID, nil
	}
	items, e := r.composeContainers(ctx, d.ApplicationID)
	if e != nil {
		return "", e
	}
	for _, c := range items {
		if c.Labels["com.docker.compose.service"] == service && c.Labels["io.shipper.infrastructure"] == "" {
			return c.ID, nil
		}
	}
	return "", errors.New("The selected service is not running")
}
func (r *DockerRuntime) execTask(ctx context.Context, d store.Deployment, run *store.TaskRun, checkpoint func() error, log func(string)) error {
	if run.ExecID != "" {
		return ErrUncertain
	} // Exec has no atomic start/completion record.
	id, e := r.taskService(ctx, d, run.Command.Service)
	if e != nil {
		return e
	}
	info, e := r.Client.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if e != nil {
		return e
	}
	if !owned(info.Container.Config.Labels, r.Instance, d.ApplicationID) || !info.Container.State.Running {
		return errors.New("The selected service is not running")
	}
	created, e := r.Client.ExecCreate(ctx, id, client.ExecCreateOptions{Cmd: CommandArgs(run.Command), WorkingDir: run.Command.Directory, User: run.Command.User, AttachStdout: true, AttachStderr: true})
	if e != nil {
		return e
	}
	run.ContainerID = id
	run.ExecID = created.ID
	run.State = "running"
	if e = checkpoint(); e != nil {
		return e
	}
	stream, e := r.Client.ExecAttach(ctx, created.ID, client.ExecAttachOptions{})
	if e != nil {
		return fmt.Errorf("%w: %v", ErrUncertain, e)
	}
	defer stream.Close()
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			stream.Close()
		case <-done:
		}
	}()
	writer := &logWriter{emit: func(line string) error { log(line); return nil }}
	_, e = stdcopy.StdCopy(writer, writer, stream.Reader)
	if e == nil && ctx.Err() == nil && writer.pending != "" {
		log(writer.pending)
	}
	if ctx.Err() != nil {
		return ErrUncertain
	}
	if e != nil {
		return fmt.Errorf("%w: %v", ErrUncertain, e)
	}
	result, e := r.Client.ExecInspect(ctx, created.ID, client.ExecInspectOptions{})
	if e != nil {
		return fmt.Errorf("%w: %v", ErrUncertain, e)
	}
	if result.Running {
		return ErrUncertain
	}
	run.ExitCode = result.ExitCode
	if result.ExitCode != 0 {
		return fmt.Errorf("Task exited with status %d", result.ExitCode)
	}
	return nil
}
func (r *DockerRuntime) createComposeTask(ctx context.Context, d store.Deployment, run store.TaskRun, log func(string)) error {
	var plan composePlan
	if e := json.Unmarshal([]byte(d.ComposePlan), &plan); e != nil {
		return e
	}
	services, ok := plan.Project["services"].(map[string]any)
	if !ok {
		return errors.New("Compose services are unavailable")
	}
	raw, ok := services[run.Command.Service].(map[string]any)
	if !ok {
		return errors.New("Choose a service from this release")
	}
	// Start only the transitive dependencies, never the application consumer. The
	// normalized Compose model retains health/completion dependency conditions.
	if strings.HasPrefix(run.Stage, "before_deploy") {
		deps := map[string]bool{}
		visiting := map[string]bool{}
		var visit func(string) error
		visit = func(name string) error {
			if visiting[name] {
				return errors.New("Cyclic task dependencies are unsupported")
			}
			if deps[name] {
				return nil
			}
			visiting[name] = true
			svc, ok := services[name].(map[string]any)
			if !ok {
				return errors.New("Task dependency is unavailable")
			}
			depends, _ := svc["depends_on"].(map[string]any)
			for key := range depends {
				if key == run.Command.Service {
					return errors.New("A migration dependency would start its consumer")
				}
				if e := visit(key); e != nil {
					return e
				}
				deps[key] = true
			}
			visiting[name] = false
			return nil
		}
		if e := visit(run.Command.Service); e != nil {
			return e
		}
		if len(deps) > 0 {
			args := []string{"up", "-d", "--no-build", "--pull", "never", "--wait", "--wait-timeout", strconv.Itoa(min(run.Command.Timeout, 90))}
			for name := range deps {
				args = append(args, name)
			}
			if e := r.composeCommand(ctx, d, args, log); e != nil {
				return e
			}
		}
	}
	svc := map[string]any{}
	for k, v := range raw {
		svc[k] = v
	}
	svc["container_name"] = taskName(d, run)
	svc["entrypoint"] = CommandArgs(run.Command)
	svc["command"] = []string{}
	svc["restart"] = "no"
	svc["healthcheck"] = map[string]any{"disable": true}
	delete(svc, "depends_on")
	delete(svc, "build")
	delete(svc, "ports")
	if run.Command.Directory != "" {
		svc["working_dir"] = run.Command.Directory
	}
	if run.Command.User != "" {
		svc["user"] = run.Command.User
	}
	labels := r.labels(d.ApplicationID, d.ID)
	labels["io.shipper.infrastructure"] = "task"
	labels["io.shipper.task.id"] = run.ID
	svc["labels"] = labels
	key := "task-" + run.ID
	plan.Project["services"] = map[string]any{key: svc}
	b, e := json.Marshal(plan)
	if e != nil {
		return e
	}
	d.ComposePlan = string(b)
	return r.composeCommand(ctx, d, []string{"create", "--no-build", "--pull", "never", key}, log)
}

func (w *Worker) prepareStartup(ctx context.Context, d *store.Deployment) error {
	for _, h := range d.Config.Hooks {
		if h.Stage == "before_start" {
			if r, ok := w.Runtime.(*DockerRuntime); ok {
				image, e := r.startupImage(ctx, d.ImageID)
				if e != nil {
					return e
				}
				d.ImageID = image
			}
			break
		}
	}
	return nil
}
func (r *DockerRuntime) composeStartup(ctx context.Context, d *store.Deployment) error {
	var plan composePlan
	if e := json.Unmarshal([]byte(d.ComposePlan), &plan); e != nil {
		return e
	}
	services, _ := plan.Project["services"].(map[string]any)
	for _, name := range serviceNames(*d) {
		commands := startupCommands(*d, name)
		if len(commands) == 0 {
			continue
		}
		svc, ok := services[name].(map[string]any)
		if !ok {
			return errors.New("Startup service is unavailable")
		}
		image, _ := svc["image"].(string)
		id, e := r.startupImage(ctx, image)
		if e != nil {
			return e
		}
		info, e := r.Client.ImageInspect(ctx, id)
		if e != nil {
			return e
		}
		entry, cmd := info.Config.Entrypoint, info.Config.Cmd
		if raw, ok := svc["entrypoint"]; ok && raw != nil {
			b, _ := json.Marshal(raw)
			if e = json.Unmarshal(b, &entry); e != nil {
				return e
			}
		}
		if raw, ok := svc["command"]; ok && raw != nil {
			b, _ := json.Marshal(raw)
			if e = json.Unmarshal(b, &cmd); e != nil {
				return e
			}
		}
		// A reused release already contains a startup wrapper. Replace its command
		// while preserving the original foreground process.
		if len(entry) >= 3 && entry[0] == startupPath {
			entry = entry[2:]
			cmd = nil
		}
		wrapped, e := wrapStartup(commands, entry, cmd)
		if e != nil {
			return e
		}
		svc["image"] = id
		svc["entrypoint"] = wrapped
		svc["command"] = []string{}
		timeout := 0
		for _, c := range commands {
			timeout = max(timeout, c.Timeout)
		}
		if raw, ok := svc["healthcheck"].(map[string]any); ok {
			raw["start_period"] = fmt.Sprintf("%ds", timeout)
		} else if info.Config.Healthcheck != nil {
			health := info.Config.Healthcheck
			svc["healthcheck"] = map[string]any{"test": health.Test, "interval": health.Interval.String(), "timeout": health.Timeout.String(), "retries": health.Retries, "start_period": fmt.Sprintf("%ds", timeout)}
		}
	}
	b, e := json.Marshal(plan)
	d.ComposePlan = string(b)
	return e
}
