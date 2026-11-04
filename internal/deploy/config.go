package deploy

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/net/publicsuffix"
	"shipper/internal/auth"
	"shipper/internal/store"
)

func Method(c store.Config) string {
	if c.Method == "" {
		return "static"
	}
	return c.Method
}
func ReleaseMethod(d store.Deployment) string {
	if d.ResolvedMethod != "" {
		return d.ResolvedMethod
	}
	return Method(d.Config)
}
func ReleasePort(d store.Deployment) int {
	if d.RuntimePort > 0 {
		return d.RuntimePort
	}
	c := d.Config
	c.Method = ReleaseMethod(d)
	return Port(c)
}
func Port(c store.Config) int {
	if c.Port > 0 {
		return c.Port
	}
	if Method(c) == "static" {
		return 8080
	}
	return 3000
}
func relativeDirectory(raw string) (string, error) {
	if raw == "" {
		return ".", nil
	}
	clean := path.Clean(raw)
	if len(raw) > 256 || strings.ContainsAny(raw, "\\\x00\r\n:") || path.IsAbs(raw) || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", errors.New("Directories must stay inside the repository")
	}
	return clean, nil
}

var variableName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,127}$`)
var volumeName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)

func ValidateBuild(c *store.Config) error {
	if e := ValidateRuntime(c.Runtime); e != nil {
		return e
	}
	if c.AutoDeploy.Enabled && c.SourceID == "" {
		return errors.New("Connect a GitHub source before enabling push deployments")
	}
	if len(c.AutoDeploy.Paths) > 32 {
		return errors.New("Use at most 32 watch paths")
	}
	for _, pattern := range c.AutoDeploy.Paths {
		if len(pattern) > 256 || strings.HasPrefix(pattern, "/") || strings.Contains(pattern, "..") || strings.ContainsAny(pattern, "\\\x00\r\n") {
			return errors.New("Watch paths must stay inside the repository")
		}
		if _, e := path.Match(pattern, ""); e != nil {
			return errors.New("Invalid watch path pattern")
		}
	}
	if e := ValidateHooks(c); e != nil {
		return e
	}
	if c.Method == "" {
		c.Method = "static"
	}
	if c.Method != "auto" && c.Method != "static" && c.Method != "dockerfile" && c.Method != "railpack" && c.Method != "compose" && c.Method != "image" && c.Method != "inline" {
		return errors.New("Choose Automatic, Static site, Dockerfile, Railpack, or Docker Compose")
	}
	if c.Method == "compose" {
		if c.ComposeSource == "" {
			c.ComposeSource = "git"
		}
		if c.ComposeSource != "git" && c.ComposeSource != "paste" {
			return errors.New("Choose a Git repository or pasted Compose YAML")
		}
		if c.ComposeFile == "" {
			c.ComposeFile = "compose.yaml"
		}
		if _, e := relativeDirectory(c.ComposeFile); e != nil {
			return e
		}
		if len(c.ComposeYAML) > 128<<10 {
			return errors.New("Compose YAML is limited to 128 KiB")
		}
		if c.ComposeSource == "paste" && c.ComposeYAML == "" {
			return errors.New("Enter Compose YAML")
		}
		if len(c.Services) > 32 {
			return errors.New("Compose applications support up to 32 service overrides")
		}
		names := map[string]bool{}
		for i := range c.Services {
			svc := &c.Services[i]
			if !serviceName.MatchString(svc.Name) || names[svc.Name] {
				return errors.New("Choose unique Compose service names")
			}
			names[svc.Name] = true
			if len(svc.UnsetVariables) > 128 {
				return errors.New("Too many removed service variables")
			}
			for _, name := range svc.UnsetVariables {
				if !variableName.MatchString(name) {
					return errors.New("Invalid service variable name")
				}
			}
			servicePort := 0
			if svc.Port != nil {
				servicePort = *svc.Port
			}
			inner := store.Config{Runtime: svc.Runtime, Method: "railpack", Port: servicePort, HealthPath: svc.HealthPath, MemoryMB: svc.MemoryMB, Variables: svc.Variables}
			if e := ValidateBuild(&inner); e != nil {
				return fmt.Errorf("Service %s: %w", svc.Name, e)
			}
			svc.HealthPath = inner.HealthPath
			svc.Variables = inner.Variables
		}
	}
	var e error
	c.RootDirectory, e = relativeDirectory(c.RootDirectory)
	if e != nil {
		return e
	}
	c.Dockerfile, e = relativeDirectory(c.Dockerfile)
	if e != nil {
		return e
	}
	if c.Dockerfile == "." {
		c.Dockerfile = "Dockerfile"
	}
	if c.Port < 0 || c.Port > 65535 {
		return errors.New("Container port must be between 1 and 65535")
	}
	if c.Method == "static" && c.Port != 0 && c.Port != 8080 {
		return errors.New("Static sites use container port 8080")
	}
	if (c.Method == "dockerfile" || c.Method == "inline") && (c.InstallCommand != "" || c.BuildCommand != "") {
		return errors.New("Set install and build commands in the Dockerfile")
	}
	if c.HealthPath == "" {
		c.HealthPath = "/"
	}
	u, e := url.Parse(c.HealthPath)
	if e != nil || !strings.HasPrefix(c.HealthPath, "/") || strings.HasPrefix(c.HealthPath, "//") || u.Host != "" || u.Fragment != "" || strings.ContainsAny(c.HealthPath, "\r\n\x00") || len(c.HealthPath) > 512 {
		return errors.New("Enter a relative HTTP health check path starting with /")
	}
	for _, command := range []string{c.InstallCommand, c.BuildCommand, c.StartCommand} {
		if len(command) > 4096 || strings.ContainsRune(command, 0) {
			return errors.New("Commands must be at most 4096 characters without null bytes")
		}
	}
	if c.MemoryMB == 0 {
		if c.Method == "static" {
			c.MemoryMB = 128
		} else {
			c.MemoryMB = 512
		}
	}
	if c.MemoryMB < 64 || c.MemoryMB > 8192 {
		return errors.New("Memory must be between 64 and 8192 MiB")
	}
	if len(c.Variables) > 128 {
		return errors.New("Applications support up to 128 variables")
	}
	names := map[string]bool{}
	for i := range c.Variables {
		v := &c.Variables[i]
		v.Name = strings.TrimSpace(v.Name)
		if !variableName.MatchString(v.Name) || names[v.Name] {
			return errors.New("Variable names must be unique letters, numbers, and underscores, starting with a letter or underscore")
		}
		names[v.Name] = true
		if v.Scope == "" {
			v.Scope = "runtime"
		}
		if v.Scope != "runtime" && v.Scope != "build" && v.Scope != "both" {
			return errors.New("Choose a valid variable scope")
		}
		if len(v.Value) > 16384 || strings.ContainsRune(v.Value, 0) {
			return fmt.Errorf("Variable %s is too large or contains null bytes", v.Name)
		}
	}
	if len(c.Volumes) > 12 {
		return errors.New("Applications support up to 12 persistent volumes")
	}
	paths := map[string]bool{}
	names = map[string]bool{}
	for _, v := range c.Volumes {
		if !volumeName.MatchString(v.Name) || names[v.Name] {
			return errors.New("Volume names must be unique lowercase letters, numbers, and hyphens")
		}
		names[v.Name] = true
		p := path.Clean(v.Path)
		if p != v.Path || !strings.HasPrefix(p, "/") || len(p) > 256 || paths[p] || strings.ContainsAny(p, "\\\r\n\x00:") {
			return errors.New("Enter unique absolute container paths for volumes")
		}
		for _, blocked := range []string{"/", "/proc", "/sys", "/dev", "/etc", "/usr", "/bin", "/sbin", "/var/run"} {
			if p == blocked || (blocked != "/" && strings.HasPrefix(p, blocked+"/")) {
				return errors.New("Volumes cannot replace system directories or Docker sockets")
			}
		}
		paths[p] = true
	}
	return nil
}
func NormalizeDomains(input []store.Domain) ([]store.Domain, error) {
	if len(input) > 32 {
		return nil, errors.New("Applications support up to 32 domains")
	}
	out := make([]store.Domain, 0, len(input))
	hosts := map[string]bool{}
	ids := map[string]bool{}
	primary := map[string]int{}
	for _, d := range input {
		if d.Service != "" && !serviceName.MatchString(d.Service) {
			return nil, errors.New("Choose a valid service for each domain")
		}
		if e := ValidateRoute(d); e != nil {
			return nil, e
		}
		d.CertificateExpiresAt = 0
		d.CertificateIssuer = ""
		d.VerificationState = ""
		d.Message = ""
		d.CheckedAt = 0
		d.Hostname = strings.ToLower(strings.TrimSpace(d.Hostname))
		if !domainPattern.MatchString(d.Hostname) || len(d.Hostname) > 253 || net.ParseIP(d.Hostname) != nil {
			return nil, errors.New("Enter a hostname without a protocol, port, or path")
		}
		if hosts[d.Hostname] {
			return nil, errors.New("Each domain can only be added once")
		}
		hosts[d.Hostname] = true
		if d.ID == "" {
			d.ID = auth.Token()[:16]
		}
		if !regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`).MatchString(d.ID) || ids[d.ID] {
			return nil, errors.New("Domain identifiers must be unique")
		}
		ids[d.ID] = true
		if d.HTTPS && strings.HasSuffix(d.Hostname, ".localhost") {
			return nil, errors.New("Use HTTP for local .localhost addresses")
		}
		if d.Primary {
			primary[d.Service]++
			d.Redirect = false
		}
		out = append(out, d)
	}
	for _, count := range primary {
		if count > 1 {
			return nil, errors.New("Choose one primary domain per service")
		}
	}
	for i := range out {
		if primary[out[i].Service] == 0 {
			out[i].Primary = true
			out[i].Redirect = false
			primary[out[i].Service] = 1
		}
	}
	return out, nil
}
func CanIncludeWWW(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if !domainPattern.MatchString(host) || strings.HasSuffix(host, ".localhost") || strings.HasPrefix(host, "www.") {
		return false
	}
	root, e := publicsuffix.EffectiveTLDPlusOne(host)
	return e == nil && root == host
}
func DetectMethod(root string) string {
	if info, e := os.Stat(filepath.Join(root, "Dockerfile")); e == nil && info.Mode().IsRegular() {
		return "dockerfile"
	}
	for _, name := range []string{"package.json", "pyproject.toml", "requirements.txt", "go.mod", "Cargo.toml", "Gemfile", "pom.xml", "build.gradle", "composer.json"} {
		if _, e := os.Stat(filepath.Join(root, name)); e == nil {
			return "railpack"
		}
	}
	if _, e := os.Stat(filepath.Join(root, "index.html")); e == nil {
		return "static"
	}
	return "railpack"
}
func Redact(c store.Config, text string) string {
	values := []string{}
	for _, svc := range c.Services {
		text = Redact(store.Config{Variables: svc.Variables}, text)
	}
	for _, v := range c.Variables {
		if v.Secret && v.Value != "" {
			values = append(values, v.Value)
			for _, line := range strings.Split(strings.ReplaceAll(v.Value, "\r\n", "\n"), "\n") {
				if line != "" && line != v.Value {
					values = append(values, line)
				}
			}
		}
	}
	sort.Slice(values, func(i, j int) bool { return len(values[i]) > len(values[j]) })
	for _, value := range values {
		text = strings.ReplaceAll(text, value, "[REDACTED]")
	}
	return text
}
func SourceRoot(repository string, c store.Config) (string, error) {
	root, e := filepath.Abs(repository)
	if e != nil {
		return "", e
	}
	candidate, e := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(c.RootDirectory)))
	if e != nil {
		return "", errors.New("Application directory does not exist")
	}
	rel, e := filepath.Rel(root, candidate)
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("Application directory escapes the repository")
	}
	return candidate, nil
}
