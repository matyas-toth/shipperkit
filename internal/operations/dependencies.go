package operations

import (
	"encoding/json"
	"sort"

	"shipper/internal/store"
)

type Dependency struct {
	Service    string `json:"service"`
	Dependency string `json:"dependency"`
	State      string `json:"state"`
	ObservedAt int64  `json:"observedAt"`
	Protocol   string `json:"protocol"`
}

// Network membership is a prerequisite check, never proof of a successful
// database transaction or arbitrary application protocol.
func Dependencies(d store.Deployment, facts []store.Observation) []Dependency {
	var plan struct {
		Project struct {
			Services map[string]struct {
				DependsOn map[string]any `json:"depends_on"`
			} `json:"services"`
		} `json:"project"`
	}
	_ = json.Unmarshal([]byte(d.ComposePlan), &plan)
	type runtime struct {
		ExitCode int      `json:"exitCode"`
		Service  string   `json:"service"`
		Networks []string `json:"networks"`
		State    string   `json:"state"`
	}
	out := []Dependency{}
	for name, service := range plan.Project.Services {
		for target, condition := range service.DependsOn {
			v := Dependency{Service: name, Dependency: target, State: "unknown", Protocol: "not_checked"}
			left, right := []runtime{}, []runtime{}
			for _, fact := range facts {
				if fact.Kind != "service" || fact.Stale {
					continue
				}
				var r runtime
				if json.Unmarshal(fact.Details, &r) != nil {
					continue
				}
				if r.Service == name {
					left = append(left, r)
					v.ObservedAt = max(v.ObservedAt, fact.ObservedAt)
				}
				if r.Service == target {
					right = append(right, r)
					v.ObservedAt = max(v.ObservedAt, fact.ObservedAt)
				}
			}
			if len(left) > 0 && len(right) > 0 {
				v.State = "failed"
				for _, a := range left {
					for _, b := range right {
						for _, network := range a.Networks {
							for _, other := range b.Networks {
								if network == other && a.State == "running" && b.State == "running" {
									v.State = "healthy"
								}
							}
						}
					}
				}
			}
			if rule, ok := condition.(map[string]any); ok && rule["condition"] == "service_completed_successfully" {
				v.State = "unknown"
				v.Protocol = "not_applicable"
				for _, r := range right {
					if r.State == "exited" {
						v.State = "failed"
						if r.ExitCode == 0 {
							v.State = "completed"
						}
					}
				}
			}
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Service+out[i].Dependency < out[j].Service+out[j].Dependency })
	return out
}
