// Package deploy implements the durable, application-scoped static deployment pipeline.
package deploy

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"shipper/internal/store"
)

var domainPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)
var commitPattern = regexp.MustCompile(`^(?:[a-fA-F0-9]{40}|[a-fA-F0-9]{64})$`)
var branchPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]{0,127}$`)

func Validate(c *store.StaticConfig) error {
	if e := ValidateBuild(c); e != nil {
		return e
	}
	if c.Method == "compose" && c.ComposeSource == "paste" {
		c.Repository = ""
		c.Branch = ""
		c.RootDirectory = "."
		if c.Domain == "" {
			return nil
		}
		if !domainPattern.MatchString(c.Domain) || strings.HasSuffix(c.Domain, ".localhost") && c.HTTPS {
			return errors.New("Enter a valid domain and use HTTP for local addresses")
		}
		return nil
	}
	if c.Method == "image" || c.Method == "inline" {
		c.Domain = strings.ToLower(strings.TrimSpace(c.Domain))
		if strings.HasSuffix(c.Domain, ".localhost") && c.HTTPS {
			return errors.New("Use HTTP for local addresses")
		}
		if c.Method == "image" {
			if _, e := ImageReference(c.Image); e != nil {
				return e
			}
			if c.Platform != "" && c.Platform != "linux/amd64" && c.Platform != "linux/arm64" {
				return errors.New("Choose an available Linux platform")
			}
		}
		if c.Method == "inline" && (c.InlineDockerfile == "" || len(c.InlineDockerfile) > 128<<10) {
			return errors.New("Enter a Dockerfile of up to 128 KiB")
		}
		c.Repository = ""
		c.SourceID = ""
		c.RepositoryID = 0
		c.RootDirectory = "."
		c.Dockerfile = "Dockerfile"
		if c.Domain == "" && c.StartupPolicy == "process" {
			return nil
		}
		if !domainPattern.MatchString(c.Domain) {
			return errors.New("Enter a valid domain")
		}
		return nil
	}
	c.Repository = strings.TrimSpace(c.Repository)
	u, e := url.Parse(c.Repository)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") || len(c.Repository) > 2048 {
		return errors.New("Enter a public HTTPS Git repository URL")
	}
	if net.ParseIP(u.Hostname()) != nil || strings.ContainsAny(u.Host, "\\\r\n\t") || !domainPattern.MatchString(strings.ToLower(u.Hostname())) {
		return errors.New("Enter a public HTTPS Git repository URL")
	}
	if strings.EqualFold(u.Hostname(), "github.com") {
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) != 2 {
			return errors.New("Enter a GitHub repository URL without a branch or file path")
		}
		u.Path = "/" + parts[0] + "/" + strings.TrimSuffix(parts[1], ".git") + ".git"
		c.Repository = u.String()
	}
	c.Branch = strings.TrimSpace(c.Branch)
	if c.Branch != "" && (!branchPattern.MatchString(c.Branch) || strings.Contains(c.Branch, "..") || strings.Contains(c.Branch, "//") || strings.HasSuffix(c.Branch, "/") || strings.HasSuffix(c.Branch, ".lock")) {
		return errors.New("Enter a valid Git branch or full commit SHA")
	}
	c.PublishDirectory = strings.TrimSpace(c.PublishDirectory)
	if c.PublishDirectory == "" {
		c.PublishDirectory = "."
	}
	if strings.Contains(c.PublishDirectory, "\\") || path.IsAbs(c.PublishDirectory) || strings.Contains(c.PublishDirectory, "\x00") || len(c.PublishDirectory) > 256 {
		return errors.New("Publish directory must stay inside the repository")
	}
	c.PublishDirectory = path.Clean(c.PublishDirectory)
	if c.PublishDirectory == ".." || strings.HasPrefix(c.PublishDirectory, "../") || strings.Contains(c.PublishDirectory, ":") {
		return errors.New("Publish directory must stay inside the repository")
	}
	if c.Routing == "" {
		c.Routing = "files"
	}
	if c.Routing != "files" && c.Routing != "spa" && c.Routing != "strict" {
		return errors.New("Choose a valid routing mode")
	}
	c.Domain = strings.ToLower(strings.TrimSpace(c.Domain))
	if (c.Method == "compose" || c.StartupPolicy == "process") && c.Domain == "" {
		return nil
	}
	if len(c.Domain) > 253 || !domainPattern.MatchString(c.Domain) || net.ParseIP(c.Domain) != nil {
		return errors.New("Enter a domain without a protocol, port, or path")
	}
	if strings.HasSuffix(c.Domain, ".localhost") && c.HTTPS {
		return errors.New("Use HTTP for local .localhost addresses")
	}
	return nil
}
func publicAddress(a netip.Addr) bool {
	a = a.Unmap()
	if !a.IsGlobalUnicast() || a.IsPrivate() || a.IsLoopback() || a.IsLinkLocalUnicast() {
		return false
	}
	for _, raw := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32"} {
		if netip.MustParsePrefix(raw).Contains(a) {
			return false
		}
	}
	return true
}
func PublicAddress(a netip.Addr) bool { return publicAddress(a) }

type Source interface {
	Clone(context.Context, store.StaticConfig, string, func(string)) (string, error)
}
type GitSource struct{ Token string }

// HTTPS-only, pinned public DNS and disabled redirects/submodules prevent Git from
// becoming a path to local files, credentials, hooks, or private network services.
func (source GitSource) Clone(ctx context.Context, c store.StaticConfig, dir string, log func(string)) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	u, _ := url.Parse(c.Repository)
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", u.Hostname())
	if err != nil || len(addresses) == 0 {
		return "", errors.New("The Git host could not be resolved")
	}
	for _, a := range addresses {
		if !publicAddress(a) {
			return "", errors.New("Git repositories must resolve to public internet addresses")
		}
	}
	ip := addresses[0].String()
	if addresses[0].Is6() {
		ip = "[" + ip + "]"
	}
	options := []string{"-c", "protocol.allow=never", "-c", "protocol.https.allow=always", "-c", "credential.helper=", "-c", "http.followRedirects=false", "-c", "http.curloptResolve=" + u.Hostname() + ":443:" + ip, "-c", "core.hooksPath=" + filepath.Join(dir, "disabled-hooks"), "-c", "core.symlinks=false"}
	args := append(append([]string(nil), options...), "clone", "--depth=1", "--single-branch", "--no-tags", "--no-recurse-submodules")
	if c.Branch != "" && !commitPattern.MatchString(c.Branch) {
		args = append(args, "--branch", c.Branch)
	}
	args = append(args, "--", c.Repository, filepath.Join(dir, "repository"))
	commands := [][]string{args}
	if commitPattern.MatchString(c.Branch) {
		repository := filepath.Join(dir, "repository")
		init := append(append([]string(nil), options...), "init", "--quiet")
		if len(c.Branch) == 64 {
			init = append(init, "--object-format=sha256")
		}
		init = append(init, repository)
		fetch := append(append([]string(nil), options...), "-C", repository, "fetch", "--depth=1", "--no-tags", "--no-recurse-submodules", "--", c.Repository, c.Branch)
		checkout := append(append([]string(nil), options...), "-C", repository, "checkout", "--detach", "FETCH_HEAD")
		commands = [][]string{init, fetch, checkout}
	}
	safeLog := func(line string) {
		if source.Token != "" {
			line = strings.ReplaceAll(line, source.Token, "[REDACTED]")
			line = strings.ReplaceAll(line, base64.StdEncoding.EncodeToString([]byte("x-access-token:"+source.Token)), "[REDACTED]")
		}
		log(line)
	}
	output := &gitLogWriter{log: safeLog}
	var cmd *exec.Cmd
	guardDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-guardDone:
				return
			case <-ticker.C:
				var size int64
				filepath.WalkDir(dir, func(file string, entry fs.DirEntry, e error) error {
					if e != nil {
						return nil
					}
					if !entry.IsDir() {
						if info, e := entry.Info(); e == nil {
							size += info.Size()
						}
					}
					if size > 512<<20 {
						cancel()
						return errors.New("clone limit")
					}
					return nil
				})
			}
		}
	}()
	for _, arguments := range commands {
		cmd = exec.CommandContext(ctx, "git", arguments...)
		cmd.Env = source.environment()
		cmd.WaitDelay = 2 * time.Second
		cmd.Stdout = output
		cmd.Stderr = output
		if err = cmd.Run(); err != nil {
			break
		}
	}
	close(guardDone)
	output.flush()
	if err != nil {
		return "", fmt.Errorf("Git clone failed: %w", err)
	}
	cmd = exec.CommandContext(ctx, "git", "-C", filepath.Join(dir, "repository"), "rev-parse", "HEAD")
	cmd.Env = source.environment()
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

type gitLogWriter struct {
	mu      sync.Mutex
	pending string
	discard bool
	lines   int
	Limit   int
	log     func(string)
}

func (w *gitLogWriter) limit() int {
	if w.Limit > 0 {
		return w.Limit
	}
	return 512
}
func (w *gitLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := len(p)
	w.pending += strings.ReplaceAll(string(p), "\r", "\n")
	for {
		line, rest, ok := strings.Cut(w.pending, "\n")
		if !ok {
			if len(w.pending) > 65536 {
				w.pending = ""
				w.discard = true
			}
			break
		}
		w.pending = rest
		if w.discard || len(line) > 65536 {
			line = "[Log line exceeded 64 KiB]"
			w.discard = false
		}
		if line != "" && w.lines < w.limit() {
			w.log(line)
			w.lines++
		}
	}
	return n, nil
}
func (w *gitLogWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.pending != "" && w.lines < w.limit() {
		if w.discard {
			w.log("[Log line exceeded 64 KiB]")
		} else {
			w.log(w.pending)
		}
	}
	w.pending = ""
}
func gitEnvironment() []string {
	out := []string{}
	for _, v := range os.Environ() {
		key, _, _ := strings.Cut(v, "=")
		key = strings.ToUpper(key)
		if strings.HasPrefix(key, "GIT_") || strings.Contains(key, "PROXY") {
			continue
		}
		out = append(out, v)
	}
	return append(out, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_LFS_SKIP_SMUDGE=1")
}

func ApacheConfig(c store.StaticConfig) string {
	overrides := "AllowOverride None"
	if c.HTAccess {
		overrides += "\nAllowOverrideList RewriteEngine RewriteOptions RewriteBase RewriteCond RewriteRule Redirect RedirectMatch ErrorDocument Header <IfModule"
	}
	routing := ""
	if c.Routing == "spa" {
		routing = "FallbackResource /index.html"
	}
	if c.Routing == "files" {
		routing = `RewriteEngine On
RewriteCond %{REQUEST_FILENAME} !-d
RewriteCond %{REQUEST_FILENAME} !-f
RewriteCond %{DOCUMENT_ROOT}%{REQUEST_URI}.html -f
RewriteRule ^(.+)$ $1.html [L]`
	}
	return fmt.Sprintf(`ServerRoot "/usr/local/apache2"
Listen 8080
ServerName localhost
LoadModule mpm_event_module modules/mod_mpm_event.so
LoadModule unixd_module modules/mod_unixd.so
LoadModule authz_core_module modules/mod_authz_core.so
LoadModule dir_module modules/mod_dir.so
LoadModule mime_module modules/mod_mime.so
LoadModule log_config_module modules/mod_log_config.so
LoadModule rewrite_module modules/mod_rewrite.so
LoadModule alias_module modules/mod_alias.so
LoadModule headers_module modules/mod_headers.so
User #65532
Group #65532
PidFile /tmp/httpd.pid
ServerTokens Prod
ServerSignature Off
TraceEnable Off
ErrorLog /proc/self/fd/2
LogFormat "%%h %%l %%u %%t \"%%r\" %%>s %%b" common
CustomLog /proc/self/fd/1 common
TypesConfig conf/mime.types
DocumentRoot "/usr/local/apache2/htdocs"
<Directory />
    AllowOverride None
    Require all denied
</Directory>
<Directory "/usr/local/apache2/htdocs">
    Options -Indexes -FollowSymLinks +SymLinksIfOwnerMatch
    %s
    Require all granted
    DirectoryIndex index.html
    %s
</Directory>
<FilesMatch "^\.">
    Require all denied
</FilesMatch>
Header always set X-Content-Type-Options "nosniff"
`, overrides, routing)
}

// StaticArchive materializes only regular publish files with normalized modes.
// It excludes repository metadata/secrets and rejects links, devices, and escapes.
func StaticArchive(repository string, c store.StaticConfig, deploymentID string) ([]byte, error) {
	root, err := filepath.Abs(repository)
	if err != nil {
		return nil, err
	}
	publish := filepath.Join(root, filepath.FromSlash(c.PublishDirectory))
	real, err := filepath.EvalSymlinks(publish)
	if err != nil {
		return nil, errors.New("Publish directory does not exist")
	}
	rel, err := filepath.Rel(root, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, errors.New("Publish directory escapes the repository")
	}
	if info, e := os.Stat(filepath.Join(real, "index.html")); e != nil || !info.Mode().IsRegular() {
		return nil, errors.New("The publish directory must contain index.html")
	}
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	var size int64
	count := 0
	err = filepath.WalkDir(real, func(file string, entry fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if file == real {
			return nil
		}
		name, _ := filepath.Rel(real, file)
		name = filepath.ToSlash(name)
		base := strings.ToLower(entry.Name())
		blocked := strings.HasPrefix(base, ".") && base != ".well-known" && !(base == ".htaccess" && c.HTAccess)
		if blocked || base == "node_modules" || base == "dockerfile" || base == "compose.yaml" || base == "compose.yml" || base == "id_rsa" || strings.HasSuffix(base, ".pem") || strings.HasSuffix(base, ".key") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("Publish files cannot be symbolic links: %s", name)
		}
		info, e := entry.Info()
		if e != nil {
			return e
		}
		mode := int64(0644)
		kind := byte(tar.TypeReg)
		if entry.IsDir() {
			mode = 0755
			kind = tar.TypeDir
		} else if !info.Mode().IsRegular() {
			return fmt.Errorf("Unsupported publish file: %s", name)
		}
		count++
		size += info.Size()
		if size > 100<<20 || count > 10000 {
			return errors.New("Static sites are limited to 100 MiB and 10,000 files")
		}
		if e = tw.WriteHeader(&tar.Header{Name: "usr/local/apache2/htdocs/" + name, Mode: mode, Size: func() int64 {
			if entry.IsDir() {
				return 0
			}
			return info.Size()
		}(), Typeflag: kind}); e != nil {
			return e
		}
		if !entry.IsDir() {
			data, e := os.ReadFile(file)
			if e != nil {
				return e
			}
			_, e = tw.Write(data)
			return e
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	conf := ApacheConfig(c) + "\nHeader always set X-Shipper-Deployment \"" + deploymentID + "\"\n"
	if err = tw.WriteHeader(&tar.Header{Name: "usr/local/apache2/conf/httpd.conf", Mode: 0644, Size: int64(len(conf))}); err != nil {
		return nil, err
	}
	tw.Write([]byte(conf))
	err = tw.Close()
	return buf.Bytes(), err
}

func (s GitSource) environment() []string {
	env := gitEnvironment()
	if s.Token != "" {
		env = append(env, "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=http.https://github.com/.extraheader", "GIT_CONFIG_VALUE_0=Authorization: Basic "+base64.StdEncoding.EncodeToString([]byte("x-access-token:"+s.Token)))
	}
	return env
}
