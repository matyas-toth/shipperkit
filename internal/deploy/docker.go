package deploy

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"shipper/internal/store"
)

const ApacheImage = "httpd:2.4-alpine@sha256:070da7661f0be8a05430b2c125c0cde9e53176b4048c813024021969a0e3893c"
const TraefikImage = "traefik:v3.6@sha256:31267173a15b4944e797a76ffd9c419707c8d8b32fe5b610f80cd0cfa05f372d"

type ProxyConfig struct {
	ReservedPorts      []int  `json:"-"`
	DashboardURL       string `json:"dashboardURL,omitempty"`
	DashboardUpstream  string `json:"dashboardUpstream,omitempty"`
	DashboardContainer string `json:"-"`

	BindIP    string `json:"bindIP"`
	HTTPPort  int    `json:"httpPort"`
	HTTPSPort int    `json:"httpsPort"`
	ACMEEmail string `json:"acmeEmail"`
	ACMECA    string `json:"acmeCA"`
	AutoHTTPS bool   `json:"autoHTTPS,omitempty"`
}
type Runtime interface {
	Ensure(context.Context) error
	Package(context.Context, store.Deployment, []byte, func(string)) (string, error)
	Start(context.Context, store.Deployment) (string, error)
	Healthy(context.Context, store.Deployment) error
	Routes(context.Context, []store.Application, map[string]store.Deployment) error
	RemoveDeployment(context.Context, store.Deployment) error
	RemoveApplication(context.Context, string) error
	ProbeLocal(context.Context, store.Deployment) (bool, error)
}
type DockerRuntime struct {
	Client          *client.Client
	Instance        string
	Config          ProxyConfig
	DataDir         string
	ACMEContact     func(context.Context) (string, error)
	DashboardDomain func(context.Context) string
}

func (r *DockerRuntime) prefix() string  { return "shipper-" + r.Instance[:12] }
func (r *DockerRuntime) proxy() string   { return r.prefix() + "-proxy" }
func (r *DockerRuntime) network() string { return r.prefix() + "-edge" }
func (r *DockerRuntime) labels(app, dep string) map[string]string {
	out := map[string]string{"io.shipper.instance": r.Instance}
	if app != "" {
		out["io.shipper.managed"] = "true"
		out["io.shipper.application.id"] = app
		out["io.shipper.deployment.id"] = dep
	} else {
		out["io.shipper.infrastructure"] = "proxy"
		out["io.shipper.proxy.config"] = r.configHash()
	}
	return out
}
func (r *DockerRuntime) pull(ctx context.Context, image string, log func(string)) error {
	if _, e := r.Client.ImageInspect(ctx, image); e == nil {
		return nil
	}
	log("Pulling " + strings.Split(image, "@")[0])
	stream, e := r.Client.ImagePull(ctx, image, client.ImagePullOptions{})
	if e != nil {
		return e
	}
	defer stream.Close()
	return stream.Wait(ctx)
}
func owned(labels map[string]string, instance, app string) bool {
	return labels["io.shipper.instance"] == instance && (app == "" || labels["io.shipper.application.id"] == app)
}
func (r *DockerRuntime) Ensure(ctx context.Context) error {
	config, err := r.EffectiveProxy(ctx)
	if err != nil {
		return err
	}
	resolved := *r
	resolved.Config = config
	return resolved.ensureProxy(ctx)
}

func (r *DockerRuntime) EffectiveProxy(ctx context.Context) (ProxyConfig, error) {
	c := r.Config
	if c.DashboardContainer != "" && r.DashboardDomain != nil {
		if domain := r.DashboardDomain(ctx); domain != "" {
			c.DashboardURL = "https://" + domain
			if c.DashboardUpstream == "" {
				c.DashboardUpstream = "http://dashboard-" + r.Instance[:12] + ":8080"
			}
		} else {
			c.DashboardURL = ""
			c.DashboardUpstream = ""
		}
	}
	if c.AutoHTTPS && c.ACMEEmail == "" && r.ACMEContact != nil {
		var err error
		c.ACMEEmail, err = r.ACMEContact(ctx)
		return c, err
	}
	return c, nil
}

func (r *DockerRuntime) ensureProxy(ctx context.Context) error {
	if info, e := r.Client.NetworkInspect(ctx, r.network(), client.NetworkInspectOptions{}); e == nil {
		if !owned(info.Network.Labels, r.Instance, "") {
			return errors.New("The proxy network name is already in use")
		}
	} else if errdefs.IsNotFound(e) {
		if _, e = r.Client.NetworkCreate(ctx, r.network(), client.NetworkCreateOptions{Driver: "bridge", Labels: r.labels("", "")}); e != nil {
			return e
		}
	} else {
		return e
	}
	if e := r.connectDashboard(ctx); e != nil {
		return e
	}
	volume := r.prefix() + "-proxy-data"
	if v, e := r.Client.VolumeInspect(ctx, volume, client.VolumeInspectOptions{}); e == nil {
		if !owned(v.Volume.Labels, r.Instance, "") {
			return errors.New("The proxy data volume name is already in use")
		}
	} else if errdefs.IsNotFound(e) {
		if _, e = r.Client.VolumeCreate(ctx, client.VolumeCreateOptions{Name: volume, Labels: r.labels("", "")}); e != nil {
			return e
		}
	} else {
		return e
	}
	inspected, e := r.Client.ContainerInspect(ctx, r.proxy(), client.ContainerInspectOptions{})
	if e == nil {
		if !owned(inspected.Container.Config.Labels, r.Instance, "") || inspected.Container.Config.Labels["io.shipper.infrastructure"] != "proxy" {
			return errors.New("The proxy container name is already in use")
		}
		if inspected.Container.Config.Labels["io.shipper.proxy.config"] != r.configHash() {
			previous := *r
			previous.Config.ACMEEmail = ""
			if !r.Config.AutoHTTPS || r.Config.ACMEEmail == "" || inspected.Container.Config.Labels["io.shipper.proxy.config"] != previous.configHash() {
				return errors.New("Proxy configuration differs from the installed settings")
			}
			// Only the one-way first-contact transition is automatic. Keep the
			// certificate/routes volume and check the image before releasing ports.
			if e = r.pull(ctx, TraefikImage, func(string) {}); e != nil {
				return e
			}
			if _, e = r.Client.ContainerRemove(ctx, inspected.Container.ID, client.ContainerRemoveOptions{Force: true}); e != nil {
				return e
			}
		} else {
			if e = r.restoreACME(ctx, r.proxy()); e != nil {
				return e
			}
			created, err := r.prepareACMEFile(ctx, r.proxy())
			if err != nil {
				return err
			}
			if !inspected.Container.State.Running {
				_, e = r.Client.ContainerStart(ctx, r.proxy(), client.ContainerStartOptions{})
			} else if created {
				// Traefik skips a resolver that cannot create its store at boot.
				// Restart it once after repairing that missing store.
				_, e = r.Client.ContainerRestart(ctx, r.proxy(), client.ContainerRestartOptions{Timeout: ptr(5)})
			}
			return e
		}
	} else if !errdefs.IsNotFound(e) {
		return e
	}
	if e = r.pull(ctx, TraefikImage, func(string) {}); e != nil {
		return e
	}
	args := []string{"--entrypoints.web.address=:8080", "--entrypoints.websecure.address=:8443", "--entrypoints.traefik.address=:9000", "--providers.file.directory=/data/dynamic", "--providers.file.watch=true", "--providers.providersThrottleDuration=100ms", "--api.insecure=true", "--api.dashboard=false", "--ping=true", "--log.level=ERROR"}
	if r.Config.ACMEEmail != "" {
		args = append(args, "--certificatesresolvers.letsencrypt.acme.email="+r.Config.ACMEEmail, "--certificatesresolvers.letsencrypt.acme.storage=/data/acme.json", "--certificatesresolvers.letsencrypt.acme.httpchallenge.entrypoint=web")
		if r.Config.ACMECA != "" {
			args = append(args, "--certificatesresolvers.letsencrypt.acme.caserver="+r.Config.ACMECA)
		}
	}
	ip, e := netip.ParseAddr(r.Config.BindIP)
	if e != nil {
		return e
	}
	bindings := network.PortMap{network.MustParsePort("8080/tcp"): []network.PortBinding{{HostIP: ip, HostPort: strconv.Itoa(r.Config.HTTPPort)}}, network.MustParsePort("8443/tcp"): []network.PortBinding{{HostIP: ip, HostPort: strconv.Itoa(r.Config.HTTPSPort)}}}
	created, e := r.Client.ContainerCreate(ctx, client.ContainerCreateOptions{Name: r.proxy(), Config: &container.Config{Image: TraefikImage, Cmd: args, User: "65532:65532", Labels: r.labels("", "")}, HostConfig: &container.HostConfig{PortBindings: bindings, ReadonlyRootfs: true, CapDrop: []string{"ALL"}, SecurityOpt: []string{"no-new-privileges:true"}, Mounts: []mount.Mount{{Type: mount.TypeVolume, Source: volume, Target: "/data"}}, RestartPolicy: container.RestartPolicy{Name: "unless-stopped"}, Resources: container.Resources{Memory: 128 << 20, NanoCPUs: 1e9, PidsLimit: ptr(int64(128))}, LogConfig: container.LogConfig{Type: "json-file", Config: map[string]string{"max-size": "5m", "max-file": "2"}}}, NetworkingConfig: &network.NetworkingConfig{EndpointsConfig: map[string]*network.EndpointSettings{r.network(): {}}}})
	if e != nil {
		return e
	}
	// Initialize the watched directory before startup; it persists independently of Shipper.
	initialRoutes := Routes(nil, nil)
	r.dashboardRoute(initialRoutes)
	initial, _ := json.Marshal(initialRoutes)
	if e = r.copy(ctx, created.ID, "/data", map[string][]byte{"./": nil, "dynamic/": nil, "dynamic/routes.yaml": initial}); e != nil {
		return e
	}
	if e = r.restoreACME(ctx, created.ID); e != nil {
		return e
	}
	if _, e = r.prepareACMEFile(ctx, created.ID); e != nil {
		return e
	}
	_, e = r.Client.ContainerStart(ctx, created.ID, client.ContainerStartOptions{})
	return e
}

func (r *DockerRuntime) prepareACMEFile(ctx context.Context, proxy string) (bool, error) {
	if r.Config.ACMEEmail == "" {
		return false, nil
	}
	stat, e := r.Client.ContainerStatPath(ctx, proxy, client.ContainerStatPathOptions{Path: "/data/acme.json"})
	if e == nil {
		if !stat.Stat.Mode.IsRegular() || stat.Stat.Mode.Perm() != 0600 {
			return false, errors.New("Certificate storage must be a regular file with mode 0600")
		}
		return false, nil
	}
	if !errdefs.IsNotFound(e) {
		return false, e
	}
	// A newly mounted named volume is owned by root on Linux. Seed the file
	// as the proxy's unprivileged UID instead of changing volume permissions.
	e = r.copy(ctx, proxy, "/data", map[string][]byte{"acme.json": []byte("{}")})
	return e == nil, e
}
func ptr[T any](v T) *T { return &v }
func (r *DockerRuntime) configHash() string {
	data, _ := json.Marshal(struct {
		BindIP    string `json:"bindIP"`
		HTTPPort  int    `json:"httpPort"`
		HTTPSPort int    `json:"httpsPort"`
		ACMEEmail string `json:"acmeEmail"`
		ACMECA    string `json:"acmeCA"`
	}{r.Config.BindIP, r.Config.HTTPPort, r.Config.HTTPSPort, r.Config.ACMEEmail, r.Config.ACMECA})
	return fmt.Sprintf("%x", sha256.Sum256(data))
}
func (r *DockerRuntime) copy(ctx context.Context, id, dest string, files map[string][]byte) error {
	var b bytes.Buffer
	tw := tar.NewWriter(&b)
	for name, data := range files {
		h := &tar.Header{Name: name, Mode: 0600, Size: int64(len(data)), Uid: 65532, Gid: 65532}
		if strings.HasSuffix(name, "/") {
			h.Typeflag = tar.TypeDir
			h.Mode = 0700
		}
		if e := tw.WriteHeader(h); e != nil {
			return e
		}
		if _, e := tw.Write(data); e != nil {
			return e
		}
	}
	if e := tw.Close(); e != nil {
		return e
	}
	_, e := r.Client.CopyToContainer(ctx, id, client.CopyToContainerOptions{DestinationPath: dest, Content: &b, CopyUIDGID: true})
	return e
}
func (r *DockerRuntime) Package(ctx context.Context, d store.Deployment, archive []byte, log func(string)) (string, error) {
	tag := "shipper/" + r.Instance[:12] + "/" + d.ApplicationID + ":" + d.ID
	if img, e := r.Client.ImageInspect(ctx, tag); e == nil {
		if !owned(img.Config.Labels, r.Instance, d.ApplicationID) {
			return "", errors.New("Image ownership check failed")
		}
		return img.ID, nil
	}
	if e := r.pull(ctx, ApacheImage, log); e != nil {
		return "", e
	}
	name := r.prefix() + "-package-" + d.ID
	if old, e := r.Client.ContainerInspect(ctx, name, client.ContainerInspectOptions{}); e == nil {
		if !owned(old.Container.Config.Labels, r.Instance, d.ApplicationID) {
			return "", errors.New("Packaging container ownership check failed")
		}
		if _, e = r.Client.ContainerRemove(ctx, name, client.ContainerRemoveOptions{Force: true}); e != nil {
			return "", e
		}
	}
	cfg := &container.Config{Image: ApacheImage, Labels: r.labels(d.ApplicationID, d.ID), User: "65532:65532", Cmd: []string{"httpd", "-D", "FOREGROUND"}, Entrypoint: []string{}, Healthcheck: &container.HealthConfig{Test: []string{"CMD", "wget", "-q", "-O", "/dev/null", "http://127.0.0.1:8080/"}, Interval: 5 * time.Second, Timeout: 3 * time.Second, Retries: 3, StartPeriod: 5 * time.Second}}
	cfg.Labels["io.shipper.infrastructure"] = "packaging"
	cfg.Labels["org.opencontainers.image.revision"] = d.Commit
	created, e := r.Client.ContainerCreate(ctx, client.ContainerCreateOptions{Name: name, Config: cfg, HostConfig: &container.HostConfig{NetworkMode: "none"}})
	if e != nil {
		return "", e
	}
	defer r.removeOwned(context.WithoutCancel(ctx), created.ID, d.ApplicationID)
	if _, e = r.Client.CopyToContainer(ctx, created.ID, client.CopyToContainerOptions{DestinationPath: "/", Content: bytes.NewReader(archive)}); e != nil {
		return "", e
	}
	// Commit merges container labels; an explicit empty value clears the helper role.
	cfg.Labels["io.shipper.infrastructure"] = ""
	log("Packaging Apache image for commit " + d.Commit)
	image, e := r.Client.ContainerCommit(ctx, created.ID, client.ContainerCommitOptions{Reference: tag, Config: cfg, Comment: "Shipper static deployment " + d.ID})
	return image.ID, e
}
func (r *DockerRuntime) Start(ctx context.Context, d store.Deployment) (string, error) {
	ports := d.Config.Networking.Ports
	if d.TaskCommand != nil {
		ports = nil
	}
	if err := r.ValidatePublishedPorts(ctx, d.ApplicationID, ports); err != nil {
		return "", err
	}
	name := runtimeName(d)
	if d.TaskCommand != nil {
		name = taskName(d, store.TaskRun{ID: d.ID})
	}
	for _, existing := range []string{d.ContainerID, name, r.prefix() + "-site-" + d.ID} {
		if existing == "" {
			continue
		}
		old, e := r.Client.ContainerInspect(ctx, existing, client.ContainerInspectOptions{})
		if errdefs.IsNotFound(e) {
			continue
		}
		if e != nil {
			return "", e
		}
		if !owned(old.Container.Config.Labels, r.Instance, d.ApplicationID) || old.Container.Config.Labels["io.shipper.deployment.id"] != d.ID {
			return "", errors.New("Container ownership check failed")
		}
		if !old.Container.State.Running && d.TaskCommand == nil {
			if e = r.seedDatabaseTLS(ctx, d, old.Container.ID); e != nil {
				return "", e
			}
			_, e = r.Client.ContainerStart(ctx, old.Container.ID, client.ContainerStartOptions{})
			if e != nil {
				return "", e
			}
		}
		return old.Container.ID, nil
	}
	configuration := &container.Config{Image: d.ImageID, Labels: r.labels(d.ApplicationID, d.ID), Env: []string{"PORT=" + strconv.Itoa(ReleasePort(d))}}
	configuration.Labels["io.shipper.infrastructure"] = ""
	configuration.Labels["io.shipper.resource"] = "application"
	for _, v := range d.Config.Variables {
		if v.Scope == "runtime" || v.Scope == "both" || v.Scope == "" {
			if v.Name == "PORT" {
				configuration.Env[0] = "PORT=" + v.Value
			} else {
				configuration.Env = append(configuration.Env, v.Name+"="+v.Value)
			}
		}
	}
	if d.Config.StartCommand != "" && ReleaseMethod(d) != "static" {
		configuration.Entrypoint = []string{"/bin/sh", "-c", d.Config.StartCommand}
		configuration.Cmd = []string{}
	}
	if e := r.configureStartup(ctx, d, configuration); e != nil {
		return "", e
	}
	if d.TaskCommand != nil {
		configuration.Labels["io.shipper.infrastructure"] = "task"
		configuration.Labels["io.shipper.task.id"] = d.ID
	}
	if e := r.configureDatabaseTLS(ctx, d, configuration); e != nil {
		return "", e
	}
	endpoints, e := r.applicationEndpoints(ctx, d)
	if e != nil {
		return "", e
	}
	mounts := []mount.Mount{}
	for _, v := range d.Config.Volumes {
		volume := r.prefix() + "-" + d.ApplicationID + "-" + v.Name
		if source := d.Config.VolumeSources[v.Name]; source != "" {
			if e := r.checkVolumeSource(ctx, d.ApplicationID, source); e != nil {
				return "", e
			}
			volume = source
		}
		existing, e := r.Client.VolumeInspect(ctx, volume, client.VolumeInspectOptions{})
		if e == nil {
			if !owned(existing.Volume.Labels, r.Instance, d.ApplicationID) {
				return "", errors.New("Persistent volume ownership check failed")
			}
		} else if errdefs.IsNotFound(e) {
			labels := r.labels(d.ApplicationID, "")
			labels["io.shipper.resource"] = "data"
			if _, e = r.Client.VolumeCreate(ctx, client.VolumeCreateOptions{Name: volume, Labels: labels}); e != nil {
				return "", e
			}
		} else {
			return "", e
		}
		mounts = append(mounts, mount.Mount{Type: mount.TypeVolume, Source: volume, Target: v.Path})
	}
	memory := d.Config.MemoryMB
	if memory == 0 {
		memory = 512
		if ReleaseMethod(d) == "static" {
			memory = 128
		}
	}
	temporary := "rw,nosuid,noexec,size=16777216,mode=1777"
	if ReleaseMethod(d) != "static" {
		temporary = "rw,nosuid,exec,size=67108864,mode=1777"
	}
	restart := container.RestartPolicy{Name: "unless-stopped"}
	if d.Config.Runtime.Restart != "" {
		restart.Name = container.RestartPolicyMode(d.Config.Runtime.Restart)
	}
	cpus := int64(1e9)
	if d.Config.Runtime.CPUs > 0 {
		cpus = int64(d.Config.Runtime.CPUs * 1e9)
	}
	pids := int64(256)
	if d.Config.Runtime.PIDs > 0 {
		pids = d.Config.Runtime.PIDs
	}
	if d.TaskCommand != nil {
		restart = container.RestartPolicy{Name: "no"}
	}
	configuration.StopTimeout = ptr(StopSeconds(d.Config.Runtime))
	c, e := r.Client.ContainerCreate(ctx, client.ContainerCreateOptions{Name: name, Config: configuration, HostConfig: &container.HostConfig{PortBindings: publishedBindings(ports), Mounts: mounts, ReadonlyRootfs: ReleaseMethod(d) == "static", Tmpfs: map[string]string{"/tmp": temporary}, CapDrop: []string{"ALL"}, CapAdd: databaseCapabilities(d), SecurityOpt: []string{"no-new-privileges:true"}, RestartPolicy: restart, Resources: container.Resources{Memory: memory << 20, NanoCPUs: cpus, PidsLimit: ptr(pids)}, LogConfig: container.LogConfig{Type: "json-file", Config: map[string]string{"max-size": "5m", "max-file": "2"}}}, NetworkingConfig: &network.NetworkingConfig{EndpointsConfig: endpoints}})
	if e != nil {
		return "", e
	}
	if e = r.seedDatabaseTLS(ctx, d, c.ID); e != nil {
		return c.ID, e
	}
	if d.TaskCommand != nil {
		return c.ID, nil
	}
	_, e = r.Client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{})
	return c.ID, e
}
func (r *DockerRuntime) exec(ctx context.Context, id string, cmd []string) ([]byte, error) {
	return r.execEnv(ctx, id, cmd, nil)
}
func (r *DockerRuntime) execEnv(ctx context.Context, id string, cmd, environment []string) ([]byte, error) {
	return r.execInput(ctx, id, cmd, environment, nil)
}
func (r *DockerRuntime) execInput(ctx context.Context, id string, cmd, environment []string, input io.Reader) ([]byte, error) {
	return r.execPipe(ctx, id, cmd, environment, input, true)
}
func (r *DockerRuntime) execPipe(ctx context.Context, id string, cmd, environment []string, input io.Reader, closeInput bool) ([]byte, error) {
	e, err := r.Client.ExecCreate(ctx, id, client.ExecCreateOptions{Cmd: cmd, Env: environment, AttachStdin: input != nil, AttachStdout: true, AttachStderr: true})
	if err != nil {
		return nil, err
	}
	stream, err := r.Client.ExecAttach(ctx, e.ID, client.ExecAttachOptions{})
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	var out cappedBuffer
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			stream.Close()
		case <-done:
		}
	}()
	if input != nil {
		if _, err = io.Copy(stream.Conn, input); err != nil {
			close(done)
			return nil, err
		}
		if closeInput {
			if err = stream.CloseWrite(); err != nil {
				close(done)
				return nil, err
			}
		}
	}
	_, err = stdcopy.StdCopy(&out, &out, stream.Reader)
	close(done)
	if err != nil {
		return out.Bytes(), err
	}
	inspected, err := r.Client.ExecInspect(ctx, e.ID, client.ExecInspectOptions{})
	if err == nil && inspected.ExitCode != 0 {
		err = fmt.Errorf("Container command failed (%d): %s", inspected.ExitCode, strings.TrimSpace(out.String()))
	}
	return out.Bytes(), err
}

type cappedBuffer struct{ bytes.Buffer }

func (b *cappedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if b.Len() < 4<<20 {
		left := (4 << 20) - b.Len()
		b.Buffer.Write(p[:min(left, len(p))])
	}
	return n, nil
}
func (r *DockerRuntime) Healthy(ctx context.Context, d store.Deployment) error {
	if ReleaseMethod(d) == "compose" && d.ContainerID == "" {
		return r.ComposeHealthy(ctx, d)
	}
	startupTimeout := time.Duration(0)
	for _, h := range d.Config.Hooks {
		if h.Stage == "before_start" && time.Duration(h.Command.Timeout)*time.Second > startupTimeout {
			startupTimeout = time.Duration(h.Command.Timeout) * time.Second
		}
	}
	deadline, stop := context.WithTimeout(ctx, time.Duration(StartupSeconds(d.Config.Runtime))*time.Second+startupTimeout)
	defer stop()
	var last error
	stable := time.Now()
	for deadline.Err() == nil {
		info, e := r.Client.ContainerInspect(deadline, d.ContainerID, client.ContainerInspectOptions{})
		if e != nil {
			return e
		}
		if !owned(info.Container.Config.Labels, r.Instance, d.ApplicationID) {
			return errors.New("Candidate ownership check failed")
		}
		if info.Container.State.Running {
			if d.Config.StartupPolicy == "process" {
				waiting := false
				for _, h := range d.Config.Hooks {
					if h.Stage == "before_start" {
						top, err := r.Client.ContainerTop(deadline, d.ContainerID, client.ContainerTopOptions{Arguments: []string{"-eo", "pid,args"}})
						if err != nil {
							return err
						}
						for _, proc := range top.Processes {
							for _, field := range proc {
								if strings.Contains(field, startupPath) {
									waiting = true
								}
							}
						}
					}
				}
				if waiting {
					stable = time.Now()
					select {
					case <-deadline.Done():
					case <-time.After(250 * time.Millisecond):
					}
					continue
				}

				if info.Container.State.Health != nil {
					if string(info.Container.State.Health.Status) == "healthy" {
						return nil
					}
					last = errors.New("Image health check is not healthy")
				} else if time.Since(stable) >= 2*time.Second {
					return nil
				}
				select {
				case <-deadline.Done():
				case <-time.After(250 * time.Millisecond):
				}
				continue
			}
			healthPath := d.Config.HealthPath
			if healthPath == "" {
				healthPath = "/"
			}
			target := "site-" + d.ID
			if ReleaseMethod(d) == "compose" {
				target = composeAlias(d.ApplicationID, info.Container.Config.Labels["com.docker.compose.service"])
			}
			out, probeError := r.probeHTTP(deadline, target, ReleasePort(d), target, healthPath)
			e = probeError
			if e == nil {
				line, _, _ := strings.Cut(string(out), "\r\n")
				parts := strings.Fields(line)
				if len(parts) > 1 {
					code, _ := strconv.Atoi(parts[1])
					if code >= 200 && code < 400 && (info.Container.State.Health == nil || string(info.Container.State.Health.Status) != "unhealthy") {
						return nil
					}
					e = fmt.Errorf("Health endpoint returned HTTP %d", code)
				} else {
					e = errors.New("The health endpoint did not return an HTTP response")
				}
			}
		} else {
			stable = time.Now()
			e = errors.New("The application exited before its health check")
		}

		last = e
		select {
		case <-deadline.Done():
		case <-time.After(500 * time.Millisecond):
		}
	}
	diagnostic, cancelDiagnostic := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancelDiagnostic()
	if logs, e := r.Client.ContainerLogs(diagnostic, d.ContainerID, client.ContainerLogsOptions{ShowStderr: true, Tail: "4"}); e == nil {
		defer logs.Close()
		var output strings.Builder
		writer := &logWriter{emit: func(line string) error {
			if output.Len() < 4096 {
				output.WriteString(RedactDeployment(d, line) + "\n")
			}
			return nil
		}}
		// Emit only complete lines so a read limit cannot expose part of a secret.
		stdcopy.StdCopy(writer, writer, io.LimitReader(logs, 256<<10))
		text := strings.TrimSpace(output.String())
		if text != "" {
			return fmt.Errorf("The candidate did not pass its HTTP health check: %w\n%s", last, text[:min(len(text), 2048)])
		}
	}
	return fmt.Errorf("The candidate did not pass its HTTP health check: %w", last)
}

// Routing references only the committed active release. All names and hosts have
// been validated before interpolation; repository data never becomes proxy config.
func Routes(apps []store.Application, active map[string]store.Deployment) map[string]any {
	routers := map[string]any{}
	services := map[string]any{}
	middlewares := map[string]any{"https-redirect": map[string]any{"redirectScheme": map[string]any{"scheme": "https", "permanent": true}}}
	for _, a := range apps {
		d, ok := active[a.ID]
		if !ok || d.ID == "" {
			continue
		}
		domains := a.Domains
		if domains == nil && d.Config.Domain != "" {
			domains = []store.Domain{{ID: "primary", Hostname: d.Config.Domain, HTTPS: d.Config.HTTPS, Primary: true}}
		}
		if len(domains) == 0 {
			continue
		}
		key := "app-" + a.ID
		if ReleaseMethod(d) != "compose" {
			services[key] = map[string]any{"loadBalancer": map[string]any{"servers": []any{map[string]string{"url": "http://site-" + d.ID + ":" + strconv.Itoa(ReleasePort(d))}}}}
		}
		domains = append([]store.Domain(nil), domains...)
		for i := range domains {
			domains[i].VerificationState = ""
			domains[i].Message = ""
			domains[i].CheckedAt = 0
			domains[i].CertificateExpiresAt = 0
			domains[i].CertificateIssuer = ""
		}
		payload, _ := json.Marshal(struct {
			Domains []store.Domain
			Release string
			Port    int
		}{domains, d.ID, ReleasePort(d)})
		revision := fmt.Sprintf("%x", sha256.Sum256(payload))
		identity := "identity-" + a.ID + "-" + revision[:12]
		middlewares[identity] = map[string]any{"headers": map[string]any{"customResponseHeaders": map[string]string{"X-Shipper-Deployment": d.ID}}}
		for _, domain := range domains {
			serviceKey := key
			group := []store.Domain{}
			for _, other := range domains {
				if other.Service == domain.Service {
					group = append(group, other)
				}
			}
			primary := store.PrimaryDomain(group)
			if ReleaseMethod(d) == "compose" {
				port := 0
				for _, svc := range d.Services {
					if svc.Name == domain.Service {
						port = svc.Port
					}
				}
				if port == 0 {
					continue
				}
				serviceKey = key + "-svc-" + domain.Service
				services[serviceKey] = map[string]any{"loadBalancer": map[string]any{"servers": []any{map[string]string{"url": "http://" + composeAlias(a.ID, domain.Service) + ":" + strconv.Itoa(port)}}}}
			}
			routerKey := serviceKey
			if !domain.Primary {
				routerKey += "-" + domain.ID
			}
			rule := "Host(`" + domain.Hostname + "`)"
			if domain.PathPrefix != "" && domain.PathPrefix != "/" {
				rule += " && PathPrefix(`" + domain.PathPrefix + "`)"
			}
			if domain.Maintenance {
				serviceKey = "maintenance-" + a.ID
				services[serviceKey] = map[string]any{"loadBalancer": map[string]any{"servers": []any{}}}
			}
			chain := []string{identity}
			policy := "route-" + routerKey + "-" + revision[:12]
			if domain.StripPrefix {
				middlewares[policy+"-strip"] = map[string]any{"stripPrefix": map[string]any{"prefixes": []string{domain.PathPrefix}}}
				chain = append(chain, policy+"-strip")
			}
			if domain.Compress {
				middlewares[policy+"-compress"] = map[string]any{"compress": map[string]any{}}
				chain = append(chain, policy+"-compress")
			}
			if len(domain.ResponseHeaders) > 0 {
				middlewares[policy+"-headers"] = map[string]any{"headers": map[string]any{"customResponseHeaders": domain.ResponseHeaders}}
				chain = append(chain, policy+"-headers")
			}
			if domain.Redirect && !domain.Primary && primary.Hostname != "" {
				redirect := "redirect-" + routerKey + "-" + revision[:12]
				scheme := "http"
				if primary.HTTPS {
					scheme = "https"
				}
				middlewares[redirect] = map[string]any{"redirectRegex": map[string]any{"regex": "^https?://" + regexp.QuoteMeta(domain.Hostname) + "(?::[0-9]+)?(.*)", "replacement": scheme + "://" + primary.Hostname + "${1}", "permanent": true}}
				chain = append([]string{identity, redirect}, chain[1:]...)
			}
			router := map[string]any{"rule": rule, "service": serviceKey, "entryPoints": []string{"web"}, "middlewares": chain}
			if domain.HTTPS {
				httpChain := append([]string{}, chain...)
				if !domain.Redirect {
					httpChain = append(httpChain, "https-redirect")
				}
				routers[routerKey+"-http"] = map[string]any{"rule": rule, "service": serviceKey, "entryPoints": []string{"web"}, "middlewares": httpChain}
				router["entryPoints"] = []string{"websecure"}
				router["tls"] = map[string]any{"certResolver": "letsencrypt"}
			}
			routers[routerKey] = router
		}
	}
	httpConfig := map[string]any{"middlewares": middlewares}
	if len(routers) > 0 {
		httpConfig["routers"] = routers
	}
	if len(services) > 0 {
		httpConfig["services"] = services
	}
	return map[string]any{"http": httpConfig}
}

func (r *DockerRuntime) Routes(ctx context.Context, apps []store.Application, active map[string]store.Deployment) error {
	if err := r.reconcileApplicationAliases(ctx, active); err != nil {
		return err
	}
	config, e := r.EffectiveProxy(ctx)
	if e != nil {
		return e
	}
	resolved := *r
	resolved.Config = config
	configuration := Routes(apps, active)
	resolved.dashboardRoute(configuration)
	data, e := json.Marshal(configuration)
	if e != nil {
		return e
	}
	if e = r.copy(ctx, r.proxy(), "/data/dynamic", map[string][]byte{"routes.next": data}); e != nil {
		return e
	}
	// JSON is a YAML subset; the file provider discovers .yaml/.yml/.toml files.
	if _, e = r.exec(ctx, r.proxy(), []string{"mv", "/data/dynamic/routes.next", "/data/dynamic/routes.yaml"}); e != nil {
		return e
	}
	// Wait for Traefik to acknowledge every new backend before retiring a release.
	until, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var lastOutput string
	for until.Err() == nil {
		out, err := r.exec(until, r.proxy(), []string{"wget", "-q", "-O", "-", "http://127.0.0.1:9000/api/http/services"})
		if until.Err() == nil {
			lastOutput = string(out)
			if err != nil {
				lastOutput = err.Error()
			}
		}
		if err == nil {
			var services []struct {
				Name         string `json:"name"`
				Status       string `json:"status"`
				LoadBalancer struct {
					Servers []struct {
						URL string `json:"url"`
					} `json:"servers"`
				} `json:"loadBalancer"`
			}
			if json.Unmarshal(out, &services) == nil {
				found := map[string]string{}
				enabled := map[string]bool{}
				for _, s := range services {
					enabled[s.Name] = s.Status == "enabled"
					if s.Status == "enabled" && len(s.LoadBalancer.Servers) > 0 {
						found[s.Name] = s.LoadBalancer.Servers[0].URL
					}
				}
				complete := true
				expectedServices, _ := configuration["http"].(map[string]any)["services"].(map[string]any)
				for name := range found {
					if strings.HasPrefix(name, "app-") && strings.HasSuffix(name, "@file") {
						if _, ok := expectedServices[strings.TrimSuffix(name, "@file")]; !ok {
							complete = false
						}
					}
				}
				for name, value := range expectedServices {
					servers := value.(map[string]any)["loadBalancer"].(map[string]any)["servers"].([]any)
					if len(servers) == 0 {
						if !enabled[name+"@file"] {
							complete = false
						}
						continue
					}
					if found[name+"@file"] != servers[0].(map[string]string)["url"] {
						complete = false
					}
				}
				if complete {
					raw, routerError := r.exec(until, r.proxy(), []string{"wget", "-q", "-O", "-", "http://127.0.0.1:9000/api/http/routers"})
					var routers []struct {
						Name        string   `json:"name"`
						Status      string   `json:"status"`
						Rule        string   `json:"rule"`
						Middlewares []string `json:"middlewares"`
					}
					if routerError == nil && json.Unmarshal(raw, &routers) == nil {
						expected, _ := configuration["http"].(map[string]any)["routers"].(map[string]any)
						seen := map[string]bool{}
						for _, router := range routers {
							if !strings.HasSuffix(router.Name, "@file") || !(strings.HasPrefix(router.Name, "app-") || strings.HasPrefix(router.Name, "shipper-dashboard")) {
								continue
							}
							name := strings.TrimSuffix(router.Name, "@file")
							value, ok := expected[name]
							if !ok {
								complete = false
								continue
							}
							want := value.(map[string]any)
							if router.Status != "enabled" || router.Rule != want["rule"].(string) {
								complete = false
								continue
							}
							for _, middleware := range want["middlewares"].([]string) {
								match := false
								for _, actual := range router.Middlewares {
									if actual == middleware+"@file" {
										match = true
									}
								}
								if !match {
									complete = false
								}
							}
							seen[name] = true
						}
						for name := range expected {
							if !seen[name] {
								complete = false
							}
						}

						if complete {
							return nil
						}
					}
				}
			}
		}
		select {
		case <-until.Done():
		case <-time.After(200 * time.Millisecond):
		}
	}
	return fmt.Errorf("Traefik did not acknowledge the routing update: %s", lastOutput[:min(len(lastOutput), 512)])
}
func (r *DockerRuntime) removeOwned(ctx context.Context, id, app string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	info, e := r.Client.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if errdefs.IsNotFound(e) {
		return nil
	}
	if e != nil {
		return e
	}
	if !owned(info.Container.Config.Labels, r.Instance, app) {
		return errors.New("Container ownership check failed; nothing was removed")
	}
	_, e = r.Client.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: true, RemoveVolumes: false})
	return e
}
func (r *DockerRuntime) RemoveDeployment(ctx context.Context, d store.Deployment) error {
	if ReleaseMethod(d) == "compose" {
		return nil
	}
	items, e := r.Client.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: make(client.Filters).Add("label", "io.shipper.instance="+r.Instance).Add("label", "io.shipper.application.id="+d.ApplicationID).Add("label", "io.shipper.deployment.id="+d.ID)})
	if e != nil {
		return e
	}
	for _, item := range items.Items {
		if _, e := r.Client.ContainerStop(ctx, item.ID, client.ContainerStopOptions{Timeout: ptr(StopSeconds(d.Config.Runtime))}); e != nil && !errdefs.IsNotFound(e) {
			return e
		}
		if e := r.removeOwned(ctx, item.ID, d.ApplicationID); e != nil {
			return e
		}
	}
	return nil
}
func (r *DockerRuntime) RemoveApplication(ctx context.Context, app string) error {
	items, e := r.Client.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: make(client.Filters).Add("label", "io.shipper.instance="+r.Instance).Add("label", "io.shipper.application.id="+app)})
	if e != nil {
		return e
	}
	for _, c := range items.Items {
		if e = r.removeOwned(ctx, c.ID, app); e != nil {
			return e
		}
	}
	return nil
}

func (r *DockerRuntime) probeHTTP(ctx context.Context, target string, port int, host, requestPath string) ([]byte, error) {
	request := "GET " + requestPath + " HTTP/1.1\r\nHost: " + host + "\r\nConnection: close\r\n\r\n"
	return r.execPipe(ctx, r.proxy(), []string{"nc", "-w", "3", target, strconv.Itoa(port)}, nil, strings.NewReader(request), false)
}
func (r *DockerRuntime) ProbeLocal(ctx context.Context, d store.Deployment) (bool, error) {
	requestPath := "/"
	for _, domain := range d.Config.Domains {
		if domain.Hostname == d.Config.Domain && domain.PathPrefix != "" {
			requestPath = domain.PathPrefix + "/"
		}
	}
	out, e := r.probeHTTP(ctx, "127.0.0.1", 8080, d.Config.Domain, requestPath)
	if e != nil {
		return false, e
	}
	header, _, _ := strings.Cut(string(out), "\r\n\r\n")
	lines := strings.Split(header, "\r\n")
	if len(lines) == 0 {
		return false, nil
	}
	parts := strings.Fields(lines[0])
	if len(parts) < 2 {
		return false, nil
	}
	code, _ := strconv.Atoi(parts[1])
	maintenance := false
	for _, domain := range d.Config.Domains {
		if domain.Hostname == d.Config.Domain {
			maintenance = domain.Maintenance
		}
	}
	if code != 200 && (code < 300 || code > 399) && !(maintenance && code == 503) {
		return false, nil
	}
	for _, line := range lines[1:] {
		key, value, ok := strings.Cut(line, ":")
		if ok && strings.EqualFold(key, "X-Shipper-Deployment") {
			return strings.TrimSpace(value) == d.ID, nil
		}
	}
	return false, nil
}

var _ io.Writer = (*cappedBuffer)(nil)
