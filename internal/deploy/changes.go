package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"shipper/internal/store"
	"sort"
)

type ChangePlan struct {
	Operation string   `json:"operation"`
	Changes   []string `json:"changes"`
	Revision  string   `json:"revision"`
}

func buildConfig(c store.Config) store.Config {
	c.AutoDeploy = store.AutoDeploy{}
	c.Runtime = store.RuntimePolicy{}
	c.Hooks = nil
	c.StartupPolicy = ""
	c.StartCommand = ""
	c.Port = 0
	c.HealthPath = ""
	c.MemoryMB = 0
	c.Volumes = nil
	c.VolumeSources = nil
	c.Domains = nil
	c.Domain = ""
	c.HTTPS = false
	c.ComposeKeep = false
	c.ComposeHasValue = false
	vars := []store.Variable{}
	for _, v := range c.Variables {
		if c.Method == "compose" || v.Scope == "build" || v.Scope == "both" {
			v.Keep = false
			v.KeepFrom = ""
			v.HasValue = false
			vars = append(vars, v)
		}
	}
	sort.Slice(vars, func(i, j int) bool { return vars[i].Name < vars[j].Name })
	c.Variables = vars
	c.Services = nil
	return c
}
func comparisonConfig(c store.Config) store.Config {
	// Historical releases can predate optional defaults. Compare effective values
	// so opening a legacy form does not manufacture a list of pending changes.
	data, _ := json.Marshal(c)
	json.Unmarshal(data, &c)
	_ = ValidateBuild(&c)
	if c.StartupPolicy == "" {
		c.StartupPolicy = "http"
	}
	if c.Variables == nil {
		c.Variables = []store.Variable{}
	}
	if c.Services == nil {
		c.Services = []store.ServiceConfig{}
	}
	if c.Hooks == nil {
		c.Hooks = []store.Hook{}
	}
	if c.Volumes == nil {
		c.Volumes = []store.Volume{}
	}
	c.Domain = ""
	c.HTTPS = false
	c.Domains = nil
	sort.Slice(c.Variables, func(i, j int) bool { return c.Variables[i].Name < c.Variables[j].Name })
	sort.Slice(c.Services, func(i, j int) bool { return c.Services[i].Name < c.Services[j].Name })
	sort.Slice(c.Hooks, func(i, j int) bool {
		return c.Hooks[i].Stage+c.Hooks[i].Command.Service < c.Hooks[j].Stage+c.Hooks[j].Command.Service
	})
	sort.Slice(c.Volumes, func(i, j int) bool { return c.Volumes[i].Name < c.Volumes[j].Name })
	return c
}
func PlanChanges(old, next store.Config) ChangePlan {
	old = comparisonConfig(old)
	next = comparisonConfig(next)
	p := ChangePlan{Operation: "recreate", Changes: []string{}}
	for key := range old.VolumeSources {
		if next.VolumeSources[key] == "" {
			p.Operation = "rebuild"
		}
	}
	if !reflect.DeepEqual(buildConfig(old), buildConfig(next)) {
		p.Operation = "rebuild"
	}
	if next.Method == "compose" {
		for _, svc := range old.Services {
			var found *store.ServiceConfig
			for i := range next.Services {
				if next.Services[i].Name == svc.Name {
					found = &next.Services[i]
				}
			}
			if found == nil {
				p.Operation = "rebuild"
				continue
			}
			for _, v := range svc.Variables {
				exists := false
				for _, n := range found.Variables {
					if n.Name == v.Name {
						exists = true
					}
				}
				if !exists {
					p.Operation = "rebuild"
				}
			}
			if svc.Runtime.CPUs > 0 && found.Runtime.CPUs == 0 || svc.Runtime.PIDs > 0 && found.Runtime.PIDs == 0 || svc.Runtime.Restart != "" && found.Runtime.Restart == "" || svc.Runtime.StopSeconds > 0 && found.Runtime.StopSeconds == 0 {
				p.Operation = "rebuild"
			}
			if !reflect.DeepEqual(svc.UnsetVariables, found.UnsetVariables) || svc.MemoryMB > 0 && found.MemoryMB == 0 || svc.Port != nil && found.Port == nil {
				p.Operation = "rebuild"
			}
		}
	}
	a, _ := json.Marshal(store.PublicConfig(old))
	b, _ := json.Marshal(store.PublicConfig(next))
	var left, right map[string]any
	json.Unmarshal(a, &left)
	json.Unmarshal(b, &right)
	labels := map[string]string{"runtime": "Runtime policy", "repository": "Repository", "branch": "Revision", "method": "Deployment method", "rootDirectory": "Application directory", "dockerfile": "Dockerfile", "installCommand": "Install command", "buildCommand": "Build command", "startCommand": "Start command", "publishDirectory": "Publish directory", "routing": "Static routing", "htaccess": ".htaccess", "port": "Container port", "healthPath": "Health check", "memoryMB": "Memory limit", "volumes": "Volumes", "hooks": "Lifecycle commands", "startupPolicy": "Readiness", "composeFile": "Compose file", "composeSource": "Compose source", "composeYAML": "Compose source", "autoDeploy": "Automatic deployments", "image": "Image", "registryId": "Registry", "platform": "Platform", "inlineDockerfile": "Dockerfile source", "sourceId": "Source connection", "services": "Service configuration"}
	for key, label := range labels {
		if !reflect.DeepEqual(left[key], right[key]) {
			p.Changes = append(p.Changes, label)
		}
	}
	if !reflect.DeepEqual(old.Variables, next.Variables) {
		p.Changes = append(p.Changes, "Environment variables")
	}
	sort.Strings(p.Changes)
	// Secret values are deliberately excluded from public diffs but still affect
	// both operation planning and the presence of a pending change.
	oldWithoutAuto, nextWithoutAuto := old, next
	oldWithoutAuto.AutoDeploy = store.AutoDeploy{}
	nextWithoutAuto.AutoDeploy = store.AutoDeploy{}
	if reflect.DeepEqual(oldWithoutAuto, nextWithoutAuto) && !reflect.DeepEqual(old.AutoDeploy, next.AutoDeploy) {
		p.Operation = "metadata"
	}
	if reflect.DeepEqual(old, next) {
		p.Operation = "none"
	}
	if p.Operation != "none" && len(p.Changes) == 0 {
		p.Changes = append(p.Changes, "Configuration")
	}
	return p
}
func (r *DockerRuntime) ReuseCompose(ctx context.Context, d *store.Deployment, previous store.Deployment) error {
	var plan composePlan
	if e := json.Unmarshal([]byte(previous.ComposePlan), &plan); e != nil {
		return e
	}
	if e := r.reuseVolumeSources(ctx, *d, &plan); e != nil {
		return e
	}
	services, ok := plan.Project["services"].(map[string]any)
	if !ok {
		return fmt.Errorf("Previous Compose configuration is unavailable")
	}
	d.Services = append([]store.Service{}, previous.Services...)
	for name, raw := range services {
		svc := raw.(map[string]any)
		// Recover the original entrypoint before replacing/removing a startup hook.
		if entry, ok := svc["entrypoint"].([]any); ok && len(entry) >= 3 && entry[0] == startupPath {
			svc["entrypoint"] = entry[2:]
			svc["command"] = []string{}
		}
		env, _ := svc["environment"].(map[string]any)
		if env == nil {
			env = map[string]any{}
		}
		for _, old := range previous.Config.Services {
			if old.Name == name {
				for _, v := range old.Variables {
					delete(env, v.Name)
				}
			}
		}
		for _, override := range d.Config.Services {
			if override.Name != name {
				continue
			}
			applyComposePolicy(svc, override.Runtime)
			for _, key := range override.UnsetVariables {
				delete(env, key)
			}
			for _, v := range override.Variables {
				env[v.Name] = v.Value
				plan.Masks = append(plan.Masks, v.Value)
			}
			if override.MemoryMB > 0 {
				svc["mem_limit"] = override.MemoryMB << 20
				delete(svc, "deploy")
			}
			for i := range d.Services {
				if d.Services[i].Name == name {
					if override.Port != nil {
						d.Services[i].Port = *override.Port
					}
					if override.HealthPath != "" {
						d.Services[i].HealthPath = override.HealthPath
					}
				}
			}
		}
		svc["environment"] = env
		for i := range d.Services {
			if d.Services[i].Name == name {
				vars := []store.Variable{}
				for key, value := range env {
					if text, ok := value.(string); ok {
						vars = append(vars, store.Variable{Name: key, Value: text, Secret: true, Scope: "runtime"})
					}
				}
				d.Services[i].Variables = store.PublicConfig(store.Config{Variables: vars}).Variables
				networks, _ := svc["networks"].(map[string]any)
				if networks == nil {
					networks = map[string]any{}
				}
				if d.Services[i].Port > 0 {
					networks["shipper_edge"] = map[string]any{"aliases": []string{composeAlias(d.ApplicationID, name)}}
				} else {
					delete(networks, "shipper_edge")
				}
				svc["networks"] = networks
			}
		}
	}
	b, e := json.Marshal(plan)
	if e != nil {
		return e
	}
	d.ComposePlan = string(b)
	d.ImageID = "compose"
	d.Commit = previous.Commit
	d.ResolvedMethod = "compose"
	return r.composeStartup(ctx, d)
}
