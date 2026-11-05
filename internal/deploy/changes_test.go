package deploy

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"shipper/internal/store"
)

func TestPlanNamesInlineSourceEditsWithoutRevealingContents(t *testing.T) {
	for _, tt := range []struct {
		label string
		old   store.Config
		next  store.Config
	}{
		{
			label: "Compose source",
			old:   store.Config{Method: "compose", ComposeSource: "paste", ComposeYAML: "services:\n  web:\n    image: nginx:alpine\n"},
			next:  store.Config{Method: "compose", ComposeSource: "paste", ComposeYAML: "services:\n  web:\n    image: nginx:alpine\n    environment: {TOKEN: fixture-only-secret}\n"},
		},
		{
			label: "Dockerfile source",
			old:   store.Config{Method: "inline", InlineDockerfile: "FROM alpine\n"},
			next:  store.Config{Method: "inline", InlineDockerfile: "FROM alpine\nENV TOKEN=fixture-only-secret\n"},
		},
	} {
		t.Run(tt.label, func(t *testing.T) {
			plan := PlanChanges(tt.old, tt.next)
			if plan.Operation != "rebuild" || !slices.Contains(plan.Changes, tt.label) {
				t.Fatalf("inline source edit was not identified: %+v", plan)
			}
			data, err := json.Marshal(plan)
			if err != nil || strings.Contains(string(data), "fixture-only-secret") {
				t.Fatal("change plan must not expose inline source contents")
			}
		})
	}
}
