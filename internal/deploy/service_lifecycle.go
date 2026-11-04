package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"shipper/internal/store"
)

func (r *DockerRuntime) LifecycleService(ctx context.Context, d store.Deployment, name, action string, logs ...func(string)) error {
	var plan composePlan
	if e := json.Unmarshal([]byte(d.ComposePlan), &plan); e != nil {
		return e
	}
	services, _ := plan.Project["services"].(map[string]any)
	if _, ok := services[name]; !ok {
		return fmt.Errorf("Service is unavailable")
	}
	if e := r.CheckServiceAction(ctx, d, name, action); e != nil {
		return e
	}
	args := []string{action, name}
	if action == "start" {
		wait := 90
		for _, svc := range d.Config.Services {
			if svc.Name == name {
				wait = max(wait, StartupSeconds(svc.Runtime))
			}
		}
		for _, h := range d.Config.Hooks {
			if h.Stage == "before_start" {
				wait = max(wait, 90+h.Command.Timeout)
			}
		}
		args = []string{"up", "-d", "--no-build", "--pull", "never", "--wait", "--wait-timeout", fmt.Sprint(wait), name}
	}
	emit := func(string) {}
	if len(logs) > 0 {
		emit = logs[0]
	}
	if e := r.composeCommand(ctx, d, args, emit); e != nil {
		return e
	}
	if action != "stop" {
		for _, svc := range d.Services {
			if svc.Name == name && svc.Port > 0 {
				id, e := r.taskService(ctx, d, name)
				if e != nil {
					return e
				}
				candidate := d
				candidate.ContainerID = id
				candidate.RuntimePort = svc.Port
				candidate.Config.HealthPath = svc.HealthPath
				for _, override := range d.Config.Services {
					if override.Name == name {
						candidate.Config.Runtime = override.Runtime
					}
				}
				return r.Healthy(ctx, candidate)
			}
		}
	}
	return nil
}

func (r *DockerRuntime) CheckServiceAction(ctx context.Context, d store.Deployment, name, action string) error {
	var plan composePlan
	if e := json.Unmarshal([]byte(d.ComposePlan), &plan); e != nil {
		return e
	}
	services, _ := plan.Project["services"].(map[string]any)
	if _, ok := services[name]; !ok {
		return fmt.Errorf("Service is unavailable")
	}
	if action != "start" {
		items, e := r.composeContainers(ctx, d.ApplicationID)
		if e != nil {
			return e
		}
		running := map[string]bool{}
		for _, c := range items {
			if c.State == "running" {
				running[c.Labels["com.docker.compose.service"]] = true
			}
		}
		var depends func(string, map[string]bool) bool
		depends = func(current string, seen map[string]bool) bool {
			if seen[current] {
				return false
			}
			seen[current] = true
			svc, ok := services[current].(map[string]any)
			if !ok {
				return false
			}
			deps, _ := svc["depends_on"].(map[string]any)
			for dep := range deps {
				if dep == name || depends(dep, seen) {
					return true
				}
			}
			return false
		}
		for other := range services {
			if other != name && running[other] && depends(other, map[string]bool{}) {
				return fmt.Errorf("Stop dependant service %s before changing %s", other, name)
			}
		}
	}
	return nil
}
