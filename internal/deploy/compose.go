package deploy

// Compose's reference loader owns interpolation, dependency normalization and
// schema validation. The official CLI owns builds and orchestration. Shipper
// adds ownership, portable repository files and its existing proxy boundary.
import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/compose-spec/compose-go/v2/dotenv"
	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/template"
	ct "github.com/compose-spec/compose-go/v2/types"
	"github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
	"go.yaml.in/yaml/v4"
	"shipper/internal/store"
)

var serviceName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,62}$`)

func (r *DockerRuntime) composeProject(app string) string { return r.prefix() + "-" + app }
func composeServiceKey(name string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(name)))[:12]
}
func composeAlias(app, service string) string { return "shp-" + app + "-" + composeServiceKey(service) }

// checkedFile runs before any loader operation that can open referenced files.
func checkedFile(root, file string) (string, error) {
	if !filepath.IsAbs(file) {
		file = filepath.Join(root, filepath.FromSlash(file))
	}
	abs, e := filepath.Abs(file)
	if e != nil {
		return "", e
	}
	rel, e := filepath.Rel(root, abs)
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", errors.New("Compose files and build contexts must stay inside the repository")
	}
	current := root
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, e := os.Lstat(current)
		if e != nil {
			return "", fmt.Errorf("Referenced Compose path does not exist: %s", rel)
		}
		if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
			return "", errors.New("Compose paths must be regular files or directories without symbolic links")
		}
	}
	return abs, nil
}

func composeContent(c store.Config, root string) ([]byte, string, error) {
	file := c.ComposeFile
	if file == "" {
		file = "compose.yaml"
	}
	if c.ComposeSource == "paste" {
		return []byte(c.ComposeYAML), filepath.Join(root, "compose.yaml"), nil
	}
	p, e := checkedFile(root, file)
	if e != nil {
		return nil, "", e
	}
	info, e := os.Stat(p)
	if e != nil || info.IsDir() || info.Size() > 128<<10 {
		return nil, "", errors.New("Compose files must be regular files smaller than 128 KiB")
	}
	data, e := os.ReadFile(p)
	return data, p, e
}

func LoadCompose(ctx context.Context, c store.Config, root, project string, boundaries ...string) (*ct.Project, error) {
	root, e := filepath.Abs(root)
	if e != nil {
		return nil, e
	}
	boundary := root
	if len(boundaries) > 0 {
		boundary, e = filepath.Abs(boundaries[0])
		if e != nil {
			return nil, e
		}
	}
	if _, e = checkedFile(boundary, root); e != nil {
		return nil, e
	}
	data, file, e := composeContent(c, root)
	if e != nil {
		return nil, e
	}
	if len(data) == 0 || len(data) > 128<<10 {
		return nil, errors.New("Enter Compose YAML of up to 128 KiB")
	}
	var raw map[string]any
	if e = yaml.Unmarshal(data, &raw); e != nil {
		return nil, errors.New("The Compose YAML could not be parsed")
	}
	if raw["include"] != nil {
		return nil, errors.New("Compose include is not supported yet; use one Compose file")
	}
	// Check the expanded YAML map, including anchor merges, before the loader
	// follows extends or resolves paths. Never allow remote resource loaders.
	if services, ok := raw["services"].(map[string]any); ok {
		for _, value := range services {
			if svc, ok := value.(map[string]any); ok && svc["extends"] != nil {
				return nil, errors.New("Compose extends is not supported yet; use YAML anchors in one file")
			}
		}
	}
	env := ct.Mapping{}
	if c.ComposeSource != "paste" {
		if _, err := os.Stat(filepath.Join(root, ".env")); err == nil {
			p, err := checkedFile(boundary, filepath.Join(root, ".env"))
			if err != nil {
				return nil, err
			}
			info, err := os.Stat(p)
			if err != nil || info.Size() > 128<<10 {
				return nil, errors.New("Repository .env is too large")
			}
			f, err := os.Open(p)
			if err != nil {
				return nil, err
			}
			values, err := dotenv.ParseWithLookup(f, func(name string) (string, bool) {
				for _, v := range c.Variables {
					if v.Name == name {
						return v.Value, true
					}
				}
				return "", false
			})
			f.Close()
			if err != nil {
				return nil, errors.New("Repository .env could not be parsed")
			}
			for k, v := range values {
				env[k] = v
			}
		}
	}
	for _, v := range c.Variables {
		env[v.Name] = v.Value
	}
	p, e := loader.LoadWithContext(ctx, ct.ConfigDetails{WorkingDir: root, ConfigFiles: []ct.ConfigFile{{Filename: file, Content: data}}, Environment: env}, func(o *loader.Options) {
		o.SetProjectName(project, true)
		o.SkipInclude = true
		o.SkipExtends = true
		o.SkipResolveEnvironment = true
		o.SkipResolveLabels = true
	})
	if e != nil {
		var required *template.MissingRequiredError
		if errors.As(e, &required) && variableName.MatchString(required.Variable) {
			return nil, fmt.Errorf("Set %s in .env before deploying", required.Variable)
		}
		return nil, errors.New("Invalid Compose configuration; check service definitions, field types and required variables")
	}
	if len(p.Services) == 0 || len(p.Services) > 32 {
		return nil, errors.New("Compose applications support between 1 and 32 active services")
	}
	if len(p.Jobs) != 0 || len(p.Models) != 0 {
		return nil, errors.New("Compose jobs and models are not supported yet")
	}
	if _, ok := p.Networks["shipper_edge"]; ok {
		return nil, errors.New("Network name shipper_edge is reserved")
	}
	if _, ok := p.Volumes["shipper_files"]; ok {
		return nil, errors.New("Volume name shipper_files is reserved")
	}
	for name, svc := range p.Services {
		if !serviceName.MatchString(name) {
			return nil, errors.New("Service names must be at most 63 letters, numbers, dots, underscores or hyphens")
		}
		if len(svc.LabelFiles) > 0 {
			return nil, fmt.Errorf("Service %s: put labels directly in the Compose file", name)
		}
		for _, f := range svc.EnvFiles {
			if !bool(f.Required) {
				if _, err := os.Lstat(f.Path); os.IsNotExist(err) {
					continue
				}
			}
			if _, e = checkedFile(boundary, f.Path); e != nil {
				return nil, e
			}
			info, _ := os.Stat(f.Path)
			if info.Size() > 128<<10 {
				return nil, errors.New("Compose env_file is too large")
			}
		}
		if svc.Build != nil {
			if _, e = checkedFile(boundary, svc.Build.Context); e != nil {
				return nil, e
			}
			if svc.Build.DockerfileInline == "" {
				if _, e = checkedFile(boundary, filepath.Join(svc.Build.Context, func() string {
					if svc.Build.Dockerfile == "" {
						return "Dockerfile"
					}
					return svc.Build.Dockerfile
				}())); e != nil {
					return nil, e
				}
			}
			if len(svc.Build.AdditionalContexts) > 0 || len(svc.Build.SSH) > 0 || len(svc.Build.Secrets) > 0 || svc.Build.Privileged || svc.Build.Network == "host" || len(svc.Build.Entitlements) > 0 {
				return nil, fmt.Errorf("Service %s: additional contexts, SSH, build secrets and privileged builds are not supported yet", name)
			}
		}
		for _, vol := range svc.Volumes {
			if vol.Type == "bind" {
				if c.ComposeSource == "paste" {
					return nil, errors.New("Pasted Compose cannot use local files; use a Git repository or named volumes")
				}
				if _, e = checkedFile(boundary, vol.Source); e != nil {
					return nil, e
				}
			}
		}
	}
	for _, f := range p.Configs {
		if f.File != "" {
			if c.ComposeSource == "paste" {
				return nil, errors.New("Pasted Compose configs must use content")
			}
			if _, e = checkedFile(boundary, f.File); e != nil {
				return nil, e
			}
		}
	}
	for _, f := range p.Secrets {
		if f.File != "" {
			if c.ComposeSource == "paste" {
				return nil, errors.New("Pasted Compose secrets must use environment")
			}
			if _, e = checkedFile(boundary, f.File); e != nil {
				return nil, e
			}
		}
	}
	p, e = p.WithServicesEnvironmentResolved(true)
	if e != nil {
		return nil, errors.New("Compose env_file could not be loaded")
	}
	return p, nil
}

func ComposeServices(p *ct.Project, c store.Config) []store.Service {
	out := []store.Service{}
	for _, name := range p.ServiceNames() {
		svc := p.Services[name]
		port := 0
		if len(svc.Ports) == 1 && svc.Ports[0].Protocol != "udp" {
			port = int(svc.Ports[0].Target)
		} else if len(svc.Expose) == 1 {
			port, _ = strconv.Atoi(strings.TrimSuffix(string(svc.Expose[0]), "/tcp"))
		}
		// Infer common HTTP ports, never a database or queue's native protocol.
		switch port {
		case 80, 443, 3000, 3001, 4000, 5000, 8000, 8001, 8080, 8081, 8888, 9000:
		default:
			port = 0
		}
		health := "/"
		for _, override := range c.Services {
			if override.Name == name {
				if override.Port != nil {
					port = *override.Port
				}
				if override.HealthPath != "" {
					health = override.HealthPath
				}
			}
		}
		oneShot := false
		for _, other := range p.Services {
			if dep, ok := other.DependsOn[name]; ok && dep.Condition == ct.ServiceConditionCompletedSuccessfully {
				oneShot = true
			}
		}
		environment := map[string]store.Variable{}
		for name, value := range svc.Environment {
			if value != nil {
				environment[name] = store.Variable{Name: name, Value: *value, Secret: true, Scope: "runtime"}
			}
		}
		for _, override := range c.Services {
			if override.Name == name {
				for _, key := range override.UnsetVariables {
					delete(environment, key)
				}
				for _, v := range override.Variables {
					environment[v.Name] = store.Variable{Name: v.Name, Value: v.Value, Secret: true, Scope: "runtime"}
				}
			}
		}
		variables := []store.Variable{}
		for _, v := range environment {
			variables = append(variables, v)
		}
		sort.Slice(variables, func(i, j int) bool { return variables[i].Name < variables[j].Name })
		variables = store.PublicConfig(store.Config{Variables: variables}).Variables
		volumes := []store.Volume{}
		for _, v := range svc.Volumes {
			source := v.Source
			if v.Type == "bind" {
				if rel, e := filepath.Rel(p.WorkingDir, source); e == nil {
					source = "./" + filepath.ToSlash(rel)
				}
			}
			volumes = append(volumes, store.Volume{Name: source, Path: v.Target})
		}
		out = append(out, store.Service{Name: name, Image: svc.Image, Port: port, HealthPath: health, Volumes: volumes, Variables: variables, OneShot: oneShot})
	}
	return out
}

// ComposePlan is serialized only into the encrypted deployment column. It never
// crosses a public API boundary. Files live in Docker volumes, not host binds.
type composePlan struct {
	PreviousDesiredState string         `json:"previousDesiredState,omitempty"`
	Project              map[string]any `json:"project"`
	Masks                []string       `json:"masks"`
}

func ComposeMasks(d store.Deployment) []string {
	var p composePlan
	json.Unmarshal([]byte(d.ComposePlan), &p)
	return p.Masks
}
func RedactDeployment(d store.Deployment, line string) string {
	line = Redact(d.Config, line)
	for _, v := range ComposeMasks(d) {
		if v != "" {
			line = strings.ReplaceAll(line, v, "[REDACTED]")
		}
	}
	return line
}

func (r *DockerRuntime) composeCommand(ctx context.Context, d store.Deployment, arguments []string, log func(string)) error {
	var plan composePlan
	if e := json.Unmarshal([]byte(d.ComposePlan), &plan); e != nil {
		return errors.New("Compose release configuration is unavailable")
	}
	dir, e := os.MkdirTemp(filepath.Join(r.DataDir, "work"), "deployment-compose-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(dir)
	// A normalized model is parsed by the CLI again. Escape dollars to prevent
	// a second interpolation pass from changing commands or revealing host env.
	if e = r.checkComposeOwnership(ctx, d, plan); e != nil {
		return e
	}
	data, e := json.Marshal(escapeCompose(plan.Project))
	if e != nil {
		return e
	}
	file := filepath.Join(dir, "compose.json")
	if e = os.WriteFile(file, data, 0600); e != nil {
		return e
	}
	envFile := filepath.Join(dir, "empty.env")
	if e = os.WriteFile(envFile, nil, 0600); e != nil {
		return e
	}
	args := []string{"compose", "--ansi", "never", "--progress", "plain", "--project-name", r.composeProject(d.ApplicationID), "--project-directory", dir, "--env-file", envFile, "-f", file}
	args = append(args, arguments...)
	output := func(line string) {
		for _, v := range plan.Masks {
			if v != "" {
				line = strings.ReplaceAll(line, v, "[REDACTED]")
			}
		}
		log(Redact(d.Config, line))
	}
	if e = r.dockerCommand(ctx, args, []string{"COMPOSE_BAKE=false"}, output); e != nil {
		return errors.New("Compose operation failed; check the deployment output")
	}
	return nil
}
func escapeCompose(value any) any {
	switch v := value.(type) {
	case string:
		return strings.ReplaceAll(v, "$", "$$")
	case map[string]any:
		out := map[string]any{}
		for k, v := range v {
			out[k] = escapeCompose(v)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, v := range v {
			out[i] = escapeCompose(v)
		}
		return out
	default:
		return value
	}
}

func (r *DockerRuntime) checkComposeOwnership(ctx context.Context, d store.Deployment, p composePlan) error {
	items, e := r.Client.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: make(client.Filters).Add("label", "com.docker.compose.project="+r.composeProject(d.ApplicationID))})
	if e != nil {
		return e
	}
	for _, c := range items.Items {
		if !owned(c.Labels, r.Instance, d.ApplicationID) {
			return errors.New("Compose project contains a container not owned by this Shipper application")
		}
	}
	for _, kind := range []string{"volumes", "networks"} {
		resources, _ := p.Project[kind].(map[string]any)
		for _, raw := range resources {
			resource, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if external, _ := resource["external"].(bool); external {
				continue
			}
			name, _ := resource["name"].(string)
			if kind == "volumes" {
				v, e := r.Client.VolumeInspect(ctx, name, client.VolumeInspectOptions{})
				if e == nil && !owned(v.Volume.Labels, r.Instance, d.ApplicationID) {
					return errors.New("Compose volume ownership check failed")
				}
				if e != nil && !errdefs.IsNotFound(e) {
					return e
				}
			} else {
				n, e := r.Client.NetworkInspect(ctx, name, client.NetworkInspectOptions{})
				if e == nil && !owned(n.Network.Labels, r.Instance, d.ApplicationID) {
					return errors.New("Compose network ownership check failed")
				}
				if e != nil && !errdefs.IsNotFound(e) {
					return e
				}
			}
		}
	}
	return nil
}

func (r *DockerRuntime) PrepareCompose(ctx context.Context, d *store.Deployment, root, repository string, log func(string)) error {
	p, e := LoadCompose(ctx, d.Config, root, r.composeProject(d.ApplicationID), repository)
	if e != nil {
		return e
	}
	d.ResolvedMethod = "compose"
	d.Services = ComposeServices(p, d.Config)
	if p.Volumes == nil {
		p.Volumes = ct.Volumes{}
	}
	labels := r.labels(d.ApplicationID, "")
	delete(labels, "io.shipper.deployment.id")
	// A stable label keeps unchanged database containers part of the active app.
	labels["io.shipper.resource"] = "application"
	for key, vol := range p.Volumes {
		if source := d.Config.VolumeSources[key]; source != "" {
			if e := r.checkVolumeSource(ctx, d.ApplicationID, source); e != nil {
				return e
			}
			vol.Name = source
			vol.External = true
		}
		if !bool(vol.External) {
			vol.Name = r.composeProject(d.ApplicationID) + "-" + key
			vol.Labels = ct.Labels(labels)
			if vol.Driver != "" && vol.Driver != "local" || len(vol.DriverOpts) > 0 {
				return errors.New("Managed Compose volumes must use the local driver without host options")
			}
			if existing, err := r.Client.VolumeInspect(ctx, vol.Name, client.VolumeInspectOptions{}); err == nil && !owned(existing.Volume.Labels, r.Instance, d.ApplicationID) {
				return errors.New("Compose volume name belongs to another application")
			} else if err != nil && !errdefs.IsNotFound(err) {
				return err
			}
		} else {
			if _, e = r.Client.VolumeInspect(ctx, vol.Name, client.VolumeInspectOptions{}); e != nil {
				return fmt.Errorf("External volume %s must already exist", key)
			}
		}
		p.Volumes[key] = vol
	}
	for key, n := range p.Networks {
		if bool(n.External) {
			return errors.New("External Compose networks are not supported; Shipper attaches public services to its proxy")
		}
		if n.Driver != "" && n.Driver != "bridge" || len(n.DriverOpts) > 0 {
			return errors.New("Compose networks must use the bridge driver without host options")
		}
		n.Name = r.composeProject(d.ApplicationID) + "-" + key
		n.Labels = ct.Labels(labels)
		if existing, err := r.Client.NetworkInspect(ctx, n.Name, client.NetworkInspectOptions{}); err == nil && !owned(existing.Network.Labels, r.Instance, d.ApplicationID) {
			return errors.New("Compose network name belongs to another application")
		} else if err != nil && !errdefs.IsNotFound(err) {
			return err
		}
		p.Networks[key] = n
	}
	p.Networks["shipper_edge"] = ct.NetworkConfig{Name: r.network(), External: true}
	files := map[string][]byte{}
	fileMount := func(source, target string, readonly bool) (ct.ServiceVolumeConfig, error) {
		abs, e := checkedFile(repository, source)
		if e != nil {
			return ct.ServiceVolumeConfig{}, e
		}
		info, e := os.Stat(abs)
		if e != nil {
			return ct.ServiceVolumeConfig{}, e
		}
		key := fmt.Sprintf("file%d", len(files))
		// Copy regular files as a directory so Docker's volume subpath works
		// consistently for both file and directory mounts on remote daemons.
		if info.IsDir() {
			files[key+"/"] = nil
			e = collectComposeFiles(abs, key, files)
		} else {
			if info.Size() > 16<<20 {
				return ct.ServiceVolumeConfig{}, errors.New("Referenced Compose files are limited to 16 MiB")
			}
			files[key], e = os.ReadFile(abs)
		}
		if e != nil {
			return ct.ServiceVolumeConfig{}, e
		}
		return ct.ServiceVolumeConfig{Type: "volume", Source: "shipper_files", Target: target, ReadOnly: readonly, Volume: &ct.ServiceVolumeVolume{Subpath: key, NoCopy: true}}, nil
	}
	mask := []string{}
	for name, svc := range p.Services {
		if svc.Privileged || len(svc.Devices) > 0 || len(svc.DeviceCgroupRules) > 0 || len(svc.Gpus) > 0 || svc.NetworkMode != "" || svc.Pid != "" || svc.Ipc != "" || svc.Uts != "" || svc.UserNSMode != "" || svc.UseAPISocket || len(svc.VolumesFrom) > 0 || len(svc.ExternalLinks) > 0 || len(svc.PreStart) > 0 || len(svc.PostStart) > 0 || len(svc.PreStop) > 0 || svc.Provider != nil || len(svc.CapAdd) > 0 || len(svc.SecurityOpt) > 0 || len(svc.Sysctls) > 0 {
			return fmt.Errorf("Service %s requests host access or privileged options that Shipper does not support", name)
		}
		if svc.GetScale() != 1 {
			return errors.New("Compose service replicas are not supported yet")
		}
		svc.ContainerName = ContainerName(d.ApplicationName, name, composeServiceKey(d.ApplicationID+":"+name))
		if existing, err := r.Client.ContainerInspect(ctx, svc.ContainerName, client.ContainerInspectOptions{}); err == nil && !owned(existing.Container.Config.Labels, r.Instance, d.ApplicationID) {
			return errors.New("Compose container name belongs to another application")
		} else if err != nil && !errdefs.IsNotFound(err) {
			return err
		}
		for k := range svc.Labels {
			if strings.HasPrefix(k, "io.shipper.") || strings.HasPrefix(k, "com.docker.compose.") || strings.HasPrefix(k, "traefik.") {
				delete(svc.Labels, k)
			}
		}
		if svc.Labels == nil {
			svc.Labels = ct.Labels{}
		}
		for k, v := range labels {
			svc.Labels[k] = v
		}
		svc.Labels["io.shipper.infrastructure"] = ""
		svc.SecurityOpt = []string{"no-new-privileges:true"}
		svc.Logging = &ct.LoggingConfig{Driver: "json-file", Options: map[string]string{"max-size": "5m", "max-file": "2"}}
		if svc.MemLimit == 0 {
			svc.MemLimit = 512 << 20
			if svc.Deploy != nil && svc.Deploy.Resources.Limits != nil && svc.Deploy.Resources.Limits.MemoryBytes > 0 {
				svc.MemLimit = svc.Deploy.Resources.Limits.MemoryBytes
			}
		}
		if svc.CPUS == 0 {
			svc.CPUS = 1
			if svc.Deploy != nil && svc.Deploy.Resources.Limits != nil && svc.Deploy.Resources.Limits.NanoCPUs > 0 {
				svc.CPUS = float32(svc.Deploy.Resources.Limits.NanoCPUs)
			}
		}
		if svc.PidsLimit == 0 {
			svc.PidsLimit = 256
		}
		if svc.Environment == nil {
			svc.Environment = ct.MappingWithEquals{}
		}
		for _, override := range d.Config.Services {
			if override.Name == name {
				if override.MemoryMB > 0 {
					svc.MemLimit = ct.UnitBytes(override.MemoryMB << 20)
					if svc.Deploy != nil && svc.Deploy.Resources.Limits != nil {
						svc.Deploy.Resources.Limits.MemoryBytes = svc.MemLimit
					}
				}
				for _, key := range override.UnsetVariables {
					delete(svc.Environment, key)
				}
				for _, v := range override.Variables {
					value := v.Value
					svc.Environment[v.Name] = &value
				}
			}
		}
		for _, v := range svc.Environment {
			if v != nil && *v != "" {
				mask = append(mask, *v)
			}
		}
		// Published host ports become internal ports; domains are the sole public
		// ingress. Databases never become public just because a dev file says so.
		svc.Ports = nil
		for i := range svc.Volumes {
			v := &svc.Volumes[i]
			if v.Type == "bind" {
				next, err := fileMount(v.Source, v.Target, v.ReadOnly)
				if err != nil {
					return err
				}
				*v = next
			} else if v.Type == "volume" && v.Source == "" {
				key := "data-" + name + "-" + strconv.Itoa(i)
				v.Source = key
				p.Volumes[key] = ct.VolumeConfig{Name: r.composeProject(d.ApplicationID) + "-" + key, Labels: ct.Labels(labels)}
			} else if v.Type != "volume" && v.Type != "tmpfs" {
				return errors.New("Unsupported Compose mount type")
			}
		}
		for _, f := range svc.Configs {
			def := p.Configs[f.Source]
			if bool(def.External) {
				return errors.New("External Compose configs are not supported")
			}
			target := f.Target
			if target == "" {
				target = "/" + f.Source
			}
			var v ct.ServiceVolumeConfig
			if def.File != "" {
				v, e = fileMount(def.File, target, true)
				if e == nil {
					if info, err := os.Stat(def.File); err == nil && info.Size() <= 128<<10 {
						if data, err := os.ReadFile(def.File); err == nil {
							mask = append(mask, string(data))
						}
					}
				}
			} else {
				key := fmt.Sprintf("file%d", len(files))
				content := def.Content
				if def.Environment != "" {
					content = p.Environment[def.Environment]
				}
				files[key] = []byte(content)
				mask = append(mask, content)
				v = ct.ServiceVolumeConfig{Type: "volume", Source: "shipper_files", Target: target, ReadOnly: true, Volume: &ct.ServiceVolumeVolume{Subpath: key, NoCopy: true}}
			}
			if e != nil {
				return e
			}
			svc.Volumes = append(svc.Volumes, v)
		}
		for _, f := range svc.Secrets {
			def := p.Secrets[f.Source]
			if bool(def.External) {
				return errors.New("External Compose secrets are not supported")
			}
			target := f.Target
			if target == "" {
				target = f.Source
			}
			if !strings.HasPrefix(target, "/") {
				target = "/run/secrets/" + target
			}
			var v ct.ServiceVolumeConfig
			if def.File != "" {
				v, e = fileMount(def.File, target, true)
				if e == nil {
					if info, err := os.Stat(def.File); err == nil && info.Size() <= 128<<10 {
						if data, err := os.ReadFile(def.File); err == nil {
							mask = append(mask, string(data))
						}
					}
				}
			} else {
				key := fmt.Sprintf("file%d", len(files))
				content := p.Environment[def.Environment]
				if content == "" {
					return fmt.Errorf("Secret %s has no value", f.Source)
				}
				files[key] = []byte(content)
				mask = append(mask, content)
				v = ct.ServiceVolumeConfig{Type: "volume", Source: "shipper_files", Target: target, ReadOnly: true, Volume: &ct.ServiceVolumeVolume{Subpath: key, NoCopy: true}}
			}
			if e != nil {
				return e
			}
			svc.Volumes = append(svc.Volumes, v)
		}
		svc.Configs = nil
		svc.Secrets = nil
		for _, summary := range d.Services {
			if summary.Name == name && summary.Port > 0 {
				if svc.Networks == nil {
					svc.Networks = map[string]*ct.ServiceNetworkConfig{}
				}
				svc.Networks["shipper_edge"] = &ct.ServiceNetworkConfig{Aliases: []string{composeAlias(d.ApplicationID, name)}}
			}
		}
		if svc.Build != nil {
			if svc.Build.Labels == nil {
				svc.Build.Labels = ct.Labels{}
			}
			for k, v := range r.labels(d.ApplicationID, d.ID) {
				svc.Build.Labels[k] = v
			}
			svc.Build.Labels["io.shipper.infrastructure"] = ""
			for _, value := range svc.Build.Args {
				if value != nil {
					mask = append(mask, *value)
				}
			}
			svc.Image = "shipper/" + r.Instance[:12] + "/" + d.ApplicationID + "-" + composeServiceKey(name) + ":" + d.ID
		}
		if svc.Restart == "" {
			svc.Restart = "unless-stopped"
			for _, meta := range d.Services {
				if meta.Name == name && meta.OneShot {
					svc.Restart = "no"
				}
			}
		}
		p.Services[name] = svc
	}
	if len(files) > 0 {
		volume := r.composeProject(d.ApplicationID) + "-files-" + d.ID
		p.Volumes["shipper_files"] = ct.VolumeConfig{Name: volume, External: true}
		if e = r.composeFiles(ctx, d, volume, files); e != nil {
			return e
		}
	}
	p.Configs = nil
	p.Secrets = nil

	fragments := map[string]bool{}
	for _, value := range mask {
		if value != "" {
			fragments[value] = true
		}
		for _, line := range strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n") {
			if line != "" {
				fragments[line] = true
			}
		}
	}
	mask = nil
	for value := range fragments {
		mask = append(mask, value)
	}
	sort.Slice(mask, func(i, j int) bool { return len(mask[i]) > len(mask[j]) })
	data, e := p.MarshalJSON()
	if e != nil {
		return e
	}
	var model map[string]any
	if e = json.Unmarshal(data, &model); e != nil {
		return e
	}
	for name, raw := range model["services"].(map[string]any) {
		for _, override := range d.Config.Services {
			if override.Name == name {
				applyComposePolicy(raw.(map[string]any), override.Runtime)
			}
		}
	}
	plan := composePlan{Project: model, Masks: mask}
	data, e = json.Marshal(plan)
	d.ComposePlan = string(data)
	return e
}

func collectComposeFiles(root, key string, files map[string][]byte) error {
	var total int64
	return filepath.WalkDir(root, func(file string, entry os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if file == root {
			return nil
		}
		if entry.IsDir() {
			rel, _ := filepath.Rel(root, file)
			files[key+"/"+filepath.ToSlash(rel)+"/"] = nil
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("Referenced directories cannot contain symlinks")
		}
		info, e := entry.Info()
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() {
			return errors.New("Referenced files must be regular files")
		}
		total += info.Size()
		if total > 32<<20 || len(files) > 10000 {
			return errors.New("Referenced Compose files exceed 32 MiB or 10,000 files")
		}
		rel, e := filepath.Rel(root, file)
		if e != nil {
			return e
		}
		files[key+"/"+filepath.ToSlash(rel)], e = os.ReadFile(file)
		return e
	})
}
func (r *DockerRuntime) composeFiles(ctx context.Context, d *store.Deployment, volume string, files map[string][]byte) error {
	labels := r.labels(d.ApplicationID, d.ID)
	labels["io.shipper.infrastructure"] = "compose-files"
	if v, e := r.Client.VolumeInspect(ctx, volume, client.VolumeInspectOptions{}); e == nil {
		if !owned(v.Volume.Labels, r.Instance, d.ApplicationID) {
			return errors.New("Compose file volume ownership check failed")
		}
	} else if errdefs.IsNotFound(e) {
		if _, e = r.Client.VolumeCreate(ctx, client.VolumeCreateOptions{Name: volume, Labels: labels}); e != nil {
			return e
		}
	} else {
		return e
	}
	if e := r.pull(ctx, ApacheImage, func(string) {}); e != nil {
		return e
	}
	helper, e := r.Client.ContainerCreate(ctx, client.ContainerCreateOptions{Config: &container.Config{Image: ApacheImage, Labels: labels}, HostConfig: &container.HostConfig{NetworkMode: "none", Mounts: []mount.Mount{{Type: mount.TypeVolume, Source: volume, Target: "/payload"}}}})
	if e != nil {
		return e
	}
	defer r.Client.ContainerRemove(context.WithoutCancel(ctx), helper.ID, client.ContainerRemoveOptions{Force: true})
	// copy creates directory entries for each parent. File ownership matches the
	// source-independent read-only mounts; no host filesystem path reaches Docker.
	all := map[string][]byte{}
	for k, v := range files {
		all[k] = v
		dir := filepath.ToSlash(filepath.Dir(k))
		for dir != "." && dir != "" {
			all[dir+"/"] = nil
			dir = filepath.ToSlash(filepath.Dir(dir))
		}
	}
	keys := make([]string, 0, len(all))
	for k := range all {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b bytes.Buffer
	tw := tar.NewWriter(&b)
	for _, k := range keys {
		v := all[k]
		h := &tar.Header{Name: k, Mode: 0644, Size: int64(len(v))}
		if strings.HasSuffix(k, "/") {
			h.Typeflag = tar.TypeDir
			h.Mode = 0755
		}
		if e = tw.WriteHeader(h); e != nil {
			return e
		}
		if _, e = tw.Write(v); e != nil {
			return e
		}
	}
	if e = tw.Close(); e != nil {
		return e
	}
	_, e = r.Client.CopyToContainer(ctx, helper.ID, client.CopyToContainerOptions{DestinationPath: "/payload", Content: &b, CopyUIDGID: true})
	return e
}

func (r *DockerRuntime) BuildCompose(ctx context.Context, d *store.Deployment, log func(string)) error {
	var plan composePlan
	if e := json.Unmarshal([]byte(d.ComposePlan), &plan); e != nil {
		return e
	}
	services := plan.Project["services"].(map[string]any)
	hasBuild := false
	for _, raw := range services {
		svc := raw.(map[string]any)
		if svc["build"] != nil {
			hasBuild = true
		}
	}
	if hasBuild {
		if e := r.EnsureBuilder(ctx, log); e != nil {
			return e
		}
		args := []string{"build", "--builder", r.builderName()}
		if d.NoCache {
			args = append(args, "--no-cache")
		}
		if e := r.composeCommand(ctx, *d, args, log); e != nil {
			return e
		}
	}
	// Pull every service before applying any changes. Pin the resolved images so
	// a failed future build can restore the exact previous runtime configuration.
	for name, raw := range services {
		svc := raw.(map[string]any)
		image, _ := svc["image"].(string)
		if image == "" {
			return fmt.Errorf("Service %s has no image", name)
		}
		if svc["build"] == nil && !strings.HasPrefix(image, "sha256:") {
			log("Pulling " + image)
			stream, e := r.Client.ImagePull(ctx, image, client.ImagePullOptions{})
			if e != nil {
				return e
			}
			e = stream.Wait(ctx)
			stream.Close()
			if e != nil {
				return e
			}
		} else if e := r.pull(ctx, image, log); e != nil {
			return e
		}
		info, e := r.Client.ImageInspect(ctx, image)
		if e != nil {
			return e
		}
		svc["image"] = info.ID
		delete(svc, "build")
		svc["pull_policy"] = "never"
	}
	data, e := json.Marshal(plan)
	d.ComposePlan = string(data)
	if e != nil {
		return e
	}
	return r.composeStartup(ctx, d)
}
func (r *DockerRuntime) UpCompose(ctx context.Context, d store.Deployment, log func(string)) error {
	wait := max(90, StartupSeconds(d.Config.Runtime))
	for _, svc := range d.Config.Services {
		wait = max(wait, StartupSeconds(svc.Runtime))
	}
	for _, h := range d.Config.Hooks {
		if h.Stage == "before_start" {
			wait = max(wait, 90+h.Command.Timeout)
		}
	}
	return r.composeCommand(ctx, d, []string{"up", "-d", "--no-build", "--pull", "never", "--remove-orphans", "--wait", "--wait-timeout", strconv.Itoa(wait)}, log)
}
func (r *DockerRuntime) ComposeHealthy(ctx context.Context, d store.Deployment) error {
	items, e := r.composeContainers(ctx, d.ApplicationID)
	if e != nil {
		return e
	}
	for _, svc := range d.Services {
		found := false
		for _, c := range items {
			if c.Labels["io.shipper.infrastructure"] != "" {
				continue
			}
			if c.Labels["com.docker.compose.service"] == svc.Name {
				found = true
				if c.State != "running" {
					info, e := r.Client.ContainerInspect(ctx, c.ID, client.ContainerInspectOptions{})
					if e != nil {
						return e
					}
					if info.Container.State.ExitCode != 0 || !svc.OneShot {
						return fmt.Errorf("Service %s exited unsuccessfully", svc.Name)
					}
				}
				if svc.Port > 0 {
					candidate := d
					candidate.ContainerID = c.ID
					candidate.RuntimePort = svc.Port
					candidate.Config.HealthPath = svc.HealthPath
					for _, override := range d.Config.Services {
						if override.Name == svc.Name {
							candidate.Config.Runtime = override.Runtime
						}
					}
					if e = r.Healthy(ctx, candidate); e != nil {
						return fmt.Errorf("Service %s: %w", svc.Name, e)
					}
				}
			}
		}
		if !found {
			return fmt.Errorf("Service %s has no container", svc.Name)
		}
	}
	return nil
}
func (r *DockerRuntime) composeContainers(ctx context.Context, app string) ([]container.Summary, error) {
	items, e := r.Client.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: make(client.Filters).Add("label", "io.shipper.instance="+r.Instance).Add("label", "io.shipper.application.id="+app).Add("label", "com.docker.compose.project="+r.composeProject(app))})
	return items.Items, e
}
func (r *DockerRuntime) ComposeLifecycle(ctx context.Context, d store.Deployment, action string) error {
	if action == "start" {
		return r.UpCompose(ctx, d, func(string) {})
	}
	args := []string{action}
	if d.Config.Runtime.StopSeconds > 0 {
		args = append(args, "--timeout", strconv.Itoa(StopSeconds(d.Config.Runtime)))
	}
	return r.composeCommand(ctx, d, args, func(string) {})
}
