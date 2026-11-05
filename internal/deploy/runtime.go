package deploy

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/client"
	"shipper/internal/store"
)

func (r *DockerRuntime) ResolveRuntime(ctx context.Context, d *store.Deployment) error {
	if d.RuntimePort > 0 {
		return nil
	}
	if ReleaseMethod(*d) == "static" {
		d.RuntimePort = 8080
		return nil
	}
	if d.Config.Port > 0 {
		d.RuntimePort = d.Config.Port
	} else {
		for _, v := range d.Config.Variables {
			if v.Name == "PORT" && (v.Scope == "runtime" || v.Scope == "both" || v.Scope == "") {
				port, e := strconv.Atoi(v.Value)
				if e != nil || port < 1 || port > 65535 {
					return errors.New("PORT must be a valid container port")
				}
				d.RuntimePort = port
			}
		}
		if d.RuntimePort == 0 {
			image, e := r.Client.ImageInspect(ctx, d.ImageID)
			if e != nil {
				return e
			}
			if image.Config != nil && len(image.Config.ExposedPorts) == 1 {
				for port := range image.Config.ExposedPorts {
					number, _, _ := strings.Cut(string(port), "/")
					d.RuntimePort, _ = strconv.Atoi(number)
				}
			}
		}
		if d.RuntimePort == 0 {
			d.RuntimePort = 3000
		}
	}
	for _, v := range d.Config.Variables {
		if v.Name == "PORT" && (v.Scope == "runtime" || v.Scope == "both" || v.Scope == "") && v.Value != strconv.Itoa(d.RuntimePort) {
			return errors.New("PORT must match the configured container port")
		}
	}
	return nil
}
func (r *DockerRuntime) Lifecycle(ctx context.Context, d store.Deployment, action string) error {
	if ReleaseMethod(d) == "compose" {
		return r.ComposeLifecycle(ctx, d, action)
	}
	info, e := r.Client.ContainerInspect(ctx, d.ContainerID, client.ContainerInspectOptions{})
	if e != nil {
		return e
	}
	if !owned(info.Container.Config.Labels, r.Instance, d.ApplicationID) || info.Container.Config.Labels["io.shipper.deployment.id"] != d.ID {
		return errors.New("Application container ownership check failed")
	}
	if action == "stop" || action == "restart" {
		if info.Container.State.Running {
			if _, e = r.Client.ContainerStop(ctx, d.ContainerID, client.ContainerStopOptions{Timeout: ptr(StopSeconds(d.Config.Runtime))}); e != nil {
				return e
			}
		}
	}
	if action == "start" || action == "restart" {
		if _, e = r.Client.ContainerStart(ctx, d.ContainerID, client.ContainerStartOptions{}); e != nil {
			return e
		}
	}
	return nil
}

type RuntimeLogSource interface {
	Logs(context.Context, string, string, int, func(string) error) error
}
type logWriter struct {
	pending string
	discard bool
	emit    func(string) error
}

func (w *logWriter) Write(data []byte) (int, error) {
	n := len(data)
	w.pending += string(data)
	for {
		line, rest, found := strings.Cut(w.pending, "\n")
		if !found {
			if len(w.pending) > 65536 {
				w.discard = true
				w.pending = ""
			}
			break
		} else {
			w.pending = rest
		}
		if w.discard || len(line) > 65536 {
			line = "[Log line exceeded 64 KiB]"
			w.discard = false
		}
		if e := w.emit(strings.TrimSuffix(line, "\r")); e != nil {
			return 0, e
		}
	}
	return n, nil
}
func (r *DockerRuntime) Logs(ctx context.Context, appID, containerID string, tail int, emit func(string) error) error {
	info, e := r.Client.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
	if e != nil {
		return e
	}
	if !owned(info.Container.Config.Labels, r.Instance, appID) || info.Container.Config.Labels["io.shipper.managed"] != "true" {
		return errors.New("Container ownership check failed")
	}
	if tail < 1 || tail > 500 {
		tail = 200
	}
	logs, e := r.Client.ContainerLogs(ctx, containerID, client.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Follow: info.Container.State != nil && info.Container.State.Running, Tail: strconv.Itoa(tail), Timestamps: true})
	if e != nil {
		return e
	}
	defer logs.Close()
	writer := &logWriter{emit: emit}
	if info.Container.Config.Tty {
		_, e = copyRuntimeLogs(writer, logs)
	} else {
		_, e = stdcopy.StdCopy(writer, writer, logs)
	}
	if writer.pending != "" {
		if err := emit(writer.pending); e == nil {
			e = err
		}
	}
	return e
}
func copyRuntimeLogs(w *logWriter, r interface{ Read([]byte) (int, error) }) (int64, error) {
	var total int64
	buffer := make([]byte, 8192)
	for {
		n, e := r.Read(buffer)
		if n > 0 {
			written, err := w.Write(buffer[:n])
			total += int64(written)
			if err != nil {
				return total, err
			}
		}
		if e != nil {
			return total, e
		}
	}
}

func ParseDotenv(text string) ([]store.Variable, error) {
	if len(text) > 128<<10 {
		return nil, errors.New("The environment file is too large")
	}
	out := []store.Variable{}
	seen := map[string]bool{}
	for index, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		name, value, ok := strings.Cut(line, "=")
		name = strings.TrimSpace(name)
		if !ok || !variableName.MatchString(name) {
			return nil, fmt.Errorf("Line %d must contain NAME=value", index+1)
		}
		if seen[name] {
			return nil, fmt.Errorf("Variable %s appears more than once", name)
		}
		seen[name] = true
		value = strings.TrimSpace(value)
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		out = append(out, store.Variable{Name: name, Value: value, Secret: true, Scope: "runtime"})
	}
	if len(out) > 128 {
		return nil, errors.New("Applications support up to 128 variables")
	}
	return out, nil
}
