package deploy

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"

	"shipper/internal/protection"
	"shipper/internal/store"
)

type ProtectedVolume struct {
	ApplicationID   string   `json:"applicationId"`
	ApplicationName string   `json:"applicationName"`
	Name            string   `json:"name"`
	Logical         string   `json:"logical"`
	Attached        bool     `json:"attached"`
	Running         bool     `json:"running"`
	Restored        bool     `json:"restored"`
	Services        []string `json:"services"`
}

func (r *DockerRuntime) ProtectedVolumes(ctx context.Context, app string) ([]ProtectedVolume, error) {
	filters := make(client.Filters).Add("label", "io.shipper.instance="+r.Instance)
	if app != "" {
		filters = filters.Add("label", "io.shipper.application.id="+app)
	}
	vs, e := r.Client.VolumeList(ctx, client.VolumeListOptions{Filters: filters})
	if e != nil {
		return nil, e
	}
	cs, e := r.Client.ContainerList(ctx, client.ContainerListOptions{All: true})
	if e != nil {
		return nil, e
	}
	out := []ProtectedVolume{}
	for _, v := range vs.Items {
		if !owned(v.Labels, r.Instance, app) || v.Labels["io.shipper.application.id"] == "" || v.Labels["io.shipper.infrastructure"] != "" {
			continue
		}
		p := ProtectedVolume{ApplicationID: v.Labels["io.shipper.application.id"], Name: v.Name, Logical: v.Labels["com.docker.compose.volume"], Restored: v.Labels["io.shipper.backup"] != "", Services: []string{}}
		if p.Logical == "" {
			p.Logical = strings.TrimPrefix(v.Name, r.prefix()+"-"+p.ApplicationID+"-")
		}
		for _, c := range cs.Items {
			for _, m := range c.Mounts {
				if m.Name == v.Name {
					p.Attached = true
					if c.State == container.StateRunning {
						p.Running = true
					}
					if svc := c.Labels["com.docker.compose.service"]; svc != "" {
						p.Services = append(p.Services, svc)
					}
				}
			}
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := strings.ToLower(out[i].Logical), strings.ToLower(out[j].Logical)
		if a != b {
			return a < b
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}
func (r *DockerRuntime) RetainedVolumeApplication(ctx context.Context, name string) (string, error) {
	v, e := r.Client.VolumeInspect(ctx, name, client.VolumeInspectOptions{})
	if e != nil {
		return "", e
	}
	app := v.Volume.Labels["io.shipper.application.id"]
	if app == "" {
		return "", errors.New("Only Shipper application data can be deleted")
	}
	return app, r.checkVolumeSource(ctx, app, name)
}
func (r *DockerRuntime) cleanBackupHelpers(ctx context.Context) error {
	items, e := r.Client.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: make(client.Filters).Add("label", "io.shipper.instance="+r.Instance).Add("label", "io.shipper.infrastructure=backup-helper")})
	if e != nil {
		return e
	}
	for _, c := range items.Items {
		app := c.Labels["io.shipper.application.id"]
		if app != "" && owned(c.Labels, r.Instance, app) && c.Labels["io.shipper.infrastructure"] == "backup-helper" {
			if e = r.removeOwned(ctx, c.ID, app); e != nil {
				return e
			}
		}
	}
	return nil
}
func (r *DockerRuntime) checkVolumeSource(ctx context.Context, app, name string) error {
	v, e := r.Client.VolumeInspect(ctx, name, client.VolumeInspectOptions{})
	if e != nil {
		return e
	}
	if !owned(v.Volume.Labels, r.Instance, app) || v.Volume.Labels["io.shipper.infrastructure"] != "" {
		return errors.New("Retained volume must belong to this application")
	}
	return nil
}
func (r *DockerRuntime) reuseVolumeSources(ctx context.Context, d store.Deployment, p *composePlan) error {
	volumes, _ := p.Project["volumes"].(map[string]any)
	for key, source := range d.Config.VolumeSources {
		if e := r.checkVolumeSource(ctx, d.ApplicationID, source); e != nil {
			return e
		}
		v, ok := volumes[key].(map[string]any)
		if !ok {
			return errors.New("Retained volume mapping does not match this Compose release")
		}
		v["name"] = source
		v["external"] = true
		delete(v, "driver")
		delete(v, "driver_opts")
		delete(v, "labels")
	}
	return nil
}
func (r *DockerRuntime) DeleteRetainedVolume(ctx context.Context, app, name string) error {
	if e := r.checkVolumeSource(ctx, app, name); e != nil {
		return e
	}
	cs, e := r.Client.ContainerList(ctx, client.ContainerListOptions{All: true})
	if e != nil {
		return e
	}
	for _, c := range cs.Items {
		for _, m := range c.Mounts {
			if m.Name == name {
				return errors.New("This volume is still attached to a container")
			}
		}
	}
	_, e = r.Client.VolumeRemove(ctx, name, client.VolumeRemoveOptions{Force: false})
	return e
}
func (r *DockerRuntime) coldVolume(ctx context.Context, app, name string) error {
	v, e := r.Client.VolumeInspect(ctx, name, client.VolumeInspectOptions{})
	if e != nil {
		return e
	}
	if !owned(v.Volume.Labels, r.Instance, app) || v.Volume.Labels["io.shipper.infrastructure"] != "" {
		return errors.New("Only this application's data volumes can be protected")
	}
	cs, e := r.Client.ContainerList(ctx, client.ContainerListOptions{All: true})
	if e != nil {
		return e
	}
	for _, c := range cs.Items {
		for _, m := range c.Mounts {
			if m.Name == name && (!owned(c.Labels, r.Instance, app) || c.State == container.StateRunning || c.State == container.StatePaused) {
				return errors.New("Stop all writers through Shipper before archiving this volume; shared or foreign mounts are not supported")
			}
		}
	}
	return nil
}
func (r *DockerRuntime) volumeHelper(ctx context.Context, app, id, name string, readonly bool) (string, error) {
	if e := r.pull(ctx, ApacheImage, func(string) {}); e != nil {
		return "", e
	}
	labels := r.labels(app, "")
	labels["io.shipper.infrastructure"] = "backup-helper"
	c, e := r.Client.ContainerCreate(ctx, client.ContainerCreateOptions{Name: r.prefix() + "-backup-" + id, Config: &container.Config{Image: ApacheImage, Entrypoint: []string{"/bin/sh", "-c"}, Cmd: []string{"sleep 1800"}, Labels: labels, NetworkDisabled: true}, HostConfig: &container.HostConfig{ReadonlyRootfs: true, CapDrop: []string{"ALL"}, SecurityOpt: []string{"no-new-privileges:true"}, Mounts: []mount.Mount{{Type: mount.TypeVolume, Source: name, Target: "/data", ReadOnly: readonly}}, Resources: container.Resources{Memory: 64 << 20, PidsLimit: ptr(int64(16))}, LogConfig: container.LogConfig{Type: "none"}}})
	if e != nil {
		return "", e
	}
	return c.ID, nil
}

// ValidateVolumeTar rejects paths/links which could escape the selected volume.
func ValidateVolumeTar(src io.Reader) error {
	tr := tar.NewReader(io.LimitReader(src, protection.MaxBytes+1))
	var size int64
	count := 0
	for {
		h, e := tr.Next()
		if e == io.EOF {
			return nil
		}
		if e != nil {
			return e
		}
		count++
		n := path.Clean(strings.TrimPrefix(h.Name, "./"))
		if count > 200000 || strings.HasPrefix(n, "/") || n == ".." || strings.HasPrefix(n, "../") || strings.ContainsAny(n, "\\:\x00") {
			return errors.New("Unsafe volume archive path")
		}
		switch h.Typeflag {
		case tar.TypeDir, tar.TypeReg, tar.TypeRegA:
		case tar.TypeSymlink:
			target := path.Clean(path.Join(path.Dir(n), h.Linkname))
			if path.IsAbs(h.Linkname) || target == ".." || strings.HasPrefix(target, "../") || strings.ContainsAny(h.Linkname, "\\:\x00") {
				return errors.New("Unsafe volume symbolic link")
			}
		default:
			return errors.New("Volume archives support regular files, directories and internal symbolic links")
		}
		size += h.Size
		if size > protection.MaxBytes {
			return errors.New("Volume archive exceeds the 2 GiB limit")
		}
	}
}
func (r *DockerRuntime) DumpVolume(ctx context.Context, app, id, name, file string) error {
	if e := r.coldVolume(ctx, app, name); e != nil {
		return e
	}
	helper, e := r.volumeHelper(ctx, app, id, name, true)
	if e != nil {
		return e
	}
	defer r.removeOwned(context.Background(), helper, app)
	res, e := r.Client.CopyFromContainer(ctx, helper, client.CopyFromContainerOptions{SourcePath: "/data/."})
	if e != nil {
		return e
	}
	defer res.Content.Close()
	f, e := os.OpenFile(file, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	n, e := io.Copy(f, io.LimitReader(res.Content, protection.MaxBytes+1))
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	if n > protection.MaxBytes {
		return errors.New("Volume archive exceeds the 2 GiB limit")
	}
	f, e = os.Open(file)
	if e != nil {
		return e
	}
	defer f.Close()
	return ValidateVolumeTar(f)
}
func (r *DockerRuntime) RestoreVolume(ctx context.Context, app, id, file string) (string, error) {
	f, e := os.Open(file)
	if e != nil {
		return "", e
	}
	defer f.Close()
	if e = ValidateVolumeTar(f); e != nil {
		return "", e
	}
	if _, e = f.Seek(0, 0); e != nil {
		return "", e
	}
	name := r.prefix() + "-" + app + "-recovered-" + id
	labels := r.labels(app, "")
	labels["io.shipper.backup"] = id
	if _, e = r.Client.VolumeInspect(ctx, name, client.VolumeInspectOptions{}); e == nil {
		return "", errors.New("Recovery volume already exists; inspect the interrupted result")
	} else if !errdefs.IsNotFound(e) {
		return "", e
	}
	if _, e = r.Client.VolumeCreate(ctx, client.VolumeCreateOptions{Name: name, Labels: labels}); e != nil {
		return "", e
	}
	helper, e := r.volumeHelper(ctx, app, id, name, false)
	if e != nil {
		return name, e
	}
	defer r.removeOwned(context.Background(), helper, app)
	_, e = r.Client.CopyToContainer(ctx, helper, client.CopyToContainerOptions{DestinationPath: "/data", Content: f, CopyUIDGID: true, AllowOverwriteDirWithFile: false})
	if e == nil {
		e = r.verifyVolume(ctx, helper, file)
	}
	return name, e
}
func volumeInventory(src io.Reader) (map[string]string, error) {
	out := map[string]string{}
	tr := tar.NewReader(src)
	for {
		h, e := tr.Next()
		if e == io.EOF {
			return out, nil
		}
		if e != nil {
			return nil, e
		}
		name := path.Clean(h.Name)
		if name == "." {
			continue
		}
		hash := sha256.New()
		if _, e = io.Copy(hash, tr); e != nil {
			return nil, e
		}
		out[name] = fmt.Sprintf("%d:%d:%d:%d:%s:%s", h.Typeflag, h.Mode, h.Uid, h.Gid, h.Linkname, hex.EncodeToString(hash.Sum(nil)))
	}
}
func (r *DockerRuntime) verifyVolume(ctx context.Context, helper, file string) error {
	source, e := os.Open(file)
	if e != nil {
		return e
	}
	expected, e := volumeInventory(source)
	source.Close()
	if e != nil {
		return e
	}
	res, e := r.Client.CopyFromContainer(ctx, helper, client.CopyFromContainerOptions{SourcePath: "/data/."})
	if e != nil {
		return e
	}
	defer res.Content.Close()
	actual, e := volumeInventory(io.LimitReader(res.Content, protection.MaxBytes+1))
	if e != nil {
		return e
	}
	if !reflect.DeepEqual(expected, actual) {
		return errors.New("Recovered volume contents or permissions failed verification; the original volume is preserved")
	}
	return nil
}

// Binary output is streamed to disk, while stdin and stdout are pumped concurrently.
func (r *DockerRuntime) protectionExec(ctx context.Context, id string, args, env []string, input io.Reader, output io.Writer) error {
	ex, e := r.Client.ExecCreate(ctx, id, client.ExecCreateOptions{Cmd: args, Env: env, AttachStdin: input != nil, AttachStdout: true, AttachStderr: true})
	if e != nil {
		return e
	}
	stream, e := r.Client.ExecAttach(ctx, ex.ID, client.ExecAttachOptions{})
	if e != nil {
		return e
	}
	defer stream.Close()
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			stream.Close()
		case <-done:
		}
	}()
	sent := make(chan error, 1)
	if input != nil {
		go func() {
			_, e := io.Copy(stream.Conn, input)
			if e == nil {
				e = stream.CloseWrite()
			}
			sent <- e
		}()
	} else {
		sent <- nil
	}
	var stderr cappedBuffer
	_, e = stdcopy.StdCopy(output, &stderr, stream.Reader)
	if e != nil {
		return e
	}
	if e = <-sent; e != nil {
		return e
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	for {
		state, e := r.Client.ExecInspect(ctx, ex.ID, client.ExecInspectOptions{})
		if e != nil {
			return e
		}
		if !state.Running {
			if state.ExitCode != 0 {
				// Tool diagnostics can contain SQL, values or connection details. Return
				// only a bounded category, never dump output or credentials.
				message := strings.ToLower(stderr.String())
				reason := "Check the database credentials, permissions and native client tools"
				switch {
				case state.ExitCode == 127:
					reason = "The selected database engine's native client tool is missing from this service"
				case strings.Contains(message, "access denied") || strings.Contains(message, "authentication failed"):
					reason = "Database authentication failed; update the backup connection credentials"
				case strings.Contains(message, "ssl") || strings.Contains(message, "tls") || strings.Contains(message, "secure connection"):
					reason = "Database TLS negotiation failed; review the connection TLS mode and trust material"
				case strings.Contains(message, "already exists"):
					reason = "The recovery database already exists; choose a new name"
				case strings.Contains(message, "permission denied") || strings.Contains(message, "privilege"):
					reason = "The database user does not have the required backup or restore permissions"
				}
				return fmt.Errorf("%s (exit %d)", reason, state.ExitCode)
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

var databaseName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,62}$`)
var sourceDatabaseName = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_.-]{0,62}$`)

func ValidRecoveryDatabase(name string) bool { return databaseName.MatchString(name) }
func (r *DockerRuntime) InferBackupCredentials(ctx context.Context, d store.Deployment, c *store.BackupSpec) error {
	id, e := r.databaseContainer(ctx, d, *c)
	if e != nil {
		return e
	}
	v, e := r.Client.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if e != nil {
		return e
	}
	env := map[string]string{}
	for _, raw := range v.Container.Config.Env {
		k, value, _ := strings.Cut(raw, "=")
		env[k] = value
	}
	if c.User == "" {
		if c.Engine == "postgres" {
			c.User = env["POSTGRES_USER"]
			if c.User == "" {
				c.User = "postgres"
			}
		} else if c.Engine == "mongodb" {
			c.User = env["MONGO_INITDB_ROOT_USERNAME"]
			if c.User == "" {
				c.User = "root"
			}
		} else {
			c.User = "root"
		}
	}
	if c.Password == "" {
		if c.Engine == "postgres" {
			c.Password = env["POSTGRES_PASSWORD"]
		} else if c.Engine == "mongodb" {
			c.Password = env["MONGO_INITDB_ROOT_PASSWORD"]
		} else if c.User == "root" {
			c.Password = env["MYSQL_ROOT_PASSWORD"]
			if c.Password == "" {
				c.Password = env["MARIADB_ROOT_PASSWORD"]
			}
		} else {
			c.Password = env["MYSQL_PASSWORD"]
			if c.Password == "" {
				c.Password = env["MARIADB_PASSWORD"]
			}
		}
	}
	if c.Engine == "mongodb" && c.Database == "" {
		c.Database = env["MONGO_INITDB_DATABASE"]
		if c.Database == "" {
			c.Database = "app"
		}
	}
	return nil
}
func ValidateBackupSpec(c *store.BackupSpec) error {
	if c.Keep == 0 {
		c.Keep = 7
	}
	if c.Keep < 1 || c.Keep > 100 {
		return errors.New("Keep between 1 and 100 backups")
	}
	if c.Kind == "instance" {
		return nil
	}
	if c.Kind == "volume" {
		if c.Volume == "" || len(c.Volume) > 255 {
			return errors.New("Choose a persistent volume")
		}
		return nil
	}
	if c.Kind != "database" || (c.Engine != "postgres" && c.Engine != "mysql" && c.Engine != "mariadb" && c.Engine != "mongodb") {
		return errors.New("Choose PostgreSQL, MySQL, MariaDB or MongoDB")
	}
	if !sourceDatabaseName.MatchString(c.Database) || c.User == "" || strings.HasPrefix(c.User, "-") || len(c.User) > 128 || strings.ContainsAny(c.User, "\x00\r\n") {
		return errors.New("Enter a database name and user")
	}
	if c.Host == "" {
		c.Host = "127.0.0.1"
	}
	if c.Host != "127.0.0.1" && c.Host != "localhost" {
		return errors.New("Database protection connects inside the selected service")
	}
	if c.TLS == "" {
		c.TLS = "prefer"
	}
	if c.TLS != "disable" && c.TLS != "prefer" && c.TLS != "require" && c.TLS != "verify-full" {
		return errors.New("Choose a supported TLS mode")
	}
	if c.Port < 0 || c.Port > 65535 || len(c.Password) > 4096 || strings.ContainsRune(c.Password, 0) {
		return errors.New("Invalid database credentials")
	}
	return nil
}
func (r *DockerRuntime) databaseContainer(ctx context.Context, d store.Deployment, c store.BackupSpec) (string, error) {
	var items []container.Summary
	var e error
	if ReleaseMethod(d) == "compose" {
		// Compose retains unchanged service containers across releases. Its
		// owned project is the stable identity, not a deployment label.
		items, e = r.composeContainers(ctx, d.ApplicationID)
	} else {
		var listed client.ContainerListResult
		listed, e = r.Client.ContainerList(ctx, client.ContainerListOptions{Filters: make(client.Filters).Add("label", "io.shipper.instance="+r.Instance).Add("label", "io.shipper.application.id="+d.ApplicationID).Add("label", "io.shipper.deployment.id="+d.ID)})
		items = listed.Items
	}
	if e != nil {
		return "", e
	}
	ids := []string{}
	for _, v := range items {
		if v.State == "running" && owned(v.Labels, r.Instance, d.ApplicationID) &&
			!strings.EqualFold(v.Labels["com.docker.compose.oneoff"], "true") &&
			(c.Service == "" || v.Labels["com.docker.compose.service"] == c.Service) {
			ids = append(ids, v.ID)
		}
	}
	if len(ids) != 1 {
		return "", errors.New("Choose exactly one running database service")
	}
	return ids[0], nil
}
func dbArgs(c store.BackupSpec) ([]string, []string) {
	port := c.Port
	if port == 0 {
		if c.Engine == "postgres" {
			port = 5432
		} else {
			port = 3306
		}
	}
	if c.Engine == "postgres" {
		return []string{"--host=" + c.Host, "--port=" + strconv.Itoa(port), "--username=" + c.User}, []string{"PGPASSWORD=" + c.Password, "PGSSLMODE=" + c.TLS, "PGCONNECT_TIMEOUT=15"}
	}
	a := []string{"--host=" + c.Host, "--port=" + strconv.Itoa(port), "--user=" + c.User}
	if c.Engine == "mysql" {
		mode := map[string]string{"disable": "DISABLED", "prefer": "PREFERRED", "require": "REQUIRED", "verify-full": "VERIFY_IDENTITY"}[c.TLS]
		a = append(a, "--ssl-mode="+mode)
	} else {
		switch c.TLS {
		case "disable":
			a = append(a, "--skip-ssl")
		case "require":
			a = append(a, "--ssl", "--ssl-verify-server-cert=0")
		case "verify-full":
			a = append(a, "--ssl", "--ssl-verify-server-cert")
		case "prefer":
			a = append(a, "--ssl", "--ssl-verify-server-cert=0")
		}
	}
	return a, []string{"MYSQL_PWD=" + c.Password}
}
func (r *DockerRuntime) DatabaseVersion(ctx context.Context, id string, c store.BackupSpec) (string, error) {
	tool := map[string]string{"postgres": "pg_dump", "mysql": "mysqldump", "mariadb": "mariadb-dump", "mongodb": "mongodump"}[c.Engine]
	var out cappedBuffer
	e := r.protectionExec(ctx, id, []string{tool, "--version"}, nil, nil, &out)
	v := strings.TrimSpace(out.String())
	if e == nil && ((c.Engine == "mysql" && strings.Contains(strings.ToLower(v), "mariadb")) || (c.Engine == "mariadb" && !strings.Contains(strings.ToLower(v), "mariadb"))) {
		e = errors.New("The selected engine does not match the database tools")
	}
	return v, e
}

type limitFile struct {
	w io.Writer
	n int64
}

func (l *limitFile) Write(p []byte) (int, error) {
	if l.n+int64(len(p)) > protection.MaxBytes {
		return 0, errors.New("Backup exceeds the 2 GiB limit")
	}
	n, e := l.w.Write(p)
	l.n += int64(n)
	return n, e
}
func mongoConnection(c store.BackupSpec, database string) string {
	port := c.Port
	if port == 0 {
		port = 27017
	}
	u := url.URL{Scheme: "mongodb", Host: net.JoinHostPort(c.Host, strconv.Itoa(port)), Path: "/" + database}
	if c.User != "" {
		u.User = url.UserPassword(c.User, c.Password)
	}
	if c.User != "" {
		query := u.Query()
		query.Set("authSource", "admin")
		u.RawQuery = query.Encode()
	}
	return u.String()
}
func (r *DockerRuntime) dumpMongoDatabase(ctx context.Context, id string, c store.BackupSpec, file string) (string, error) {
	version, e := r.DatabaseVersion(ctx, id, c)
	if e != nil {
		return "", e
	}
	f, e := os.OpenFile(file, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return "", e
	}
	env := []string{"MONGO_URI=" + mongoConnection(c, c.Database)}
	e = r.protectionExec(ctx, id, []string{"sh", "-c", `exec mongodump --uri="$MONGO_URI" --archive --gzip`}, env, nil, &limitFile{w: f})
	ce := f.Close()
	if e == nil {
		e = ce
	}
	return version, e
}
func (r *DockerRuntime) restoreMongoDatabase(ctx context.Context, id string, c store.BackupSpec, file string) error {
	f, e := os.Open(file)
	if e != nil {
		return e
	}
	defer f.Close()
	env := []string{
		"MONGO_URI=" + mongoConnection(c, c.Target),
		"MONGO_NS_FROM=" + c.Database + ".*",
		"MONGO_NS_TO=" + c.Target + ".*",
	}
	return r.protectionExec(ctx, id, []string{"sh", "-c", `exec mongorestore --uri="$MONGO_URI" --archive=/dev/stdin --gzip --nsInclude="$MONGO_NS_FROM" --nsFrom="$MONGO_NS_FROM" --nsTo="$MONGO_NS_TO"`}, env, f, io.Discard)
}
func (r *DockerRuntime) DumpDatabase(ctx context.Context, d store.Deployment, c store.BackupSpec, file string) (string, error) {
	id, e := r.databaseContainer(ctx, d, c)
	if e != nil {
		return "", e
	}
	if c.Engine == "mongodb" {
		return r.dumpMongoDatabase(ctx, id, c, file)
	}
	version, e := r.DatabaseVersion(ctx, id, c)
	if e != nil {
		return "", e
	}
	args, env := dbArgs(c)
	if c.Engine == "postgres" {
		args = append([]string{"pg_dump", "--format=custom", "--no-owner", "--no-acl"}, args...)
		args = append(args, "--dbname="+c.Database)
	} else {
		tool := "mysqldump"
		if c.Engine == "mariadb" {
			tool = "mariadb-dump"
		}
		args = append([]string{tool, "--lock-all-tables", "--quick", "--routines", "--events", "--triggers", "--hex-blob"}, args...)
		if c.Engine == "mysql" {
			args = append(args, "--no-tablespaces", "--set-gtid-purged=OFF")
		}
		args = append(args, c.Database)
	}
	f, e := os.OpenFile(file, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return "", e
	}
	e = r.protectionExec(ctx, id, args, env, nil, &limitFile{w: f})
	ce := f.Close()
	if e == nil {
		e = ce
	}
	return version, e
}
func (r *DockerRuntime) RestoreDatabase(ctx context.Context, d store.Deployment, c store.BackupSpec, m protection.Manifest, file string) error {
	if !databaseName.MatchString(c.Target) || c.Target == c.Database {
		return errors.New("Restore into a new database with a different name")
	}
	id, e := r.databaseContainer(ctx, d, c)
	if e != nil {
		return e
	}
	version, e := r.DatabaseVersion(ctx, id, c)
	if e != nil {
		return e
	}
	if m.Engine != c.Engine || m.EngineVersion != version {
		return errors.New("Restore requires the same database tool version and engine as the backup")
	}
	if c.Engine == "mongodb" {
		return r.restoreMongoDatabase(ctx, id, c, file)
	}
	if c.NativeImport {
		return r.restoreNativeDatabase(ctx, id, c, file)
	}
	a, env := dbArgs(c)
	run := func(args []string, in io.Reader) error { return r.protectionExec(ctx, id, args, env, in, io.Discard) }
	f, e := os.Open(file)
	if e != nil {
		return e
	}
	defer f.Close()
	if c.Engine == "postgres" {
		if e = run(append([]string{"createdb"}, append(a, "--", c.Target)...), nil); e != nil {
			return e
		}
		e = run(append([]string{"pg_restore", "--exit-on-error", "--single-transaction", "--no-owner", "--no-acl", "--dbname=" + c.Target}, a...), f)
		if e == nil {
			e = run(append([]string{"psql", "--no-psqlrc", "--set=ON_ERROR_STOP=1", "--dbname=" + c.Target, "--command=BEGIN; CREATE TABLE shipper_restore_probe (id integer); INSERT INTO shipper_restore_probe VALUES (1); SELECT id FROM shipper_restore_probe; ROLLBACK;"}, a...), nil)
		}
	} else {
		tool := "mysql"
		if c.Engine == "mariadb" {
			tool = "mariadb"
		}
		if e = run(append([]string{tool}, append(a, "--execute=CREATE DATABASE `"+c.Target+"`")...), nil); e != nil {
			return e
		}
		e = run(append([]string{tool}, append(a, c.Target)...), f)
		if e == nil {
			e = run(append([]string{tool}, append(a, c.Target, "--execute=CREATE TABLE shipper_restore_probe (id integer); INSERT INTO shipper_restore_probe VALUES (1); SELECT id FROM shipper_restore_probe; DROP TABLE shipper_restore_probe;")...), nil)
		}
	}
	return e
}
func (r *DockerRuntime) BackupACME(ctx context.Context) ([]byte, error) {
	v, e := r.Client.ContainerInspect(ctx, r.proxy(), client.ContainerInspectOptions{})
	if errdefs.IsNotFound(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	if !owned(v.Container.Config.Labels, r.Instance, "") {
		return nil, errors.New("Proxy ownership check failed")
	}
	res, e := r.Client.CopyFromContainer(ctx, r.proxy(), client.CopyFromContainerOptions{SourcePath: "/data/acme.json"})
	if errdefs.IsNotFound(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	defer res.Content.Close()
	tr := tar.NewReader(res.Content)
	h, e := tr.Next()
	if e != nil || h.Size > 8<<20 {
		return nil, errors.New("Invalid certificate state")
	}
	b, e := io.ReadAll(io.LimitReader(tr, 8<<20))
	if e != nil {
		return nil, e
	}
	if len(b) == 0 {
		return nil, nil
	}
	if !json.Valid(b) {
		return nil, errors.New("Certificate state changed during backup; retry the backup")
	}
	return b, nil
}
func (r *DockerRuntime) restoreACME(ctx context.Context, id string) error {
	marker := filepath.Join(r.DataDir, "recovered-acme.json")
	f, e := os.Open(marker)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	data, e := protection.ReadBounded(f, 8<<20)
	closeError := f.Close()
	if e == nil {
		e = closeError
	}
	if e != nil || !json.Valid(data) {
		return errors.New("Recovered certificate state is invalid")
	}
	res, e := r.Client.CopyFromContainer(ctx, id, client.CopyFromContainerOptions{SourcePath: "/data/acme.json"})
	if e == nil {
		tr := tar.NewReader(res.Content)
		_, e = tr.Next()
		var existing []byte
		if e == nil {
			existing, e = protection.ReadBounded(tr, 8<<20)
		}
		res.Content.Close()
		if e != nil {
			return e
		}
		if len(existing) > 0 {
			if string(existing) != string(data) {
				return errors.New("Recovered certificate state conflicts with existing proxy data; review it before starting the proxy")
			}
			return os.Remove(marker)
		}
	} else if !errdefs.IsNotFound(e) {
		return e
	}
	v, e := r.Client.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if e != nil {
		return e
	}
	if v.Container.State.Running {
		return errors.New("Stop the proxy before installing recovered certificate state")
	}
	if e = r.copy(ctx, id, "/data", map[string][]byte{"acme.json": data}); e != nil {
		return e
	}
	return os.Remove(marker)
}
func RecoveredProxy(dir string) (ProxyConfig, error) {
	var c ProxyConfig
	f, e := os.Open(filepath.Join(dir, "proxy-recovery.json"))
	if os.IsNotExist(e) {
		return c, nil
	}
	if e != nil {
		return c, e
	}
	defer f.Close()
	data, e := protection.ReadBounded(f, 65536)
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(data, &c)
	return c, e
}
