package deploy

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"

	"shipper/internal/auth"
	"shipper/internal/engine"
	"shipper/internal/events"
	"shipper/internal/protection"
	"shipper/internal/store"
)

// Real Docker tests have their own instance label and never touch operator workloads.
func TestDockerProtection(t *testing.T) {
	if os.Getenv("SHIPPER_TEST_BACKUPS") != "1" {
		t.Skip("set SHIPPER_TEST_BACKUPS=1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 18*time.Minute)
	defer cancel()
	engine, e := engine.New()
	if e != nil {
		t.Fatal(e)
	}
	defer engine.Close()
	r := &DockerRuntime{Client: engine.Client(), Instance: auth.Token()}
	dir := t.TempDir()
	r.DataDir = dir
	db, e := store.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	db.Setting(ctx, "instance_id", r.Instance)
	worker := NewWorker(db, r, events.New(), dir)
	t.Run("cold volume copy and original preservation", func(t *testing.T) {
		app := "volume-" + auth.Token()[:12]
		name := r.prefix() + "-" + app + "-data"
		_, e := r.Client.VolumeCreate(ctx, client.VolumeCreateOptions{Name: name, Labels: r.labels(app, "")})
		if e != nil {
			t.Fatal(e)
		}
		defer r.Client.VolumeRemove(context.Background(), name, client.VolumeRemoveOptions{})
		helper, e := r.volumeHelper(ctx, app, auth.Token()[:12], name, false)
		if e != nil {
			t.Fatal(e)
		}
		var b bytes.Buffer
		tw := tar.NewWriter(&b)
		content := "restored volume data\n"
		tw.WriteHeader(&tar.Header{Name: "message.txt", Mode: 0640, Size: int64(len(content)), Typeflag: tar.TypeReg, Uid: 1000, Gid: 1000})
		io.WriteString(tw, content)
		tw.Close()
		_, e = r.Client.CopyToContainer(ctx, helper, client.CopyToContainerOptions{DestinationPath: "/data", Content: &b, CopyUIDGID: true})
		r.removeOwned(context.Background(), helper, app)
		if e != nil {
			t.Fatal(e)
		}
		file := filepath.Join(t.TempDir(), "volume.tar")
		if e = r.DumpVolume(ctx, app, auth.Token()[:12], name, file); e != nil {
			t.Fatal(e)
		}
		restored, e := r.RestoreVolume(ctx, app, auth.Token()[:12], file)
		if restored != "" {
			defer r.Client.VolumeRemove(context.Background(), restored, client.VolumeRemoveOptions{})
		}
		if e != nil {
			t.Fatal(e)
		}
		for _, source := range []string{name, restored} {
			h, e := r.volumeHelper(ctx, app, auth.Token()[:12], source, true)
			if e != nil {
				t.Fatal(e)
			}
			res, e := r.Client.CopyFromContainer(ctx, h, client.CopyFromContainerOptions{SourcePath: "/data/message.txt"})
			if e != nil {
				t.Fatal(e)
			}
			tr := tar.NewReader(res.Content)
			header, e := tr.Next()
			if e != nil {
				t.Fatal(e)
			}
			got, _ := io.ReadAll(tr)
			res.Content.Close()
			r.removeOwned(context.Background(), h, app)
			if string(got) != content || header.Uid != 1000 || header.Mode != 0640 {
				t.Fatal("data or permissions did not round trip", string(got), header)
			}
		}
	})
	for _, fixture := range []struct {
		engine, image string
		env           []string
		path          string
	}{{"postgres", "postgres:17-alpine", []string{"POSTGRES_PASSWORD=test-backup-password", "POSTGRES_DB=originaldb"}, "/var/lib/postgresql/data"}, {"mysql", "mysql:8.4", []string{"MYSQL_ROOT_PASSWORD=test-backup-password", "MYSQL_DATABASE=originaldb"}, "/var/lib/mysql"}, {"mariadb", "mariadb:11", []string{"MARIADB_ROOT_PASSWORD=test-backup-password", "MARIADB_DATABASE=originaldb"}, "/var/lib/mysql"}} {
		t.Run(fixture.engine+" native restore read write", func(t *testing.T) {
			app := "db-" + auth.Token()[:12]
			dep := "release-" + auth.Token()[:12]
			a := store.Application{ID: app, Name: fixture.engine, Config: store.Config{Method: "image", Image: fixture.image}}
			d, e := db.Enqueue(ctx, a, store.Deployment{ID: dep, Operation: "deploy"}, "", true)
			if e != nil {
				t.Fatal(e)
			}
			d.State = "succeeded"
			if e = db.SaveDeployment(ctx, d); e != nil {
				t.Fatal(e)
			}
			if e = r.pull(ctx, fixture.image, func(s string) { t.Log(s) }); e != nil {
				t.Fatal(e)
			}
			name := r.prefix() + "-" + app + "-data"
			_, e = r.Client.VolumeCreate(ctx, client.VolumeCreateOptions{Name: name, Labels: r.labels(app, dep)})
			if e != nil {
				t.Fatal(e)
			}
			defer r.Client.VolumeRemove(context.Background(), name, client.VolumeRemoveOptions{})
			c, e := r.Client.ContainerCreate(ctx, client.ContainerCreateOptions{Name: r.prefix() + "-" + app, Config: &container.Config{Image: fixture.image, Env: fixture.env, Labels: r.labels(app, dep)}, HostConfig: &container.HostConfig{Mounts: []mount.Mount{{Type: mount.TypeVolume, Source: name, Target: fixture.path}}, Resources: container.Resources{Memory: 768 << 20}}})
			if e != nil {
				t.Fatal(e)
			}
			defer r.removeOwned(context.Background(), c.ID, app)
			if _, e = r.Client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{}); e != nil {
				t.Fatal(e)
			}
			spec := store.BackupSpec{Kind: "database", Engine: fixture.engine, Database: "originaldb", User: "root", Password: "test-backup-password", Host: "127.0.0.1", TLS: "disable", Keep: 2}
			if fixture.engine == "postgres" {
				spec.User = "postgres"
			}
			if fixture.engine == "mysql" {
				spec.TLS = "prefer"
			}
			args, env := dbArgs(spec)
			sqlRun := func(database, sql string) (string, error) {
				var out cappedBuffer
				var cmd []string
				if fixture.engine == "postgres" {
					cmd = append([]string{"psql", "--no-psqlrc", "--set=ON_ERROR_STOP=1", "--tuples-only", "--no-align", "--dbname=" + database, "--command=" + sql}, args...)
				} else {
					tool := "mysql"
					if fixture.engine == "mariadb" {
						tool = "mariadb"
					}
					cmd = append([]string{tool, "--batch", "--skip-column-names"}, args...)
					cmd = append(cmd, database, "--execute="+sql)
				}
				e := r.protectionExec(ctx, c.ID, cmd, env, nil, &out)
				return out.String(), e
			}
			deadline := time.Now().Add(4 * time.Minute)
			for {
				_, e = sqlRun("originaldb", "SELECT 1")
				if e == nil {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("database did not become ready", e)
				}
				time.Sleep(time.Second)
			}
			if _, e = sqlRun("originaldb", "CREATE TABLE messages(id integer PRIMARY KEY, body varchar(80)); INSERT INTO messages VALUES(1,'before backup');"); e != nil {
				t.Fatal(e)
			}
			dump := filepath.Join(t.TempDir(), "database.dump")
			version, e := r.DumpDatabase(ctx, d, spec, dump)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = sqlRun("originaldb", "INSERT INTO messages VALUES(2,'after backup');"); e != nil {
				t.Fatal(e)
			}
			spec.Target = "recovereddb"
			m := protection.Manifest{Engine: spec.Engine, EngineVersion: version}
			if e = r.RestoreDatabase(ctx, d, spec, m, dump); e != nil {
				t.Fatal(e)
			}
			out, e := sqlRun("recovereddb", "SELECT body FROM messages ORDER BY id; INSERT INTO messages VALUES(3,'new write'); SELECT body FROM messages WHERE id=3;")
			if e != nil || !strings.Contains(out, "before backup") || !strings.Contains(out, "new write") || strings.Contains(out, "after backup") {
				t.Fatal("restored application read/write failed", out, e)
			}
			out, e = sqlRun("originaldb", "SELECT count(*) FROM messages;")
			if e != nil || strings.TrimSpace(out) != "2" {
				t.Fatal("original database was changed", out, e)
			}
			if e = r.RestoreDatabase(ctx, d, spec, m, dump); e == nil {
				t.Fatal("existing recovery database overwritten")
			}
			if fixture.engine == "postgres" {
				t.Run("encrypted off-server recovery and retention", func(t *testing.T) {
					objects := map[string][]byte{}
					failPut := false
					objectStore := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
						if !strings.HasPrefix(req.Header.Get("Authorization"), "AWS4-HMAC-SHA256 ") {
							t.Error("unsigned S3 request")
						}
						switch req.Method {
						case "PUT":
							if failPut {
								w.WriteHeader(503)
								return
							}
							objects[req.URL.Path], _ = io.ReadAll(req.Body)
						case "GET":
							b, ok := objects[req.URL.Path]
							if !ok {
								w.WriteHeader(404)
								return
							}
							w.Write(b)
						case "DELETE":
							delete(objects, req.URL.Path)
						}
					}))
					defer objectStore.Close()
					dest := store.BackupDestination{ID: auth.Token()[:24], Name: "Recovery fixture", Endpoint: objectStore.URL, Region: "us-east-1", Bucket: "bucket", AccessKey: "fixture", SecretKey: "fixture-secret", Private: true}
					if e = db.SaveBackupDestination(ctx, dest); e != nil {
						t.Fatal(e)
					}
					if e = db.SetActive(ctx, app, dep); e != nil {
						t.Fatal(e)
					}
					spec.Destination = dest.ID
					spec.Target = ""
					spec.Keep = 1
					plan := store.BackupPlan{ID: auth.Token()[:24], ApplicationID: app, Name: "Database", Config: spec, Timezone: "UTC", Enabled: true}
					if e = db.SaveBackupPlan(ctx, plan); e != nil {
						t.Fatal(e)
					}
					backup, e := db.QueueBackup(ctx, store.BackupRun{PlanID: plan.ID, ApplicationID: app, Operation: "backup", Config: spec})
					if e != nil {
						t.Fatal(e)
					}
					worker.ProcessBackup(ctx)
					backup, e = db.BackupRun(ctx, backup.ID)
					if e != nil || backup.State != "succeeded" || backup.ObjectKey == "" {
						t.Fatal("off-server backup failed", backup, e)
					}
					if e = os.Remove(worker.BackupPath(backup.ID)); e != nil {
						t.Fatal(e)
					}
					restoreSpec := backup.Config
					restoreSpec.Target = "offserver_copy"
					job, e := db.QueueBackup(ctx, store.BackupRun{ApplicationID: app, Operation: "restore", Config: restoreSpec, SourceID: backup.ID})
					if e != nil {
						t.Fatal(e)
					}
					worker.ProcessBackup(ctx)
					job, e = db.BackupRun(ctx, job.ID)
					if e != nil || job.State != "succeeded" || job.VerifiedAt == 0 {
						t.Fatal("off-server restore failed", job, e)
					}
					out, e := sqlRun("offserver_copy", "SELECT count(*) FROM messages; INSERT INTO messages VALUES(4,'off-server write'); SELECT body FROM messages WHERE id=4;")
					if e != nil || !strings.Contains(out, "2") || !strings.Contains(out, "off-server write") {
						t.Fatal("off-server copy read/write failed", out, e)
					}
					next, e := db.QueueBackup(ctx, store.BackupRun{PlanID: plan.ID, ApplicationID: app, Operation: "backup", Config: spec})
					if e != nil {
						t.Fatal(e)
					}
					worker.ProcessBackup(ctx)
					next, e = db.BackupRun(ctx, next.ID)
					if e != nil || next.State != "succeeded" {
						t.Fatal(next, e)
					}
					old, _ := db.BackupRun(ctx, backup.ID)
					if old.State != "expired" || len(objects) != 1 {
						t.Fatal("retention did not remove only the previous object", old.State, len(objects))
					}
					key, _, e := worker.RecoveryKey(ctx)
					if e != nil {
						t.Fatal(e)
					}
					f, e := os.Open(worker.BackupPath(next.ID))
					if e != nil {
						t.Fatal(e)
					}
					imported, e := worker.ImportBackup(ctx, app, key, f)
					f.Close()
					if e != nil || imported.State != "succeeded" {
						t.Fatal("encrypted archive import failed", imported, e)
					}
					failPut = true
					failed, e := db.QueueBackup(ctx, store.BackupRun{PlanID: plan.ID, ApplicationID: app, Operation: "backup", Config: spec})
					if e != nil {
						t.Fatal(e)
					}
					worker.ProcessBackup(ctx)
					failed, e = db.BackupRun(ctx, failed.ID)
					if e != nil || failed.State != "failed" {
						t.Fatal("failed upload was not reported", failed, e)
					}
					if _, e = os.Stat(worker.BackupPath(failed.ID)); !os.IsNotExist(e) {
						t.Fatal("failed upload leaked a local archive", e)
					}
					if len(objects) != 1 {
						t.Fatal("failed upload pruned the previous off-server recovery point")
					}
				})
			}
		})
	}
	t.Run("portable instance secrets and identity", func(t *testing.T) {
		port := func() int {
			listener, e := net.Listen("tcp", "127.0.0.1:0")
			if e != nil {
				t.Fatal(e)
			}
			defer listener.Close()
			return listener.Addr().(*net.TCPAddr).Port
		}
		r.Config = ProxyConfig{BindIP: "127.0.0.1", HTTPPort: port(), HTTPSPort: port()}
		if e := r.Ensure(ctx); e != nil {
			t.Fatal(e)
		}
		defer func() {
			_ = r.removeOwned(context.Background(), r.proxy(), "")
			_, _ = r.Client.VolumeRemove(context.Background(), r.prefix()+"-proxy-data", client.VolumeRemoveOptions{})
			_, _ = r.Client.NetworkRemove(context.Background(), r.network(), client.NetworkRemoveOptions{})
		}()
		if e := r.copy(ctx, r.proxy(), "/data", map[string][]byte{"acme.json": []byte("{}")}); e != nil {
			t.Fatal(e)
		}
		key, _, e := worker.RecoveryKey(ctx)
		if e != nil {
			t.Fatal(e)
		}
		v := store.BackupRun{ID: auth.Token()[:24], Config: store.BackupSpec{Kind: "instance", Keep: 2}}
		if e = worker.protect(ctx, &v); e != nil {
			t.Fatal(e)
		}
		keyfile := filepath.Join(t.TempDir(), "identity.txt")
		os.WriteFile(keyfile, []byte(key), 0600)
		target := filepath.Join(t.TempDir(), "restored")
		if e = RecoverInstance(ctx, worker.BackupPath(v.ID), keyfile, target); e != nil {
			t.Fatal(e)
		}
		restored, e := store.Open(target)
		if e != nil {
			t.Fatal(e)
		}
		defer restored.Close()
		id, e := restored.Setting(ctx, "instance_id", "")
		if e != nil || id != r.Instance {
			t.Fatal("instance identity lost", e)
		}
		original, _ := os.ReadFile(filepath.Join(dir, "encryption.key"))
		recovered, _ := os.ReadFile(filepath.Join(target, "encryption.key"))
		if !bytes.Equal(original, recovered) {
			t.Fatal("original encryption key lost")
		}
		if e = r.removeOwned(ctx, r.proxy(), ""); e != nil {
			t.Fatal(e)
		}
		if _, e = r.Client.VolumeRemove(ctx, r.prefix()+"-proxy-data", client.VolumeRemoveOptions{}); e != nil {
			t.Fatal(e)
		}
		r.DataDir = target
		if e = r.Ensure(ctx); e != nil {
			t.Fatal(e)
		}
		acme, e := r.BackupACME(ctx)
		if e != nil || string(acme) != "{}" {
			t.Fatal("certificate state did not survive fresh-volume recovery", string(acme), e)
		}
		if _, e = os.Stat(filepath.Join(target, "recovered-acme.json")); !os.IsNotExist(e) {
			t.Fatal("certificate recovery marker was not consumed", e)
		}
		if e = os.WriteFile(filepath.Join(target, "recovered-acme.json"), []byte(`{"conflict":true}`), 0600); e != nil {
			t.Fatal(e)
		}
		if e = r.Ensure(ctx); e == nil {
			t.Fatal("conflicting certificate state overwritten")
		}
		if e = RecoverInstance(ctx, worker.BackupPath(v.ID), keyfile, target); e == nil {
			t.Fatal("existing installation overwritten")
		}
	})
}
func TestVolumeTarRejectsEscapingLinks(t *testing.T) {
	for _, h := range []*tar.Header{{Name: "../outside", Typeflag: tar.TypeReg}, {Name: "link", Typeflag: tar.TypeSymlink, Linkname: "../../outside"}, {Name: "/root", Typeflag: tar.TypeReg}, {Name: "device", Typeflag: tar.TypeChar}} {
		var b bytes.Buffer
		tw := tar.NewWriter(&b)
		tw.WriteHeader(h)
		tw.Close()
		if e := ValidateVolumeTar(&b); e == nil {
			t.Fatal("unsafe volume archive accepted", h)
		}
	}
}
