package catalog

import (
	"context"
	"testing"

	"shipper/internal/deploy"
	"shipper/internal/store"
)

// Every manifest template must load through the same Compose loader used by a
// real deployment. This catches upstream templates that depend on files or
// host features Shipper will not accept.
func TestCatalogTemplatesLoad(t *testing.T) {
	failures := 0
	for _, service := range Embedded().Services {
		variables := []store.Variable{}
		for _, field := range service.Fields {
			value := field.Default
			switch {
			case field.Kind == "domain":
				value = "http://" + field.Service + ".localhost"
			case field.Generate != "":
				value = "generated-value-for-test"
			case field.Required && value == "":
				value = "required-value"
			}
			variables = append(variables, store.Variable{Name: field.Key, Value: value, Secret: field.Secret, Scope: "runtime"})
		}
		config := store.Config{
			Method: "compose", ComposeSource: "paste", ComposeYAML: service.Compose,
			ComposeFile: "compose.yaml", Variables: variables, StartupPolicy: "http",
		}
		root := t.TempDir()
		if _, err := deploy.LoadCompose(context.Background(), config, root, "catalog-"+service.ID); err != nil {
			failures++
			if failures <= 20 {
				t.Logf("%s: %v", service.ID, err)
			}
		}
	}
	if failures > 0 {
		t.Fatalf("%d catalog templates do not load", failures)
	}
}
