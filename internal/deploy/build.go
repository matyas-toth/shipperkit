package deploy

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
	"shipper/internal/store"
)

const BuildkitImage = "moby/buildkit:v0.34.1@sha256:99368f3e46177c6405b9740e4461d91cee004128daeecca78d4ceac50ec1da5c"
const RailpackVersion = "0.43.0"
const RailpackFrontend = "ghcr.io/railwayapp/railpack-frontend:v0.43.0@sha256:355ef61fae6f2551103cd8b10c710cb07167dd353d561f15c69b8d7ac1c676cc"

const StaticBuildImage = "node:24-alpine@sha256:8a32c8cf649a49915a66e81e7a609223c708d0adabf288ee3f4077d765542f85"

//go:embed static_publish.js
var staticPublishScript string

type Builder interface {
	Build(context.Context, store.Deployment, string, func(string)) (string, error)
}

func (r *DockerRuntime) builderName() string { return r.prefix() + "-builder" }
func (r *DockerRuntime) clientConfig() (string, error) {
	root := r.DataDir
	if root == "" {
		root = ".shipper"
	}
	dir, e := filepath.Abs(filepath.Join(root, "docker-client"))
	if e != nil {
		return "", e
	}
	e = os.MkdirAll(dir, 0700)
	return dir, e
}
func (r *DockerRuntime) dockerEnvironment() []string {
	allowed := map[string]bool{"PATH": true, "SYSTEMROOT": true, "WINDIR": true, "TEMP": true, "TMP": true, "HOME": true, "XDG_RUNTIME_DIR": true, "DOCKER_TLS_VERIFY": true, "DOCKER_CERT_PATH": true, "PROGRAMFILES": true, "PROGRAMFILES(X86)": true, "PROGRAMDATA": true, "USERPROFILE": true, "COMSPEC": true, "HOMEDRIVE": true, "HOMEPATH": true, "LOCALAPPDATA": true}
	out := []string{}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if allowed[strings.ToUpper(key)] {
			out = append(out, entry)
		}
	}
	return append(out, "DOCKER_HOST="+r.Client.DaemonHost(), "BUILDX_NO_DEFAULT_ATTESTATIONS=1")
}
func (r *DockerRuntime) dockerCommand(ctx context.Context, arguments, environment []string, log func(string)) error {
	config, e := r.clientConfig()
	if e != nil {
		return e
	}
	args := append([]string{"--config", config}, arguments...)
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Env = append(append(r.dockerEnvironment(), "DOCKER_CONFIG="+config, "BUILDX_CONFIG="+filepath.Join(config, "buildx")), environment...)
	cmd.WaitDelay = 3 * time.Second
	stream := &gitLogWriter{log: log, Limit: 10000}
	cmd.Stdout = stream
	cmd.Stderr = cmd.Stdout
	e = cmd.Run()
	stream.flush()
	if e != nil {
		return fmt.Errorf("Build client failed: %w; check the build output above", e)
	}
	return nil
}
func (r *DockerRuntime) EnsureBuilder(ctx context.Context, log func(string)) error {
	name := r.builderName()
	cache := name + "-cache"
	labels := r.labels("", "")
	labels["io.shipper.infrastructure"] = "builder"
	if v, e := r.Client.VolumeInspect(ctx, cache, client.VolumeInspectOptions{}); e == nil {
		if !owned(v.Volume.Labels, r.Instance, "") {
			return errors.New("Builder cache ownership check failed")
		}
	} else if errdefs.IsNotFound(e) {
		if _, e = r.Client.VolumeCreate(ctx, client.VolumeCreateOptions{Name: cache, Labels: labels}); e != nil {
			return e
		}
	} else {
		return e
	}
	info, e := r.Client.ContainerInspect(ctx, name, client.ContainerInspectOptions{})
	if e == nil {
		if !owned(info.Container.Config.Labels, r.Instance, "") || info.Container.Config.Labels["io.shipper.infrastructure"] != "builder" {
			return errors.New("Builder ownership check failed")
		}
		if !info.Container.State.Running {
			if _, e = r.Client.ContainerStart(ctx, name, client.ContainerStartOptions{}); e != nil {
				return e
			}
		}
	} else if errdefs.IsNotFound(e) {
		if e = r.pull(ctx, BuildkitImage, log); e != nil {
			return e
		}
		created, e := r.Client.ContainerCreate(ctx, client.ContainerCreateOptions{Name: name, Config: &container.Config{Image: BuildkitImage, Labels: labels, Cmd: []string{"--config", "/etc/buildkit/shipper.toml"}}, HostConfig: &container.HostConfig{Privileged: true, Mounts: []mount.Mount{{Type: mount.TypeVolume, Source: cache, Target: "/var/lib/buildkit"}}, Resources: container.Resources{Memory: 2 << 30, NanoCPUs: 2e9, PidsLimit: ptr(int64(1024))}, RestartPolicy: container.RestartPolicy{Name: "unless-stopped"}, LogConfig: container.LogConfig{Type: "json-file", Config: map[string]string{"max-size": "5m", "max-file": "2"}}}})
		if e != nil {
			return e
		}
		conf := []byte("[worker.oci]\n  gc = true\n  reservedSpace = \"256MB\"\n  maxUsedSpace = \"4GB\"\n  minFreeSpace = \"1GB\"\n")
		var b bytes.Buffer
		tw := tar.NewWriter(&b)
		tw.WriteHeader(&tar.Header{Name: "shipper.toml", Mode: 0644, Size: int64(len(conf))})
		tw.Write(conf)
		tw.Close()
		if _, e = r.Client.CopyToContainer(ctx, created.ID, client.CopyToContainerOptions{DestinationPath: "/etc/buildkit", Content: &b}); e != nil {
			return e
		}
		if _, e = r.Client.ContainerStart(ctx, name, client.ContainerStartOptions{}); e != nil {
			return e
		}
	} else {
		return e
	}
	config, e := r.clientConfig()
	if e != nil {
		return e
	}
	if _, e = os.Stat(filepath.Join(config, "buildx", "instances", name)); os.IsNotExist(e) {
		if e = r.dockerCommand(ctx, []string{"buildx", "create", "--name", name, "--driver", "remote", "docker-container://" + name}, nil, log); e != nil {
			return e
		}
	}
	return r.dockerCommand(ctx, []string{"buildx", "inspect", name, "--bootstrap"}, nil, func(string) {})
}

func copyContext(source, destination string) error {
	var size int64
	count := 0
	return filepath.WalkDir(source, func(file string, entry fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if file == source {
			return nil
		}
		name := strings.ToLower(entry.Name())
		blocked := name == ".git" || name == ".ssh" || name == "node_modules" || strings.HasPrefix(name, ".env") || strings.HasSuffix(name, ".pem") || strings.HasSuffix(name, ".key") || name == "id_rsa"
		if blocked {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, e := filepath.Rel(source, file)
		if e != nil {
			return e
		}
		target := filepath.Join(destination, rel)
		if !strings.HasPrefix(target, destination+string(filepath.Separator)) {
			return errors.New("Build context escapes its root")
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("Build context links must be replaced with regular files: %s", rel)
		}
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		info, e := entry.Info()
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("Unsupported build context file: %s", rel)
		}
		size += info.Size()
		count++
		if size > 512<<20 || count > 30000 {
			return errors.New("Build contexts are limited to 512 MiB and 30,000 files")
		}
		in, e := os.Open(file)
		if e != nil {
			return e
		}
		defer in.Close()
		out, e := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm()&0755)
		if e != nil {
			return e
		}
		_, copyError := io.Copy(out, in)
		closeError := out.Close()
		if copyError != nil {
			return copyError
		}
		return closeError
	})
}
func (r *DockerRuntime) Build(ctx context.Context, d store.Deployment, repository string, log func(string)) (string, error) {
	method := ReleaseMethod(d)
	if method == "static" && d.Config.BuildCommand == "" {
		archive, e := StaticArchive(repository, d.Config, d.ID)
		if e != nil {
			return "", e
		}
		return r.Package(ctx, d, archive, log)
	}
	if e := r.EnsureBuilder(ctx, log); e != nil {
		return "", e
	}
	tag := "shipper/" + r.Instance[:12] + "/" + d.ApplicationID + ":" + d.ID
	if image, e := r.Client.ImageInspect(ctx, tag); e == nil {
		if !owned(image.Config.Labels, r.Instance, d.ApplicationID) {
			return "", errors.New("Built image ownership check failed")
		}
		return image.ID, nil
	}
	work, e := os.MkdirTemp(filepath.Dir(repository), "build-")
	if e != nil {
		return "", e
	}
	defer os.RemoveAll(work)
	contextDir := filepath.Join(work, "context")
	if e = os.Mkdir(contextDir, 0700); e != nil {
		return "", e
	}
	if e = copyContext(repository, contextDir); e != nil {
		return "", e
	}
	dockerfile := filepath.Join(contextDir, filepath.FromSlash(d.Config.Dockerfile))
	args := []string{"buildx", "build", "--builder", r.builderName(), "--load", "--progress=plain", "--tag", tag}
	environment := []string{}
	if d.NoCache {
		args = append(args, "--no-cache")
	}
	for key, value := range r.labels(d.ApplicationID, d.ID) {
		args = append(args, "--label", key+"="+value)
	}
	args = append(args, "--label", "org.opencontainers.image.revision="+d.Commit)
	for index, v := range d.Config.Variables {
		if v.Scope == "runtime" || v.Scope == "" {
			continue
		}
		if v.Secret || method == "railpack" {
			key := fmt.Sprintf("SHIPPER_BUILD_VALUE_%d", index)
			environment = append(environment, key+"="+v.Value)
			args = append(args, "--secret", "id="+v.Name+",env="+key)
		} else {
			args = append(args, "--build-arg", v.Name+"="+v.Value)
		}
	}
	switch method {
	case "railpack":
		log("Preparing Railpack " + RailpackVersion)
		plan, e := r.prepareRailpack(ctx, d, contextDir, log)
		if e != nil {
			return "", e
		}
		dockerfile = filepath.Join(work, "railpack-plan.json")
		if e = os.WriteFile(dockerfile, plan, 0600); e != nil {
			return "", e
		}
		args = append(args, "--build-arg", "BUILDKIT_SYNTAX="+RailpackFrontend, "--build-arg", "cache-key="+r.Instance+"-"+d.ApplicationID)
		var secrets strings.Builder
		for _, v := range d.Config.Variables {
			if v.Scope == "build" || v.Scope == "both" {
				secrets.WriteString(v.Name + "=" + v.Value + "\n")
			}
		}
		sum := sha256.Sum256([]byte(secrets.String()))
		args = append(args, "--build-arg", fmt.Sprintf("secrets-hash=%x", sum))
	case "static":
		dockerfile = filepath.Join(work, "static.Dockerfile")
		data, e := staticBuildfile(d)
		if e != nil {
			return "", e
		}
		if e = os.WriteFile(filepath.Join(contextDir, ".shipper-httpd.conf"), []byte(ApacheConfig(d.Config)+"\nHeader always set X-Shipper-Deployment \""+d.ID+"\"\n"), 0644); e != nil {
			return "", e
		}
		if e = os.WriteFile(dockerfile, []byte(data), 0600); e != nil {
			return "", e
		}
	case "dockerfile":
		if d.Config.InstallCommand != "" || d.Config.BuildCommand != "" {
			return "", errors.New("Set install and build commands in the Dockerfile, or select another deployment method")
		}
		info, e := os.Stat(dockerfile)
		if e != nil || !info.Mode().IsRegular() {
			return "", errors.New("Dockerfile does not exist in the application directory")
		}
	default:
		return "", errors.New("Unsupported build method")
	}
	args = append(args, "--file", dockerfile, contextDir)
	log("Building " + method + " image")
	if e = r.dockerCommand(ctx, args, environment, log); e != nil {
		return "", e
	}
	image, e := r.Client.ImageInspect(ctx, tag)
	if e != nil {
		return "", e
	}
	// The custom Railpack frontend does not apply Dockerfile --label options.
	// Commit metadata from an unstarted, isolated container while preserving the
	// generated image's complete runtime configuration and filesystem layers.
	if method == "railpack" {
		encoded, err := json.Marshal(image.Config)
		if err != nil {
			return "", err
		}
		var config container.Config
		if err = json.Unmarshal(encoded, &config); err != nil {
			return "", err
		}
		config.Image = image.ID
		if config.Labels == nil {
			config.Labels = map[string]string{}
		}
		for key, value := range r.labels(d.ApplicationID, d.ID) {
			config.Labels[key] = value
		}
		config.Labels["org.opencontainers.image.revision"] = d.Commit
		metadata, err := r.Client.ContainerCreate(ctx, client.ContainerCreateOptions{Name: r.prefix() + "-metadata-" + d.ID, Config: &config, HostConfig: &container.HostConfig{NetworkMode: "none"}})
		if err != nil {
			return "", err
		}
		defer r.removeOwned(context.WithoutCancel(ctx), metadata.ID, d.ApplicationID)
		committed, err := r.Client.ContainerCommit(ctx, metadata.ID, client.ContainerCommitOptions{Reference: tag, Config: &config, Comment: "Shipper Railpack image metadata"})
		if err != nil {
			return "", err
		}
		image, e = r.Client.ImageInspect(ctx, committed.ID)
		if e != nil {
			return "", e
		}
	}
	if !owned(image.Config.Labels, r.Instance, d.ApplicationID) {
		return "", errors.New("Built image ownership check failed")
	}
	return image.ID, nil
}
func staticBuildfile(d store.Deployment) (string, error) {
	install := d.Config.InstallCommand
	if install == "" {
		install = "if [ -f package-lock.json ]; then npm ci; else npm install; fi"
	}
	filterJSON, _ := json.Marshal([]string{"node", "-e", staticPublishScript, "/app/" + d.Config.PublishDirectory, fmt.Sprint(d.Config.HTAccess)})
	secretMounts := ""
	declarations := ""
	for _, v := range d.Config.Variables {
		if v.Scope == "build" || v.Scope == "both" {
			if v.Secret {
				secretMounts += "--mount=type=secret,id=" + v.Name + ",env=" + v.Name + " "
			} else {
				declarations += "ARG " + v.Name + "\nENV " + v.Name + "=$" + v.Name + "\n"
			}
		}
	}
	installJSON, _ := json.Marshal([]string{"/bin/sh", "-c", install})
	buildJSON, _ := json.Marshal([]string{"/bin/sh", "-c", d.Config.BuildCommand})
	return "# syntax=docker/dockerfile:1\nFROM " + StaticBuildImage + " AS build\nWORKDIR /app\n" + declarations + "COPY . .\nRUN " + secretMounts + string(installJSON) + "\nRUN " + secretMounts + string(buildJSON) + "\nRUN " + string(filterJSON) + "\nFROM " + ApacheImage + "\nCOPY --from=build /shipper-output/ /usr/local/apache2/htdocs/\nCOPY .shipper-httpd.conf /usr/local/apache2/conf/httpd.conf\nUSER 65532:65532\nCMD [\"httpd\",\"-D\",\"FOREGROUND\"]\n", nil
}

// The prepare tool has bounded writable tmpfs, no host mounts, no socket, and no
// Linux capabilities. Only its generated plan is copied back into the builder.
func (r *DockerRuntime) prepareRailpack(ctx context.Context, d store.Deployment, source string, log func(string)) ([]byte, error) {
	info, e := r.Client.Info(ctx, client.InfoOptions{})
	if e != nil {
		return nil, e
	}
	arch := "x86_64"
	checksum := "248a8f60f36854aabed6124113631078a802de3461660855fc4de9c26065c1fb"
	if info.Info.Architecture == "aarch64" || info.Info.Architecture == "arm64" {
		arch = "arm64"
		checksum = "732ece14eb4f4676a43ebba19431f954edf7b130acb76159ef0f2ff86ac42ac9"
	}
	binary, e := r.railpackBinary(ctx, arch, checksum)
	if e != nil {
		return nil, e
	}
	if e = r.pull(ctx, BuildkitImage, log); e != nil {
		return nil, e
	}
	name := r.prefix() + "-prepare-" + d.ID
	labels := r.labels(d.ApplicationID, d.ID)
	labels["io.shipper.infrastructure"] = "prepare"
	if old, e := r.Client.ContainerInspect(ctx, name, client.ContainerInspectOptions{}); e == nil {
		if !owned(old.Container.Config.Labels, r.Instance, d.ApplicationID) {
			return nil, errors.New("Prepare container ownership check failed")
		}
		if e = r.removeOwned(ctx, name, d.ApplicationID); e != nil {
			return nil, e
		}
	}
	created, e := r.Client.ContainerCreate(ctx, client.ContainerCreateOptions{Name: name, Config: &container.Config{Image: BuildkitImage, Entrypoint: []string{"/bin/sh", "-c", "sleep 1800"}, User: "65532:65532", Env: []string{"HOME=/tmp", "XDG_CACHE_HOME=/tmp/cache"}, Labels: labels}, HostConfig: &container.HostConfig{ReadonlyRootfs: true, CapDrop: []string{"ALL"}, SecurityOpt: []string{"no-new-privileges:true"}, Tmpfs: map[string]string{"/work": "rw,nosuid,exec,size=536870912,mode=1777", "/tmp": "rw,nosuid,exec,size=536870912,mode=1777"}, Resources: container.Resources{Memory: 1536 << 20, NanoCPUs: 1e9, PidsLimit: ptr(int64(128))}}})
	if e != nil {
		return nil, e
	}
	defer r.removeOwned(context.WithoutCancel(ctx), created.ID, d.ApplicationID)
	if _, e = r.Client.ContainerStart(ctx, created.ID, client.ContainerStartOptions{}); e != nil {
		return nil, e
	}
	var archive bytes.Buffer
	tw := tar.NewWriter(&archive)
	tw.WriteHeader(&tar.Header{Name: "railpack", Mode: 0755, Uid: 65532, Gid: 65532, Size: int64(len(binary))})
	tw.Write(binary)
	tw.WriteHeader(&tar.Header{Name: "app/", Typeflag: tar.TypeDir, Mode: 0755, Uid: 65532, Gid: 65532})
	e = filepath.WalkDir(source, func(file string, entry fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if file == source {
			return nil
		}
		rel, _ := filepath.Rel(source, file)
		info, e := entry.Info()
		if e != nil {
			return e
		}
		h := &tar.Header{Name: "app/" + filepath.ToSlash(rel), Mode: int64(info.Mode().Perm()), Uid: 65532, Gid: 65532, Size: info.Size()}
		if entry.IsDir() {
			h.Typeflag = tar.TypeDir
			h.Size = 0
			h.Mode = 0755
		}
		if e = tw.WriteHeader(h); e != nil {
			return e
		}
		if !entry.IsDir() {
			b, e := os.ReadFile(file)
			if e != nil {
				return e
			}
			_, e = tw.Write(b)
			return e
		}
		return nil
	})
	if e != nil {
		return nil, e
	}
	tw.Close()
	if _, e = r.execInput(ctx, created.ID, []string{"tar", "-x", "-C", "/work"}, nil, &archive); e != nil {
		return nil, e
	}
	args := []string{"/work/railpack", "prepare", "--plan-out", "/work/plan.json", "--info-out", "/work/info.json"}
	env := []string{}
	for _, v := range d.Config.Variables {
		if v.Scope == "build" || v.Scope == "both" {
			args = append(args, "--env", v.Name+"="+v.Value)
		}
	}
	for key, value := range map[string]string{"RAILPACK_INSTALL_CMD": d.Config.InstallCommand, "RAILPACK_BUILD_CMD": d.Config.BuildCommand, "RAILPACK_START_CMD": d.Config.StartCommand} {
		if value != "" {
			args = append(args, "--env", key+"="+value)
		}
	}
	args = append(args, "/work/app")
	out, e := r.execEnv(ctx, created.ID, args, env)
	for _, line := range strings.Split(string(out), "\n") {
		if line != "" {
			log(line)
		}
	}
	if e != nil {
		return nil, fmt.Errorf("Railpack could not prepare this application: %w", e)
	}
	plan, e := r.exec(ctx, created.ID, []string{"cat", "/work/plan.json"})
	if e != nil {
		return nil, fmt.Errorf("Read Railpack build plan: %w", e)
	}
	if len(plan) >= 4<<20 {
		return nil, errors.New("Railpack plan is too large")
	}
	var valid any
	if json.Unmarshal(plan, &valid) != nil {
		return nil, errors.New("Railpack returned an invalid build plan")
	}
	return plan, nil
}
func (r *DockerRuntime) railpackBinary(ctx context.Context, arch, checksum string) ([]byte, error) {
	base := r.DataDir
	if base == "" {
		base = ".shipper"
	}
	cache := filepath.Join(base, "tools", "railpack-"+RailpackVersion+"-"+arch+".tar.gz")
	archive, e := os.ReadFile(cache)
	if e != nil {
		req, e := http.NewRequestWithContext(ctx, "GET", "https://github.com/railwayapp/railpack/releases/download/v"+RailpackVersion+"/railpack-v"+RailpackVersion+"-"+arch+"-unknown-linux-musl.tar.gz", nil)
		if e != nil {
			return nil, e
		}
		response, e := (&http.Client{Timeout: 2 * time.Minute}).Do(req)
		if e != nil {
			return nil, e
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			return nil, fmt.Errorf("Railpack download returned HTTP %d", response.StatusCode)
		}
		archive, e = io.ReadAll(io.LimitReader(response.Body, 64<<20))
		if e != nil {
			return nil, e
		}
	}
	sum := sha256.Sum256(archive)
	if hex.EncodeToString(sum[:]) != checksum {
		return nil, errors.New("Railpack release checksum did not match")
	}
	if e = os.MkdirAll(filepath.Dir(cache), 0700); e != nil {
		return nil, e
	}
	if e = os.WriteFile(cache, archive, 0600); e != nil {
		return nil, e
	}
	gz, e := gzip.NewReader(bytes.NewReader(archive))
	if e != nil {
		return nil, e
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, e := tr.Next()
		if e != nil {
			return nil, e
		}
		if filepath.Base(h.Name) == "railpack" && h.Typeflag == tar.TypeReg && h.Size < 128<<20 {
			return io.ReadAll(io.LimitReader(tr, 128<<20))
		}
	}
}
