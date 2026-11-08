package deploy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type SourceInspection struct {
	Method         string   `json:"method"`
	ComposeFile    string   `json:"composeFile,omitempty"`
	Commit         string   `json:"commit,omitempty"`
	InstallCommand string   `json:"installCommand"`
	BuildCommand   string   `json:"buildCommand"`
	StartCommand   string   `json:"startCommand"`
	Variables      []string `json:"variables"`
}

func InspectSource(root string) SourceInspection {
	out := SourceInspection{Method: DetectMethod(root), Variables: []string{}}
	for _, name := range []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"} {
		if _, e := readInspectionFile(root, name); e == nil {
			out.ComposeFile = name
			break
		}
	}
	if raw, e := readInspectionFile(root, "package.json"); e == nil {
		var pkg struct {
			Scripts map[string]string `json:"scripts"`
		}
		if json.Unmarshal(raw, &pkg) == nil {
			manager := "npm"
			if _, e := readInspectionFile(root, "pnpm-lock.yaml"); e == nil {
				manager = "pnpm"
			} else if _, e := readInspectionFile(root, "yarn.lock"); e == nil {
				manager = "yarn"
			} else if _, e := readInspectionFile(root, "bun.lock"); e == nil {
				manager = "bun"
			}
			out.InstallCommand = manager + " install"
			if manager == "npm" {
				if _, e := readInspectionFile(root, "package-lock.json"); e == nil {
					out.InstallCommand = "npm ci"
				}
			}
			if pkg.Scripts["build"] != "" {
				out.BuildCommand = manager + " run build"
			}
			if pkg.Scripts["start"] != "" {
				out.StartCommand = manager + " run start"
			}
		}
	}
	names := map[string]bool{}
	for _, name := range []string{".env.example", ".env.sample", ".env.template", ".env.dist"} {
		raw, e := readInspectionFile(root, name)
		if e != nil {
			continue
		}
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "export "))
			key, _, ok := strings.Cut(line, "=")
			key = strings.TrimSpace(key)
			if ok && variableName.MatchString(key) {
				names[key] = true
			}
		}
	}
	for name := range names {
		out.Variables = append(out.Variables, name)
	}
	sort.Strings(out.Variables)
	if len(out.Variables) > 128 {
		out.Variables = out.Variables[:128]
	}
	return out
}
func readInspectionFile(root, name string) ([]byte, error) {
	file, e := checkedFile(root, filepath.Join(root, name))
	if e != nil {
		return nil, e
	}
	info, e := os.Stat(file)
	if e != nil {
		return nil, e
	}
	if info.Size() > 128<<10 {
		return nil, os.ErrInvalid
	}
	return os.ReadFile(file)
}
