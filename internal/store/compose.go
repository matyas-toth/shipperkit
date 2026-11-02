package store

import (
	"encoding/json"
	"sort"
)

// Effective Compose values are available only inside the authenticated reveal
// and mutation paths. Public service summaries carry names and retained flags.
func ComposeVariables(d Deployment, service string) []Variable {
	var p struct {
		Project struct {
			Services map[string]struct {
				Environment map[string]*string `json:"environment"`
			} `json:"services"`
		} `json:"project"`
	}
	if json.Unmarshal([]byte(d.ComposePlan), &p) != nil {
		return nil
	}
	values := p.Project.Services[service].Environment
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	out := []Variable{}
	for _, name := range names {
		if values[name] != nil {
			out = append(out, Variable{Name: name, Value: *values[name], Secret: true, Scope: "runtime"})
		}
	}
	return out
}

func MergeReleaseVariables(next *Config, old Config, release Deployment) error {
	old.Services = append([]ServiceConfig(nil), old.Services...)
	for _, svc := range release.Services {
		index := -1
		for i := range old.Services {
			if old.Services[i].Name == svc.Name {
				index = i
				break
			}
		}
		if index < 0 {
			old.Services = append(old.Services, ServiceConfig{Name: svc.Name})
			index = len(old.Services) - 1
		}
		values := append([]Variable(nil), old.Services[index].Variables...)
		for _, v := range ComposeVariables(release, svc.Name) {
			found := false
			for _, existing := range values {
				if existing.Name == v.Name {
					found = true
				}
			}
			if !found {
				values = append(values, v)
			}
		}
		old.Services[index].Variables = values
	}
	return MergeVariables(next, old)
}
