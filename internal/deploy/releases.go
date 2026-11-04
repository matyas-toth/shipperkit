package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/moby/moby/client"
	"shipper/internal/store"
	"strconv"
)

// Recovery deliberately requires retained local artifacts. It never rebuilds a
// historical branch or pulls a mutable image tag and calls that a rollback.
func (r *DockerRuntime) CheckRelease(ctx context.Context, d store.Deployment) error {
	images := []string{d.ImageID}
	if ReleaseMethod(d) == "compose" {
		var plan composePlan
		if json.Unmarshal([]byte(d.ComposePlan), &plan) != nil {
			return errors.New("The recorded Compose plan is unavailable")
		}
		services, ok := plan.Project["services"].(map[string]any)
		if !ok || len(services) == 0 {
			return errors.New("The recorded Compose services are unavailable")
		}
		images = nil
		for _, raw := range services {
			svc, ok := raw.(map[string]any)
			if !ok {
				return errors.New("Invalid recorded service")
			}
			image, _ := svc["image"].(string)
			images = append(images, image)
		}
		if volumes, ok := plan.Project["volumes"].(map[string]any); ok {
			for _, raw := range volumes {
				v, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				name, _ := v["name"].(string)
				if name != "" {
					if _, e := r.Client.VolumeInspect(ctx, name, client.VolumeInspectOptions{}); e != nil {
						return errors.New("A recorded volume is unavailable; recover it before rolling back")
					}
				}
			}
		}
	}
	for _, image := range images {
		if image == "" {
			return errors.New("No retained image is recorded")
		}
		if _, e := r.Client.ImageInspect(ctx, image); e != nil {
			return errors.New("A recorded image is unavailable; restore the artifact before rolling back")
		}
	}
	return nil
}

func prepareRollbackCompose(d *store.Deployment) error {
	var plan composePlan
	if e := json.Unmarshal([]byte(d.ComposePlan), &plan); e != nil {
		return e
	}
	services, ok := plan.Project["services"].(map[string]any)
	if !ok {
		return errors.New("Recorded services are unavailable")
	}
	for _, raw := range services {
		svc := raw.(map[string]any)
		labels, _ := svc["labels"].(map[string]any)
		if labels == nil {
			labels = map[string]any{}
		}
		labels["io.shipper.deployment.id"] = d.ID
		svc["labels"] = labels
	}
	// Failure recovery must preserve the state of the release being replaced.
	plan.PreviousDesiredState = "running"
	data, e := json.Marshal(plan)
	d.ComposePlan = string(data)
	return e
}

func (w *Worker) deploymentHooks(ctx context.Context, d store.Deployment, stage string, bestEffort bool) error {
	if d.Operation == "rollback" {
		return nil
	}
	return w.hooks(ctx, d, stage, bestEffort)
}

func applyComposePolicy(svc map[string]any, p store.RuntimePolicy) {
	if p.CPUs > 0 {
		svc["cpus"] = p.CPUs
		if deploy, ok := svc["deploy"].(map[string]any); ok {
			if resources, ok := deploy["resources"].(map[string]any); ok {
				if limits, ok := resources["limits"].(map[string]any); ok {
					delete(limits, "cpus")
				}
			}
		}
	}
	if p.PIDs > 0 {
		svc["pids_limit"] = p.PIDs
		if deploy, ok := svc["deploy"].(map[string]any); ok {
			if resources, ok := deploy["resources"].(map[string]any); ok {
				if limits, ok := resources["limits"].(map[string]any); ok {
					delete(limits, "pids")
				}
			}
		}
	}
	if p.Restart != "" {
		svc["restart"] = p.Restart
		if deploy, ok := svc["deploy"].(map[string]any); ok {
			delete(deploy, "restart_policy")
		}
	}
	if p.StopSeconds > 0 {
		svc["stop_grace_period"] = strconv.Itoa(p.StopSeconds) + "s"
	}
}
