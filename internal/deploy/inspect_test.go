package deploy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceSuggestionsDoNotImportValues(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"scripts":{"start":"node server.js","build":"vite build"}}`), 0600)
	os.WriteFile(filepath.Join(root, "package-lock.json"), []byte("{}"), 0600)
	os.WriteFile(filepath.Join(root, ".env.example"), []byte("export DATABASE_URL=postgres://a-real-secret\nAPI_TOKEN=another-secret\n# comment\n"), 0600)
	out := InspectSource(root)
	if out.InstallCommand != "npm ci" || out.BuildCommand != "npm run build" || out.StartCommand != "npm run start" || strings.Join(out.Variables, ",") != "API_TOKEN,DATABASE_URL" {
		t.Fatal(out)
	}
}

func TestInspectionDetectsComposeBeforeIndividualBuildFiles(t *testing.T) {
	for _, name := range []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			os.WriteFile(filepath.Join(root, "Dockerfile"), []byte("FROM nginx"), 0600)
			os.WriteFile(filepath.Join(root, name), []byte("services:\n  web:\n    build: .\n"), 0600)
			out := InspectSource(root)
			if out.Method != "compose" || out.ComposeFile != name {
				t.Fatal(out)
			}
		})
	}
}
