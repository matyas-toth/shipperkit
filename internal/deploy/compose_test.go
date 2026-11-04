package deploy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"shipper/internal/store"
)

func TestComposeReferenceLoader(t *testing.T) {
	root := t.TempDir()
	content := "services:\n  web:\n    image: nginx:alpine\n    ports: ['8080:80']\n    environment:\n      TOKEN: ${TOKEN:?TOKEN is required}\n  db:\n    image: postgres:17-alpine\n    ports: ['5432:5432']\n    volumes: ['data:/var/lib/postgresql/data']\nvolumes:\n  data:\n"
	c := store.Config{Method: "compose", ComposeSource: "paste", ComposeYAML: content, Variables: []store.Variable{{Name: "TOKEN", Value: "a$literal", Secret: true}}}
	p, e := LoadCompose(context.Background(), c, root, "shipper-unit")
	if e != nil {
		t.Fatal(e)
	}
	if p.Name != "shipper-unit" || *p.Services["web"].Environment["TOKEN"] != "a$literal" {
		t.Fatal("Compose interpolation or project identity changed")
	}
	services := ComposeServices(p, c)
	if len(services) != 2 || services[0].Name != "db" || services[1].Port != 80 {
		t.Fatal(services)
	}
	c.Services = []store.ServiceConfig{{Name: "web", Variables: []store.Variable{{Name: "EXTRA", Value: "value"}}}}
	if ComposeServices(p, c)[1].Port != 80 {
		t.Fatal("An environment-only edit changed the inherited HTTP port")
	}
	c.Services[0].Port = ptr(0)
	if ComposeServices(p, c)[1].Port != 0 {
		t.Fatal("An explicit internal-only override was ignored")
	}
	c.Services = nil
	// Shell variables and the process's Docker settings never participate.
	t.Setenv("TOKEN", "host-secret")
	c.Variables = nil
	if _, e = LoadCompose(context.Background(), c, root, "shipper-unit"); e == nil {
		t.Fatal("Read host environment")
	}
	for _, bad := range []string{
		"include: /etc/passwd\nservices: {}",
		"services:\n  web:\n    extends: {file: /etc/passwd, service: web}",
		"services:\n  web:\n    image: nginx\n    env_file: ../outside.env",
		"services:\n  web:\n    image: nginx\n    volumes: ['/var/run/docker.sock:/var/run/docker.sock']",
	} {
		c.ComposeYAML = bad
		if _, e = LoadCompose(context.Background(), c, root, "shipper-unit"); e == nil {
			t.Fatalf("Unsafe file reference accepted: %s", bad)
		}
	}
}

func TestComposeRepositoryFiles(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "deploy"), 0700)
	os.WriteFile(filepath.Join(root, "deploy", "compose.yml"), []byte("services:\n  web:\n    image: nginx\n    env_file: config.env\n    ports: ['8080:80']\n"), 0600)
	os.WriteFile(filepath.Join(root, "config.env"), []byte("VALUE=from-file\n"), 0600)
	c := store.Config{Method: "compose", ComposeSource: "git", ComposeFile: "deploy/compose.yml"}
	p, e := LoadCompose(context.Background(), c, root, "shipper-files")
	if e != nil {
		t.Fatal(e)
	}
	if p.Services["web"].Environment["VALUE"] == nil || *p.Services["web"].Environment["VALUE"] != "from-file" {
		t.Fatal("Project directory semantics changed")
	}
	// A monorepo project directory may reference siblings inside its checkout.
	os.WriteFile(filepath.Join(root, "deploy", "compose.yml"), []byte("services:\n  web:\n    image: nginx\n    env_file: ../config.env\n"), 0600)
	c.ComposeFile = "compose.yml"
	p, e = LoadCompose(context.Background(), c, filepath.Join(root, "deploy"), "shipper-siblings", root)
	if e != nil || p.Services["web"].Environment["VALUE"] == nil {
		t.Fatal("Repository sibling path failed", e)
	}

	c.ComposeFile = "../escape.yml"
	if _, e = LoadCompose(context.Background(), c, root, "shipper-files"); e == nil {
		t.Fatal("Compose path escaped root")
	}
}

func TestComposeLiteralAndSecretMasking(t *testing.T) {
	out := escapeCompose(map[string]any{"command": []any{"echo", "$TOKEN $$ $(hello)"}}).(map[string]any)
	if out["command"].([]any)[1] != "$$TOKEN $$$$ $$(hello)" {
		t.Fatal(out)
	}
	d := store.Deployment{ComposePlan: `{"masks":["inline-password"]}`, Config: store.Config{Services: []store.ServiceConfig{{Name: "web", Variables: []store.Variable{{Name: "TOKEN", Value: "service-secret", Secret: true}}}}}}
	if got := RedactDeployment(d, "inline-password service-secret"); strings.Contains(got, "password") || strings.Contains(got, "service-secret") {
		t.Fatal(got)
	}
}
