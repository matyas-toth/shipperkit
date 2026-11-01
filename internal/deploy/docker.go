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
	BindIP    string `json:"bindIP"`
	HTTPPort  int    `json:"httpPort"`
	HTTPSPort int    `json:"httpsPort"`
	ACMEEmail string `json:"acmeEmail"`
	ACMECA    string `json:"acmeCA"`
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
	Client   *client.Client
	Instance string
	Config   ProxyConfig
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
			return errors.New("Proxy configuration changed; recreate the Shipper proxy container while retaining its data volume, then redeploy")
		}
		if !inspected.Container.State.Running {
			_, e = r.Client.ContainerStart(ctx, r.proxy(), client.ContainerStartOptions{})
		}
		return e
	}
	if !errdefs.IsNotFound(e) {
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
	initial, _ := json.Marshal(Routes(nil, nil))
	if e = r.copy(ctx, created.ID, "/data", map[string][]byte{"./": nil, "dynamic/": nil, "dynamic/routes.yaml": initial}); e != nil {
		return e
	}
	_, e = r.Client.ContainerStart(ctx, created.ID, client.ContainerStartOptions{})
	return e
}
func ptr[T any](v T) *T { return &v }
func (r *DockerRuntime) configHash() string {
	data, _ := json.Marshal(r.Config)
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
	delete(cfg.Labels, "io.shipper.infrastructure")
	log("Packaging Apache image for commit " + d.Commit)
	image, e := r.Client.ContainerCommit(ctx, created.ID, client.ContainerCommitOptions{Reference: tag, Config: cfg, Comment: "Shipper static deployment " + d.ID})
	return image.ID, e
}
func (r *DockerRuntime) Start(ctx context.Context, d store.Deployment) (string, error) {
	name := r.prefix() + "-site-" + d.ID
	if old, e := r.Client.ContainerInspect(ctx, name, client.ContainerInspectOptions{}); e == nil {
		if !owned(old.Container.Config.Labels, r.Instance, d.ApplicationID) {
			return "", errors.New("Container ownership check failed")
		}
		if !old.Container.State.Running {
			_, e = r.Client.ContainerStart(ctx, name, client.ContainerStartOptions{})
			if e != nil {
				return "", e
			}
		}
		return old.Container.ID, nil
	}
	c, e := r.Client.ContainerCreate(ctx, client.ContainerCreateOptions{Name: name, Config: &container.Config{Image: d.ImageID, Labels: r.labels(d.ApplicationID, d.ID)}, HostConfig: &container.HostConfig{ReadonlyRootfs: true, Tmpfs: map[string]string{"/tmp": "rw,nosuid,noexec,size=16777216,mode=1777"}, CapDrop: []string{"ALL"}, SecurityOpt: []string{"no-new-privileges:true"}, RestartPolicy: container.RestartPolicy{Name: "unless-stopped"}, Resources: container.Resources{Memory: 128 << 20, NanoCPUs: 1e9, PidsLimit: ptr(int64(128))}, LogConfig: container.LogConfig{Type: "json-file", Config: map[string]string{"max-size": "5m", "max-file": "2"}}}, NetworkingConfig: &network.NetworkingConfig{EndpointsConfig: map[string]*network.EndpointSettings{r.network(): {Aliases: []string{"site-" + d.ID}}}}})
	if e != nil {
		return "", e
	}
	_, e = r.Client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{})
	return c.ID, e
}
func (r *DockerRuntime) exec(ctx context.Context, id string, cmd []string) ([]byte, error) {
	e, err := r.Client.ExecCreate(ctx, id, client.ExecCreateOptions{Cmd: cmd, AttachStdout: true, AttachStderr: true})
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
	deadline, stop := context.WithTimeout(ctx, 45*time.Second)
	defer stop()
	var last error
	for deadline.Err() == nil {
		info, e := r.Client.ContainerInspect(deadline, d.ContainerID, client.ContainerInspectOptions{})
		if e != nil {
			return e
		}
		if !owned(info.Container.Config.Labels, r.Instance, d.ApplicationID) {
			return errors.New("Candidate ownership check failed")
		}
		if info.Container.State.Running {
			_, e = r.exec(deadline, d.ContainerID, []string{"wget", "-q", "-T", "2", "-O", "/dev/null", "http://127.0.0.1:8080/"})
			if e == nil {
				return nil
			}
		} else {
			e = errors.New("Apache exited before its health check")
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
		var output cappedBuffer
		stdcopy.StdCopy(&output, &output, io.LimitReader(logs, 64<<10))
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
		if !ok || d.ID == "" || d.Config.Domain == "" {
			continue
		}
		rule := "Host(`" + d.Config.Domain + "`)"
		key := "app-" + a.ID
		services[key] = map[string]any{"loadBalancer": map[string]any{"servers": []any{map[string]string{"url": "http://site-" + d.ID + ":8080"}}}}
		router := map[string]any{"rule": rule, "service": key, "entryPoints": []string{"web"}}
		if d.Config.HTTPS {
			routers[key+"-http"] = map[string]any{"rule": rule, "service": key, "entryPoints": []string{"web"}, "middlewares": []string{"https-redirect"}}
			router["entryPoints"] = []string{"websecure"}
			router["tls"] = map[string]any{"certResolver": "letsencrypt"}
		}
		routers[key] = router
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
	data, e := json.Marshal(Routes(apps, active))
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
				for _, s := range services {
					if s.Status == "enabled" && len(s.LoadBalancer.Servers) > 0 {
						found[s.Name] = s.LoadBalancer.Servers[0].URL
					}
				}
				complete := true
				for name := range found {
					if strings.HasPrefix(name, "app-") && strings.HasSuffix(name, "@file") {
						id := strings.TrimSuffix(strings.TrimPrefix(name, "app-"), "@file")
						if _, ok := active[id]; !ok {
							complete = false
						}
					}
				}
				for id, d := range active {
					if found["app-"+id+"@file"] != "http://site-"+d.ID+":8080" {
						complete = false
						break
					}
				}
				if complete {
					raw, routerError := r.exec(until, r.proxy(), []string{"wget", "-q", "-O", "-", "http://127.0.0.1:9000/api/http/routers"})
					var routers []struct {
						Name   string `json:"name"`
						Status string `json:"status"`
					}
					if routerError == nil && json.Unmarshal(raw, &routers) == nil {
						enabled := map[string]bool{}
						for _, router := range routers {
							enabled[router.Name] = router.Status == "enabled"
						}
						for id, d := range active {
							if !enabled["app-"+id+"@file"] || (d.Config.HTTPS && !enabled["app-"+id+"-http@file"]) {
								complete = false
								break
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
	for _, name := range []string{r.prefix() + "-site-" + d.ID, r.prefix() + "-package-" + d.ID} {
		if e := r.removeOwned(ctx, name, d.ApplicationID); e != nil {
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

func (r *DockerRuntime) ProbeLocal(ctx context.Context, d store.Deployment) (bool, error) {
	out, e := r.exec(ctx, r.proxy(), []string{"wget", "-q", "-S", "-T", "4", "-O", "/dev/null", "--header", "Host: " + d.Config.Domain, "http://127.0.0.1:8080/"})
	if e != nil {
		return false, e
	}
	for _, line := range strings.Split(string(out), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if ok && strings.EqualFold(key, "X-Shipper-Deployment") {
			return strings.TrimSpace(value) == d.ID, nil
		}
	}
	return false, nil
}

var _ io.Writer = (*cappedBuffer)(nil)
