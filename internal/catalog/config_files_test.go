package catalog

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"
	"shipper/internal/deploy"
	"shipper/internal/store"
)

// fileTarget matches a mount target that names a single file.
var fileTarget = regexp.MustCompile(`(?i)[^/]\.(toml|ya?ml|json|conf|cfg|ini|env|js|ts|php|xml|sh|txt|lua|py|properties|rb|pem|key|crt|htpasswd|sqlite3?|db|sql|css|html)$|/(Caddyfile|nginx\.conf)$`)

// A volume mounted on a file path is an empty directory, which is how Garage
// failed with "Is a directory". Template files must be Compose configs.
func TestCatalogMountsFilesAsConfigs(t *testing.T) {
	for _, service := range Embedded().Services {
		var doc struct {
			Services map[string]struct {
				Volumes []any `yaml:"volumes"`
				Configs []struct {
					Source string `yaml:"source"`
				} `yaml:"configs"`
			} `yaml:"services"`
			Configs map[string]struct {
				Content *string `yaml:"content"`
			} `yaml:"configs"`
		}
		if err := yaml.Unmarshal([]byte(service.Compose), &doc); err != nil {
			t.Fatalf("%s: %v", service.ID, err)
		}
		for name, svc := range doc.Services {
			for _, raw := range svc.Volumes {
				volume, _ := raw.(map[string]any)
				target, _ := volume["target"].(string)
				if volume["type"] == "volume" && fileTarget.MatchString(target) {
					t.Errorf("%s/%s mounts a volume on the file %s", service.ID, name, target)
				}
			}
			for _, config := range svc.Configs {
				if definition, ok := doc.Configs[config.Source]; !ok || definition.Content == nil {
					t.Errorf("%s/%s uses config %s without inline content", service.ID, name, config.Source)
				}
			}
		}
	}
}

// Template files reach the container exactly as upstream wrote them: runtime
// references such as nginx's $host survive Compose interpolation, and scripts
// stay executable.
func TestCatalogConfigFilesAreVerbatim(t *testing.T) {
	for _, tc := range []struct {
		id, target, contains string
		executable           bool
	}{
		{"garage", "/etc/garage.toml", `rpc_secret_file = "env:GARAGE_RPC_SECRET"`, false},
		{"appflowy", "/etc/nginx/nginx.conf", "proxy_set_header Host $http_host", false},
		{"docker-registry", "/entrypoint.sh", "$USERNAME", true},
	} {
		service, ok := Embedded().Get(tc.id)
		if !ok {
			t.Fatalf("%s is missing", tc.id)
		}
		variables := []store.Variable{}
		for _, field := range service.Fields {
			value := field.Default
			if field.Generate != "" || (field.Required && value == "") {
				value = "value-for-test"
			}
			variables = append(variables, store.Variable{Name: field.Key, Value: value, Scope: "runtime"})
		}
		config := store.Config{Method: "compose", ComposeSource: "paste", ComposeYAML: service.Compose, ComposeFile: "compose.yaml", Variables: variables, StartupPolicy: "http"}
		project, err := deploy.LoadCompose(context.Background(), config, t.TempDir(), "catalog-"+tc.id)
		if err != nil {
			t.Fatalf("%s: %v", tc.id, err)
		}
		found := false
		for _, svc := range project.Services {
			for _, ref := range svc.Configs {
				if ref.Target != tc.target {
					continue
				}
				found = true
				if content := project.Configs[ref.Source].Content; !strings.Contains(content, tc.contains) {
					t.Errorf("%s %s lost %q:\n%s", tc.id, tc.target, tc.contains, content)
				}
				if executable := ref.Mode != nil && *ref.Mode&0o100 != 0; executable != tc.executable {
					t.Errorf("%s %s executable %v", tc.id, tc.target, executable)
				}
			}
		}
		if !found {
			t.Errorf("%s does not mount %s", tc.id, tc.target)
		}
	}
}

// An access key is a credential. Garage rejects one shorter than 8
// characters, and a readable default such as "minio" is guessable, so every
// access key a template takes from a field is generated.
func TestCatalogGeneratesAccessKeys(t *testing.T) {
	accessKey := regexp.MustCompile(`(?i)ACCESS_?KEY|KEY_ID|MINIO_ROOT_USER`)
	reference := regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)`)
	for _, service := range Embedded().Services {
		generated := map[string]bool{}
		fields := map[string]bool{}
		for _, field := range service.Fields {
			fields[field.Key] = true
			generated[field.Key] = field.Generate != ""
		}
		var doc struct {
			Services map[string]struct {
				Environment map[string]any `yaml:"environment"`
			} `yaml:"services"`
		}
		if err := yaml.Unmarshal([]byte(service.Compose), &doc); err != nil {
			t.Fatalf("%s: %v", service.ID, err)
		}
		for name, svc := range doc.Services {
			for key, value := range svc.Environment {
				if !accessKey.MatchString(key) {
					continue
				}
				for _, match := range reference.FindAllStringSubmatch(fmt.Sprint(value), -1) {
					if fields[match[1]] && strings.Contains(match[1], "USER") && !generated[match[1]] {
						t.Errorf("%s/%s takes %s from %s, which is not generated", service.ID, name, key, match[1])
					}
				}
			}
		}
	}
}
