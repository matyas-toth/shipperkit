package deploy

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"shipper/internal/auth"
	"shipper/internal/engine"
)

// This acceptance test creates a single isolated, labelled container with no ports or volumes.
func TestDockerTerminalShellStateAndRevoke(t *testing.T) {
	if os.Getenv("SHIPPER_TEST_TERMINAL") != "1" {
		t.Skip("set SHIPPER_TEST_TERMINAL=1 for isolated Docker terminal acceptance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	engine, err := engine.New()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	runtime := &DockerRuntime{Client: engine.Client(), Instance: auth.Token()}
	if err = runtime.pull(ctx, "alpine:3.22", func(string) {}); err != nil {
		t.Fatal(err)
	}
	created, err := runtime.Client.ContainerCreate(ctx, client.ContainerCreateOptions{Config: &container.Config{Image: "alpine:3.22", Cmd: []string{"sleep", "120"}, Labels: runtime.labels("terminal-test", "release")}, HostConfig: &container.HostConfig{NetworkMode: "none"}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		clean, stop := context.WithTimeout(context.Background(), 15*time.Second)
		defer stop()
		if err := runtime.removeOwned(clean, created.ID, "terminal-test"); err != nil {
			t.Error(err)
		}
	}()
	if _, err = runtime.Client.ContainerStart(ctx, created.ID, client.ContainerStartOptions{}); err != nil {
		t.Fatal(err)
	}
	if err = runtime.CheckTerminal(ctx, "other-app", created.ID); err == nil {
		t.Fatal("unrelated application allowed")
	}
	foreign := &DockerRuntime{Client: engine.Client(), Instance: auth.Token()}
	if err = foreign.CheckTerminal(ctx, "terminal-test", created.ID); err == nil {
		t.Fatal("foreign instance allowed")
	}
	shell, err := runtime.OpenTerminal(ctx, "terminal-test", created.ID, auth.Token()[:24], 90, 24)
	if err != nil {
		t.Fatal(err)
	}
	defer shell.Close()
	if err = shell.Resize(ctx, 110, 30); err != nil {
		t.Fatal(err)
	}
	read := make(chan string, 32)
	go func() {
		buffer := make([]byte, 4096)
		for {
			n, err := shell.Read(buffer)
			if n > 0 {
				read <- string(buffer[:n])
			}
			if err != nil {
				return
			}
		}
	}()
	await := func(marker string) {
		t.Helper()
		timer := time.NewTimer(10 * time.Second)
		defer timer.Stop()
		var output strings.Builder
		for {
			select {
			case value := <-read:
				output.WriteString(value)
				if strings.Contains(output.String(), marker) {
					return
				}
			case <-timer.C:
				t.Fatalf("shell output missing %q: %s", marker, output.String())
			}
		}
	}
	if _, err = io.WriteString(shell, "export SHIPPER_TEST_VALUE=retained; cd /tmp\n"); err != nil {
		t.Fatal(err)
	}
	// A later input frame must retain both environment and working directory.
	if _, err = io.WriteString(shell, "printf 'STATE:%s:%s\\n' \"$SHIPPER_TEST_VALUE\" \"$PWD\"\n"); err != nil {
		t.Fatal(err)
	}
	await("STATE:retained:/tmp")
	if err = shell.Close(); err != nil {
		t.Fatal(err)
	}
	inspected, err := runtime.Client.ExecInspect(ctx, shell.(*dockerTerminal).exec, client.ExecInspectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if inspected.Running {
		t.Fatal("revoked shell remains running")
	}
	if _, err = io.WriteString(shell, "echo forbidden\n"); err == nil {
		t.Fatal("revoked terminal accepted input")
	}
	short, stop := context.WithTimeout(ctx, 1500*time.Millisecond)
	timed, err := runtime.OpenTerminal(short, "terminal-test", created.ID, auth.Token()[:24], 80, 24)
	if err != nil {
		stop()
		t.Fatal(err)
	}
	defer timed.Close()
	// Do not cancel the attach or invoke Close: the deadline must be enforced
	// by the container even when the API server cannot perform cleanup.
	ended := make(chan struct{})
	go func() { _, _ = io.Copy(io.Discard, timed); close(ended) }()
	select {
	case <-ended:
	case <-time.After(6 * time.Second):
		t.Fatal("container-side timeout did not terminate shell")
	}
	stop()
	info, err := runtime.Client.ExecInspect(ctx, timed.(*dockerTerminal).exec, client.ExecInspectOptions{})
	if err != nil || info.Running {
		t.Fatal("timed shell remains running", err)
	}
	t.Log("Interactive TTY retained shell state, resized, rejected unrelated ownership, and terminated after revocation")
}
