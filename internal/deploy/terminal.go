package deploy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/moby/moby/client"
)

// TerminalSource exposes only an owned container's interactive shell, never a host process.
type TerminalSource interface {
	CheckTerminal(context.Context, string, string) error
	OpenTerminal(context.Context, string, string, string, uint, uint) (Terminal, error)
}
type Terminal interface {
	io.ReadWriteCloser
	Resize(context.Context, uint, uint) error
}

func (r *DockerRuntime) CheckTerminal(ctx context.Context, app, id string) error {
	info, err := r.Client.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return err
	}
	c := info.Container
	if c.Config == nil || !owned(c.Config.Labels, r.Instance, app) || c.Config.Labels["io.shipper.managed"] != "true" || c.Config.Labels["io.shipper.infrastructure"] != "" {
		return errors.New("container ownership check failed")
	}
	if c.State == nil || !c.State.Running {
		return errors.New("container is not running")
	}
	return nil
}

var terminalID = regexp.MustCompile(`^[a-f0-9]{24}$`)

type dockerTerminal struct {
	runtime               *DockerRuntime
	app, id, exec, marker string
	stream                client.ExecAttachResult
	once                  sync.Once
	closeErr              error
}

func (r *DockerRuntime) OpenTerminal(ctx context.Context, app, id, session string, cols, rows uint) (Terminal, error) {
	if !terminalID.MatchString(session) {
		return nil, errors.New("invalid terminal session")
	}
	if err := r.CheckTerminal(ctx, app, id); err != nil {
		return nil, err
	}
	// The random, private marker identifies the shell inside its own PID namespace.
	marker := "/tmp/.shipper-terminal-" + session
	seconds := 1800
	if deadline, ok := ctx.Deadline(); ok {
		seconds = max(1, min(seconds, int(time.Until(deadline).Seconds())+1))
	}
	// A container-side deadline also stops the shell if the server crashes.
	// timeout is provided by BusyBox and coreutils; images without it fail closed.
	command := `exec timeout -s KILL ` + strconv.Itoa(seconds) + ` /bin/sh -c 'umask 077; set -C; echo $$ > ` + marker + ` || exit 1; export HISTFILE=/dev/null; exec /bin/sh -i'`
	created, err := r.Client.ExecCreate(ctx, id, client.ExecCreateOptions{Cmd: []string{"/bin/sh", "-c", command}, Env: []string{"TERM=xterm-256color", "HISTFILE=/dev/null"}, TTY: true, AttachStdin: true, AttachStdout: true, AttachStderr: true, ConsoleSize: client.ConsoleSize{Width: cols, Height: rows}})
	if err != nil {
		return nil, err
	}
	stream, err := r.Client.ExecAttach(ctx, created.ID, client.ExecAttachOptions{TTY: true, ConsoleSize: client.ConsoleSize{Width: cols, Height: rows}})
	if err != nil {
		return nil, err
	}
	t := &dockerTerminal{runtime: r, app: app, id: id, exec: created.ID, marker: marker, stream: stream}
	return t, nil
}
func (t *dockerTerminal) Read(p []byte) (int, error)  { return t.stream.Reader.Read(p) }
func (t *dockerTerminal) Write(p []byte) (int, error) { return t.stream.Conn.Write(p) }
func (t *dockerTerminal) Resize(ctx context.Context, cols, rows uint) error {
	if cols < 2 || cols > 300 || rows < 2 || rows > 120 {
		return errors.New("invalid terminal size")
	}
	_, err := t.runtime.Client.ExecResize(ctx, t.exec, client.ExecResizeOptions{Width: cols, Height: rows})
	return err
}
func (t *dockerTerminal) Close() error {
	t.once.Do(func() {
		// Close the attach immediately to stop browser access even if Docker is unavailable.
		t.stream.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		if err := t.runtime.CheckTerminal(ctx, t.app, t.id); err != nil {
			t.closeErr = err
			return
		}
		before, err := t.runtime.Client.ExecInspect(ctx, t.exec, client.ExecInspectOptions{})
		if err != nil {
			t.closeErr = err
			return
		}
		if !before.Running {
			// A natural shell exit may leave its marker behind. Never signal a
			// stale PID after the original exec has already ended.
			_, t.closeErr = t.runtime.exec(ctx, t.id, []string{"/bin/sh", "-c", "rm -f " + t.marker})
			return
		}
		// Never signal a host PID. This cleanup runs in the original owned container.
		// HUP closes interactive jobs, then KILL ends any shell ignoring hangup.
		command := `if [ -f ` + t.marker + ` ]; then read p < ` + t.marker + `; case "$p" in ''|*[!0-9]*) exit 1;; esac; if [ "$p" -gt 1 ]; then kill -HUP -"$p" 2>/dev/null || kill -HUP "$p" 2>/dev/null; kill -KILL -"$p" 2>/dev/null || kill -KILL "$p" 2>/dev/null; fi; rm -f ` + t.marker + `; fi`
		if _, err := t.runtime.exec(ctx, t.id, []string{"/bin/sh", "-c", command}); err != nil {
			t.closeErr = err
			return
		}
		info, err := t.runtime.Client.ExecInspect(ctx, t.exec, client.ExecInspectOptions{})
		if err != nil {
			t.closeErr = err
			return
		}
		if info.Running {
			t.closeErr = fmt.Errorf("terminal shell did not terminate")
		}
	})
	return t.closeErr
}
