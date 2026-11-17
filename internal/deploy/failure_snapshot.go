package deploy

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/moby/moby/client"
	"shipper/internal/store"
)

// FailureSnapshot preserves the failed service logs before the candidate is
// removed. It is bounded and redacted before it is stored.
type FailureSnapshot struct {
	CapturedAt int64               `json:"capturedAt"`
	Complete   bool                `json:"complete"`
	Services   []FailureServiceLog `json:"services"`
}

type FailureServiceLog struct {
	Service     string   `json:"service"`
	ContainerID string   `json:"containerId,omitempty"`
	Lines       []string `json:"lines"`
}

type failureTarget struct {
	Service     string
	ContainerID string
}

func (w *Worker) captureFailureLogs(ctx context.Context, d store.Deployment, targets []failureTarget) {
	source, ok := w.Runtime.(RuntimeLogSnapshotSource)
	if !ok {
		return
	}
	work, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	services := []FailureServiceLog{}
	for _, target := range targets {
		if target.ContainerID == "" || work.Err() != nil || len(services) >= 20 {
			continue
		}
		lines := []string{}
		_ = source.LogSnapshot(work, d.ApplicationID, target.ContainerID, 100, func(line string) error {
			if len(lines) >= 100 {
				return nil
			}
			line = strings.TrimSpace(RedactDeployment(d, line))
			if line != "" {
				lines = append(lines, line)
			}
			return nil
		})
		if len(lines) == 0 {
			continue
		}
		name := target.Service
		if name == "" {
			name = "application"
		}
		services = append(services, FailureServiceLog{Service: name, ContainerID: target.ContainerID, Lines: lines})
	}
	if len(services) == 0 {
		return
	}
	snapshot := FailureSnapshot{CapturedAt: time.Now().Unix(), Complete: true, Services: services}
	raw, e := json.Marshal(snapshot)
	if e != nil {
		return
	}
	if len(raw) > 256<<10 {
		snapshot.Complete = false
		for index := range snapshot.Services {
			if len(snapshot.Services[index].Lines) > 20 {
				snapshot.Services[index].Lines = snapshot.Services[index].Lines[len(snapshot.Services[index].Lines)-20:]
			}
		}
		raw, _ = json.Marshal(snapshot)
	}
	if e = w.DB.SaveDeploymentDiagnosis(work, d.ID, d.ApplicationID, snapshot.CapturedAt, raw); e != nil {
		slog.Warn("Deployment failure logs could not be saved", "error", e)
	}
}

func (w *Worker) captureComposeFailure(ctx context.Context, d store.Deployment, runtime *DockerRuntime) {
	work, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	items, e := runtime.Client.ContainerList(work, client.ContainerListOptions{All: true, Filters: make(client.Filters).Add("label", "com.docker.compose.project="+runtime.composeProject(d.ApplicationID))})
	if e != nil {
		return
	}
	targets := []failureTarget{}
	for _, item := range items.Items {
		service := item.Labels["com.docker.compose.service"]
		if service == "" && len(item.Names) > 0 {
			service = strings.TrimPrefix(item.Names[0], "/")
		}
		targets = append(targets, failureTarget{Service: service, ContainerID: item.ID})
	}
	if len(targets) > 0 {
		w.captureFailureLogs(work, d, targets)
	}
}
