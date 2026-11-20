package deploy

import (
	"strings"
	"testing"

	"github.com/moby/moby/api/types/container"
)

func TestCandidateCrashDetection(t *testing.T) {
	state := func(running bool, status container.ContainerState, exit, restarts int, policy container.RestartPolicyMode) container.InspectResponse {
		return container.InspectResponse{
			State:        &container.State{Running: running, Status: status, ExitCode: exit},
			RestartCount: restarts,
			HostConfig:   &container.HostConfig{RestartPolicy: container.RestartPolicy{Name: policy}},
		}
	}
	for _, tc := range []struct {
		name string
		c    container.InspectResponse
		want string
	}{
		{"healthy start", state(true, "running", 0, 0, "unless-stopped"), ""},
		{"one restart can still recover", state(false, "restarting", 1, 1, "unless-stopped"), ""},
		{"exited but will restart", state(false, "exited", 1, 0, "unless-stopped"), ""},
		{"crash loop", state(false, "restarting", 1, 3, "unless-stopped"), "keeps restarting (exit code 1)"},
		{"running after a crash loop", state(true, "running", 0, 4, "unless-stopped"), "keeps restarting"},
		{"never restarted", state(false, "exited", 2, 0, "no"), "exited with code 2"},
	} {
		err := candidateCrashed(tc.c)
		if tc.want == "" && err != nil || tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
			t.Fatalf("%s: got %v, want %q", tc.name, err, tc.want)
		}
	}
}
