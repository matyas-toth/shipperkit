// Package cli is a thin authenticated client of Shipper's shared operations API.
package cli

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"shipper/internal/auth"
	"shipper/internal/deploy"
	"shipper/internal/store"
)

type Config struct {
	URL      string            `json:"url"`
	Token    string            `json:"token"`
	Projects map[string]string `json:"projects,omitempty"`
}
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s: %s (HTTP %d)", e.Code, e.Message, e.Status)
}

type Client struct {
	storedToken string
	Config      Config
	HTTP        *http.Client
	Out         io.Writer
	JSON        bool
	Dir         string
}

func directory() (string, error) {
	if v := os.Getenv("SHIPPER_CLIENT_DIR"); v != "" {
		return v, nil
	}
	v, e := os.UserConfigDir()
	return filepath.Join(v, "shipper"), e
}
func save(file string, v any) error {
	if e := os.MkdirAll(filepath.Dir(file), 0700); e != nil {
		return e
	}
	data, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.OpenFile(file, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	if e = f.Chmod(0600); e != nil {
		return e
	}
	_, e = f.Write(data)
	return e
}
func (c *Client) request(ctx context.Context, method, path string, body any, kind string, key string, out any) error {
	var reader io.Reader
	if r, ok := body.(io.Reader); ok {
		reader = r
	} else if body != nil {
		raw, e := json.Marshal(body)
		if e != nil {
			return e
		}
		reader = bytes.NewReader(raw)
	}
	req, e := http.NewRequestWithContext(ctx, method, c.Config.URL+"/api/v1"+path, reader)
	if e != nil {
		return e
	}
	req.Header.Set("Authorization", "Bearer "+c.Config.Token)
	if kind == "" {
		kind = "application/json"
	}
	req.Header.Set("Content-Type", kind)
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	transport := *c.HTTP
	if kind == "application/x-tar" {
		transport.Timeout = 10 * time.Minute
	}
	res, e := transport.Do(req)
	if e != nil {
		return errors.New("Shipper could not be reached; reconnect using the operation ID before retrying")
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		var result struct {
			Error struct{ Code, Message string }
		}
		json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&result)
		return &APIError{Code: result.Error.Code, Message: result.Error.Message, Status: res.StatusCode}
	}
	if out != nil {
		return json.NewDecoder(io.LimitReader(res.Body, 16<<20)).Decode(out)
	}
	return nil
}
func (c *Client) output(v any) error {
	if c.JSON {
		return json.NewEncoder(c.Out).Encode(v)
	}
	data, e := json.MarshalIndent(v, "", "  ")
	if e == nil {
		_, e = fmt.Fprintln(c.Out, string(data))
	}
	return e
}
func (c *Client) resolve(ctx context.Context, value string) (store.Application, error) {
	var result struct{ Items []store.Application }
	if e := c.request(ctx, "GET", "/applications", nil, "", "", &result); e != nil {
		return store.Application{}, e
	}
	matches := []store.Application{}
	for _, a := range result.Items {
		if a.ID == value || a.Name == value {
			matches = append(matches, a)
		}
	}
	if len(matches) != 1 {
		return store.Application{}, errors.New("Choose one unambiguous application name or ID")
	}
	return matches[0], nil
}

// Parse flags on either side of the positional project/name.
func parse(fs *flag.FlagSet, args []string) error {
	options, positionals := []string{}, []string{}
	for i := 0; i < len(args); i++ {
		v := args[i]
		if strings.HasPrefix(v, "--") {
			options = append(options, v)
			name := strings.Split(strings.TrimPrefix(v, "--"), "=")[0]
			f := fs.Lookup(name)
			if f == nil {
				return fmt.Errorf("Unknown option --%s", name)
			}
			isBool := false
			if b, ok := f.Value.(interface{ IsBoolFlag() bool }); ok {
				isBool = b.IsBoolFlag()
			}
			if !strings.Contains(v, "=") && !isBool {
				if i+1 >= len(args) {
					return fmt.Errorf("Missing value for --%s", name)
				}
				i++
				options = append(options, args[i])
			}
		} else {
			positionals = append(positionals, v)
		}
	}
	return fs.Parse(append(options, positionals...))
}
func Run(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("Use connect, apps, inspect, doctor, plan, deploy, watch, logs, rollback or task")
	}
	dir, e := directory()
	if e != nil {
		return e
	}
	c := &Client{HTTP: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("Management redirects are disabled") }}, Out: out, Dir: dir}
	clean := []string{}
	for _, v := range args {
		if v == "--json" {
			c.JSON = true
		} else {
			clean = append(clean, v)
		}
	}
	args = clean
	if len(args) == 0 {
		return errors.New("Choose a command")
	}
	for _, arg := range args[1:] {
		if arg == "--help" || arg == "-h" {
			return c.help(args[0])
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	configFile := filepath.Join(dir, "client.json")
	if args[0] == "connect" {
		fs := flag.NewFlagSet("connect", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		stdin := fs.Bool("token-stdin", false, "Read a scoped access token from stdin")
		if e = parse(fs, args[1:]); e != nil {
			return e
		}
		if fs.NArg() != 1 {
			return errors.New("Usage: shipper connect https://shipper.example.com --token-stdin")
		}
		u, e := url.Parse(strings.TrimSuffix(fs.Arg(0), "/"))
		if e != nil || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")) {
			return errors.New("Use an HTTPS Shipper origin; HTTP is allowed only on loopback")
		}
		token := os.Getenv("SHIPPER_TOKEN")
		if *stdin {
			data, e := io.ReadAll(io.LimitReader(os.Stdin, 1024))
			if e != nil {
				return e
			}
			token = strings.TrimSpace(string(data))
		}
		if len(token) != 64 {
			return errors.New("Provide an operations token through --token-stdin or SHIPPER_TOKEN")
		}
		c.Config = Config{URL: u.String(), Token: token}
		var caps map[string]any
		if e = c.request(ctx, "GET", "/capabilities", nil, "", "", &caps); e != nil {
			return e
		}
		if e = save(configFile, c.Config); e != nil {
			return e
		}
		return c.output(map[string]any{"connected": true, "url": c.Config.URL, "instanceId": caps["instanceId"]})
	}
	data, e := os.ReadFile(configFile)
	if e != nil {
		return errors.New("Connect to Shipper first")
	}
	if e = json.Unmarshal(data, &c.Config); e != nil {
		return e
	}
	c.storedToken = c.Config.Token
	if raw := os.Getenv("SHIPPER_TOKEN"); raw != "" {
		c.Config.Token = raw
	}
	switch args[0] {
	case "apps":
		var v any
		e = c.request(ctx, "GET", "/applications", nil, "", "", &v)
		if e == nil {
			e = c.output(v)
		}
		return e
	case "inspect", "doctor", "logs", "watch", "rollback", "task":
		return c.applicationCommand(ctx, args)
	case "plan", "deploy":
		return c.project(ctx, args)
	default:
		return errors.New("Unknown Shipper command")
	}
}
func (c *Client) applicationCommand(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	operation := fs.String("operation", "", "Operation ID")
	service := fs.String("service", "", "Service name")
	after := fs.Int64("after", 0, "Log cursor")
	wait := fs.Duration("timeout", 10*time.Minute, "Watch timeout")
	confirm := fs.Bool("apply", false, "Apply a reviewed rollback")
	revision := fs.String("revision", "", "Reviewed revision")
	if e := parse(fs, args[1:]); e != nil {
		return e
	}
	if fs.NArg() < 1 {
		return errors.New("Provide an application name or ID")
	}
	a, e := c.resolve(ctx, fs.Arg(0))
	if e != nil {
		return e
	}
	base := "/applications/" + a.ID
	switch args[0] {
	case "inspect", "doctor":
		path := base
		if args[0] == "doctor" {
			path += "/diagnostics"
		}
		var v any
		e = c.request(ctx, "GET", path, nil, "", "", &v)
		if e == nil {
			e = c.output(v)
		}
		return e
	case "watch":
		if *operation == "" {
			return errors.New("Provide --operation ID")
		}
		return c.watch(ctx, a.ID, *operation, *wait)
	case "logs":
		if *operation != "" {
			var v any
			e = c.request(ctx, "GET", base+"/deployments/"+url.PathEscape(*operation)+"/logs?after="+fmt.Sprint(*after), nil, "", "", &v)
			if e == nil {
				e = c.output(v)
			}
			return e
		}
		id := ""
		for _, container := range a.Containers {
			if *service == "" || container.Service == *service {
				if id != "" {
					return errors.New("Choose --service for a multi-container application")
				}
				id = container.ID
			}
		}
		if id == "" {
			return errors.New("No matching runtime container")
		}
		req, _ := http.NewRequestWithContext(ctx, "GET", c.Config.URL+"/api/v1"+base+"/logs?container="+url.QueryEscape(id)+"&tail=200", nil)
		req.Header.Set("Authorization", "Bearer "+c.Config.Token)
		res, e := c.HTTP.Do(req)
		if e != nil {
			return e
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			return fmt.Errorf("Logs returned HTTP %d", res.StatusCode)
		}
		_, e = io.Copy(c.Out, io.LimitReader(res.Body, 8<<20))
		return e
	case "rollback":
		if *operation == "" {
			return errors.New("Provide --operation ID")
		}
		body := map[string]any{"review": true}
		if *confirm {
			if *revision == "" {
				return errors.New("Provide the reviewed --revision")
			}
			body = map[string]any{"revision": *revision}
		}
		var v any
		e = c.request(ctx, "POST", base+"/deployments/"+url.PathEscape(*operation)+"/rollback", body, "", auth.Token(), &v)
		if e == nil {
			e = c.output(v)
		}
		return e
	case "task":
		if fs.NArg() != 2 {
			return errors.New("Provide the saved task ID after the application")
		}
		var v any
		e = c.request(ctx, "POST", base+"/tasks/"+url.PathEscape(fs.Arg(1))+"/run", map[string]any{}, "", auth.Token(), &v)
		if e == nil {
			e = c.output(v)
		}
		return e
	}
	return nil
}
func (c *Client) watch(ctx context.Context, app, id string, timeout time.Duration) error {
	if timeout <= 0 || timeout > 30*time.Minute {
		return errors.New("Watch timeout must be between 0 and 30 minutes")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	state := ""
	for {
		var d store.Deployment
		if e := c.request(ctx, "GET", "/applications/"+app+"/deployments/"+url.PathEscape(id), nil, "", "", &d); e != nil {
			return e
		}
		if d.State != state {
			if e := c.output(map[string]any{"id": d.ID, "applicationId": app, "state": d.State}); e != nil {
				return e
			}
			state = d.State
		}
		if d.State == "succeeded" {
			return nil
		}
		if d.State == "failed" || d.State == "cancelled" || d.State == "interrupted" {
			return fmt.Errorf("Operation %s ended as %s; inspect its logs before retrying", id, d.State)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("Watch ended; operation %s may still be running", id)
		case <-tick.C:
		}
	}
}

type Plan struct {
	ApplicationID string       `json:"applicationId"`
	Revision      string       `json:"revision"`
	Config        store.Config `json:"config"`
	Operation     string       `json:"operation"`
	Changes       []string     `json:"changes"`
	New           bool         `json:"new"`
	Name          string       `json:"name"`
	URL           string       `json:"url"`
	Project       string       `json:"project"`
}

func (c *Client) project(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	app := fs.String("app", "", "Application name or ID")
	forceNew := fs.Bool("new", false, "Create another application instead of using its project association")
	name := fs.String("name", "", "New application name")
	method := fs.String("method", "auto", "Build method")
	domain := fs.String("domain", "", "Hostname")
	port := fs.Int("port", 0, "HTTP port")
	worker := fs.Bool("worker", false, "Process readiness")
	file := fs.String("config", "", "Optional JSON application configuration")
	planID := fs.String("plan", "", "Apply a saved plan ID")
	wait := fs.Bool("wait", false, "Watch until finished")
	if e := parse(fs, args[1:]); e != nil {
		return e
	}
	chosen := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { chosen[f.Name] = true })
	var p Plan
	if *planID != "" {
		if !safePlanID(*planID) {
			return errors.New("Invalid plan ID")
		}
		raw, e := os.ReadFile(filepath.Join(c.Dir, "plans", *planID+".json"))
		if e != nil {
			return e
		}
		if e = json.Unmarshal(raw, &p); e != nil {
			return e
		}
		if p.URL != c.Config.URL {
			return errors.New("The plan belongs to another Shipper instance")
		}
	} else {
		root := "."
		if fs.NArg() > 0 {
			root = fs.Arg(0)
		}
		root, e := filepath.Abs(root)
		if e != nil {
			return e
		}
		p.Project = root
		p.URL = c.Config.URL
		if *forceNew && *app != "" {
			return errors.New("Choose --new or --app")
		}
		if *app == "" && !*forceNew {
			*app = c.Config.Projects[root]
		}
		p.New = *app == ""
		p.Config = store.Config{Method: *method, Port: *port, HealthPath: "/", RootDirectory: ".", PublishDirectory: ".", Routing: "files", Dockerfile: "Dockerfile", ComposeSource: "git", ComposeFile: "compose.yaml"}
		p.Name = *name
		if p.Name == "" {
			p.Name = filepath.Base(root)
		}
		if !p.New {
			a, e := c.resolve(ctx, *app)
			if e != nil {
				return e
			}
			p.ApplicationID = a.ID
			p.Config = a.Config
			p.Name = a.Name
			if *domain != "" && *domain != a.Config.Domain {
				return errors.New("Edit an existing application's domains through Domains or the domains API")
			}
		}
		if *file != "" {
			raw, e := os.ReadFile(*file)
			if e != nil {
				return e
			}
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.DisallowUnknownFields()
			if e = decoder.Decode(&p.Config); e != nil {
				return e
			}
			if e = decoder.Decode(&struct{}{}); e != io.EOF {
				return errors.New("Configuration must contain one JSON object")
			}
		}
		if chosen["method"] {
			p.Config.Method = *method
		}
		if *domain != "" {
			p.Config.Domain = *domain
			p.Config.HTTPS = !strings.HasSuffix(*domain, ".localhost")
		}
		if chosen["port"] {
			p.Config.Port = *port
		}
		if chosen["worker"] {
			p.Config.StartupPolicy = "http"
			if *worker {
				p.Config.StartupPolicy = "process"
			}
		}
		if p.Config.Method == "auto" {
			appRoot, e := deploy.SourceRoot(root, p.Config)
			if e != nil {
				return e
			}
			p.Config.Method = deploy.DetectMethod(appRoot)
			for _, name := range []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"} {
				if info, e := os.Stat(filepath.Join(appRoot, name)); e == nil && info.Mode().IsRegular() {
					p.Config.Method = "compose"
					p.Config.ComposeFile = name
					break
				}
			}
		}
		archive, files, size, e := Archive(root)
		if e != nil {
			return e
		}
		defer os.Remove(archive)
		body, e := os.Open(archive)
		if e != nil {
			return e
		}
		defer body.Close()
		provenance := filepath.Base(root)
		git := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "HEAD")
		if raw, e := git.Output(); e == nil {
			provenance = strings.TrimSpace(string(raw))
			status, _ := exec.CommandContext(ctx, "git", "--no-optional-locks", "-c", "core.fsmonitor=false", "-C", root, "status", "--porcelain", "--ignore-submodules=all").Output()
			if len(status) > 0 {
				provenance += " dirty"
			}
		}
		var source store.SourceSnapshot
		e = c.request(ctx, "POST", "/sources?application="+url.QueryEscape(p.ApplicationID)+"&provenance="+url.QueryEscape(provenance), body, "application/x-tar", "", &source)
		if e != nil {
			return e
		}
		p.Config.LocalSource = source.ID
		if p.Config.Method == "compose" {
			p.Config.ComposeSource = "git"
			p.Config.ComposeYAML = ""
			p.Config.ComposeKeep = false
			p.Config.ComposeHasValue = false
		}
		p.Config.Repository = ""
		p.Config.SourceID = ""
		p.Config.RepositoryID = 0
		p.Config.AutoDeploy = store.AutoDeploy{}
		p.Config.Branch = ""
		if p.New {
			var result Plan
			e = c.request(ctx, "POST", "/plan", map[string]any{"name": p.Name, "config": p.Config}, "", "", &result)
			if e != nil {
				return e
			}
			p.ApplicationID = result.ApplicationID
			p.Revision = result.Revision
			p.Operation = result.Operation
			p.Changes = result.Changes
		} else {
			var result Plan
			e = c.request(ctx, "POST", "/applications/"+p.ApplicationID+"/changes", map[string]any{"config": p.Config}, "", "", &result)
			if e != nil {
				return e
			}
			p.Revision = result.Revision
			p.Operation = result.Operation
			p.Changes = result.Changes
		}
		id := auth.Token()[:24]
		if e = save(filepath.Join(c.Dir, "plans", id+".json"), p); e != nil {
			return e
		}
		if args[0] == "plan" {
			return c.output(map[string]any{"planId": id, "applicationId": p.ApplicationID, "revision": p.Revision, "operation": p.Operation, "changes": p.Changes, "source": source, "files": files, "bytes": size})
		}
		*planID = id
	}
	if args[0] == "plan" {
		return errors.New("Use deploy --plan ID to apply a saved plan")
	}
	path := "/applications/" + p.ApplicationID + "/deployments"
	body := map[string]any{"config": p.Config, "revision": p.Revision}
	if p.New {
		path = "/applications"
		body["name"] = p.Name
		body["id"] = p.ApplicationID
	}
	var d store.Deployment
	if e := c.request(ctx, "POST", path, body, "", "cli-plan:"+*planID, &d); e != nil {
		return e
	}
	if c.Config.Projects == nil {
		c.Config.Projects = map[string]string{}
	}
	c.Config.Projects[p.Project] = d.ApplicationID
	persisted := c.Config
	persisted.Token = c.storedToken
	if e := save(filepath.Join(c.Dir, "client.json"), persisted); e != nil {
		return e
	}
	if e := c.output(map[string]any{"applicationId": d.ApplicationID, "operationId": d.ID, "state": d.State}); e != nil {
		return e
	}
	if *wait {
		return c.watch(ctx, d.ApplicationID, d.ID, 20*time.Minute)
	}
	return nil
}
func safePlanID(id string) bool {
	if len(id) != 24 {
		return false
	}
	for _, v := range id {
		if !strings.ContainsRune("abcdef0123456789", v) {
			return false
		}
	}
	return true
}
func Archive(root string) (file string, files int, size int64, err error) {
	f, e := os.CreateTemp("", "shipper-source-*.tar")
	if e != nil {
		err = e
		return
	}
	file = f.Name()
	defer func() {
		f.Close()
		if err != nil {
			os.Remove(file)
		}
	}()
	w := tar.NewWriter(f)
	defer w.Close()
	// .shipperignore adds simple filepath globs; .dockerignore keeps Docker semantics
	// at build time. Secret/dependency exclusions always apply before upload.
	patterns := []string{}
	if raw, e := os.ReadFile(filepath.Join(root, ".shipperignore")); e == nil {
		for _, v := range strings.Split(string(raw), "\n") {
			v = strings.TrimSpace(v)
			if v != "" && !strings.HasPrefix(v, "#") {
				patterns = append(patterns, v)
			}
		}
	}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(root, path)
		if e != nil {
			return e
		}
		if rel == "." {
			return nil
		}
		excluded := store.SourceExcluded(rel)
		for _, pattern := range patterns {
			matched, _ := filepath.Match(pattern, filepath.ToSlash(rel))
			if matched {
				excluded = true
			}
		}
		if excluded {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("Source symlinks are unsupported: %s", rel)
		}
		if entry.IsDir() {
			return nil
		}
		info, e := entry.Info()
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() {
			return errors.New("Only regular source files are supported")
		}
		files++
		size += info.Size()
		if files > 10000 || size > 64<<20 {
			return errors.New("Source is limited to 64 MiB and 10000 files; add .shipperignore exclusions")
		}
		h := &tar.Header{Name: filepath.ToSlash(rel), Mode: int64(info.Mode().Perm()), Size: info.Size(), Typeflag: tar.TypeReg}
		if e = w.WriteHeader(h); e != nil {
			return e
		}
		input, e := os.Open(path)
		if e != nil {
			return e
		}
		_, e = io.CopyN(w, input, info.Size())
		input.Close()
		return e
	})
	if err == nil {
		err = w.Close()
	}
	if err == nil {
		err = f.Close()
	}
	return
}
