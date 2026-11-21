package deploy

import (
	"archive/tar"
	"bytes"
	"context"
	"github.com/moby/moby/client"
	"io"
	"os"
	"path/filepath"
	"shipper/internal/auth"
	"shipper/internal/engine"
	"shipper/internal/events"
	"shipper/internal/store"
	"strings"
	"testing"
	"time"
)

func TestDockerManagedDatabaseResources(t *testing.T) {
	if os.Getenv("SHIPPER_TEST_DATABASE_RESOURCES") != "1" {
		t.Skip("set SHIPPER_TEST_DATABASE_RESOURCES=1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 18*time.Minute)
	defer cancel()
	docker, e := engine.New()
	if e != nil {
		t.Fatal(e)
	}
	defer docker.Close()
	dir := t.TempDir()
	db, e := store.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	r := &DockerRuntime{Client: docker.Client(), Instance: auth.Token(), DataDir: dir}
	w := NewWorker(db, r, events.New(), dir)
	t.Cleanup(func() {
		clean, stop := context.WithTimeout(context.Background(), time.Minute)
		defer stop()
		cs, _ := r.Client.ContainerList(clean, client.ContainerListOptions{All: true, Filters: make(client.Filters).Add("label", "io.shipper.instance="+r.Instance)})
		for _, c := range cs.Items {
			if owned(c.Labels, r.Instance, "") {
				_ = r.removeOwned(clean, c.ID, "")
			}
		}
		vs, _ := r.Client.VolumeList(clean, client.VolumeListOptions{Filters: make(client.Filters).Add("label", "io.shipper.instance="+r.Instance)})
		for _, v := range vs.Items {
			if owned(v.Labels, r.Instance, "") {
				_, _ = r.Client.VolumeRemove(clean, v.Name, client.VolumeRemoveOptions{})
			}
		}
		ns, _ := r.Client.NetworkList(clean, client.NetworkListOptions{Filters: make(client.Filters).Add("label", "io.shipper.instance="+r.Instance)})
		for _, n := range ns.Items {
			if owned(n.Labels, r.Instance, "") {
				_, _ = r.Client.NetworkRemove(clean, n.ID, client.NetworkRemoveOptions{})
			}
		}
	})
	for _, fixture := range []struct{ engine, version string }{{"postgresql", "18"}, {"mysql", "8.4"}, {"mariadb", "11.8"}, {"redis", "7"}, {"redis", "8"}} {
		t.Run(fixture.engine+"/"+fixture.version, func(t *testing.T) {
			c, e := DatabaseResourceConfig(fixture.engine, fixture.version)
			if e != nil {
				t.Fatal(e)
			}
			if e = ValidateBuild(&c); e != nil {
				t.Fatal(e)
			}
			app := store.Application{ID: auth.Token()[:24], Name: fixture.engine, Config: c}
			d, e := db.Enqueue(ctx, app, store.Deployment{ID: auth.Token()[:24], Operation: "deploy"}, "", true)
			if e != nil {
				t.Fatal(e)
			}
			d.ImageID = c.Image
			d.ResolvedMethod = "image"
			d.RuntimePort = c.Port
			if e = r.pull(ctx, c.Image, func(line string) { t.Log(line) }); e != nil {
				t.Fatal(e)
			}
			d.ContainerID, e = r.Start(ctx, d)
			if e != nil {
				t.Fatal(e)
			}
			if e = r.Healthy(ctx, d); e != nil {
				t.Fatal("Database did not become ready before promotion", e)
			}
			d.State = "succeeded"
			if e = db.SaveDeployment(ctx, d); e != nil {
				t.Fatal(e)
			}
			if e = db.SetActive(ctx, app.ID, d.ID); e != nil {
				t.Fatal(e)
			}
			if e = r.reconcileApplicationAliases(ctx, map[string]store.Deployment{app.ID: d}); e != nil {
				t.Fatal(e)
			}
			t.Cleanup(func() { _ = r.removeOwned(context.Background(), d.ContainerID, app.ID) })
			if r.DatabaseLegacyTLS(ctx, d) {
				t.Fatal("a new database still requires TLS on the private network")
			}
			// Any application reaches the database by name over the shared
			// private network, with nothing to connect and no CA to install.
			clientConfig := store.Config{Method: "image", Image: c.Image, StartupPolicy: "process", StartCommand: "sleep 900"}
			consumer := store.Deployment{ID: auth.Token()[:24], ApplicationID: auth.Token()[:24], Config: clientConfig, ImageID: c.Image, ResolvedMethod: "image"}
			consumer.ContainerID, e = r.Start(ctx, consumer)
			if e != nil {
				t.Fatal(e)
			}
			defer func() {
				clean, stop := context.WithTimeout(context.Background(), 30*time.Second)
				defer stop()
				_ = r.removeOwned(clean, consumer.ContainerID, consumer.ApplicationID)
				_ = r.removeOwned(clean, d.ContainerID, app.ID)
			}()
			env := databaseEnvironment(c)
			host := "db-" + app.ID
			query := func(database, sql string, tls bool) (string, error) {
				queryContext, stop := context.WithTimeout(ctx, 10*time.Second)
				defer stop()
				var args, vars []string
				if fixture.engine == "redis" {
					args = append([]string{"redis-cli", "-h", host, "--no-auth-warning", "--raw"}, strings.Fields(sql)...)
					vars = []string{"REDISCLI_AUTH=" + env["REDIS_PASSWORD"]}
				} else {
					spec, _ := DatabaseBackupSpec(d)
					spec.Host = "127.0.0.1"
					spec.TLS = "require"
					args, vars = dbArgs(spec)
					for i, v := range args {
						if strings.HasPrefix(v, "--host=") {
							args[i] = "--host=" + host
						}
					}
					if fixture.engine == "postgresql" {
						mode := "require"
						if !tls {
							mode = "disable"
						}
						vars = append(vars, "PGSSLMODE="+mode)
						args = append([]string{"psql", "-qAt", "--no-psqlrc", "--set=ON_ERROR_STOP=1", "--dbname=" + database, "--command=" + sql}, args...)
					} else {
						tool := "mysql"
						if fixture.engine == "mariadb" {
							tool = "mariadb"
						}
						for i, v := range args {
							if strings.HasPrefix(v, "--ssl-mode=") {
								if tls {
									args[i] = "--ssl-mode=REQUIRED"
								} else {
									// caching_sha2_password sends the password over a plain
									// connection only RSA-encrypted with the server's key.
									args[i] = "--ssl-mode=DISABLED"
									if fixture.engine == "mysql" {
										args = append(args, "--get-server-public-key")
									}
								}
							}
						}
						if fixture.engine == "mariadb" {
							if tls {
								args = append(args, "--ssl", "--skip-ssl-verify-server-cert")
							} else {
								args = append(args, "--skip-ssl")
							}
						}
						args = append([]string{tool, "--batch", "--skip-column-names"}, append(args, database, "--execute="+sql)...)
					}
				}
				out, e := r.execEnv(queryContext, consumer.ContainerID, args, vars)
				return strings.TrimSpace(string(out)), e
			}
			ping := "SELECT 1"
			if fixture.engine == "redis" {
				ping = "PING"
			}
			for deadline := time.Now().Add(4 * time.Minute); ; {
				_, e = query("app", ping, false)
				if e == nil {
					break
				}
				if time.Now().After(deadline) {
					_ = r.Logs(ctx, app.ID, d.ContainerID, 40, func(line string) error { t.Log(line); return nil })
					t.Fatal("plain private connection", e)
				}
				time.Sleep(time.Second)
			}
			if fixture.engine != "redis" {
				// Clients that still ask for TLS, such as sslmode=require, keep working.
				if out, err := query("app", ping, true); err != nil {
					t.Fatal("TLS private connection", out, err)
				}
			}
			if e = r.Healthy(ctx, d); e != nil {
				t.Fatal("native engine health", e)
			}

			uri, uriErr := DatabaseURI(d)
			if uriErr != nil {
				t.Fatal(uriErr)
			}
			if strings.Contains(uri, "sslmode") || strings.HasPrefix(uri, "rediss:") {
				t.Fatal("private connection string asks for TLS", uri)
			}
			if fixture.engine == "postgresql" {
				out, err := r.execEnv(ctx, consumer.ContainerID, []string{"psql", uri, "-qAtc", "CREATE TEMP TABLE shipper_application_write (id integer); INSERT INTO shipper_application_write VALUES(42); SELECT id FROM shipper_application_write"}, nil)
				if err != nil || strings.TrimSpace(string(out)) != "42" {
					t.Fatal("generated URI application read/write", string(out), err)
				}
			}
			if fixture.engine == "redis" {
				out, err := r.execEnv(ctx, consumer.ContainerID, []string{"redis-cli", "-u", uri, "--no-auth-warning", "--raw", "PING"}, nil)
				if err != nil || strings.TrimSpace(string(out)) != "PONG" {
					t.Fatal("generated URI", string(out), err)
				}
			}
			if fixture.engine == "mysql" || fixture.engine == "mariadb" {
				prefix := "MYSQL_"
				tool := "mysql"
				plain := []string{"--ssl-mode=DISABLED", "--get-server-public-key"}
				if fixture.engine == "mariadb" {
					prefix = "MARIADB_"
					tool = "mariadb"
					plain = []string{"--skip-ssl"}
				}
				args := append([]string{tool, "--host=" + host, "--user=" + env[prefix+"USER"], "--batch", "--skip-column-names"}, plain...)
				args = append(args, "app", "--execute=CREATE TEMPORARY TABLE shipper_application_write (id integer); INSERT INTO shipper_application_write VALUES(42); SELECT id FROM shipper_application_write")
				out, err := r.execEnv(ctx, consumer.ContainerID, args, []string{"MYSQL_PWD=" + env[prefix+"PASSWORD"]})
				if err != nil || strings.TrimSpace(string(out)) != "42" {
					t.Fatal("generated credentials application read/write", string(out), err)
				}
			}

			if e = w.EnsureDatabaseBackupPlans(ctx); e != nil {
				t.Fatal(e)
			}
			plans, e := db.BackupPlans(ctx, app.ID)
			if e != nil || len(plans) != 1 || plans[0].Schedule != "0 3 * * *" {
				t.Fatal("default scheduled protection", plans, e)
			}
			if fixture.engine == "redis" {
				if out, e := query("", "SET shipper_key persisted", true); e != nil || out != "OK" {
					t.Fatal(out, e)
				}
				if _, e = query("", "SAVE", true); e != nil {
					t.Fatal(e)
				}
			} else {
				if _, e = query("app", "CREATE TABLE messages(id integer PRIMARY KEY, body varchar(80)); INSERT INTO messages VALUES(1,'persisted');", true); e != nil {
					t.Fatal(e)
				}
			}
			backup, e := db.QueueBackup(ctx, store.BackupRun{ApplicationID: app.ID, PlanID: plans[0].ID, Operation: "backup", Config: plans[0].Config})
			if e != nil {
				t.Fatal(e)
			}
			if !w.ProcessBackup(ctx) {
				t.Fatal("backup not processed")
			}
			backup, e = db.BackupRun(ctx, backup.ID)
			if e != nil || backup.State != "succeeded" {
				t.Fatal("backup", backup.Error, e)
			}
			spec := plans[0].Config
			spec.Target = "restored_copy"
			restore, e := db.QueueBackup(ctx, store.BackupRun{ApplicationID: app.ID, Operation: "restore", SourceID: backup.ID, Config: spec})
			if e != nil {
				t.Fatal(e)
			}
			w.ProcessBackup(ctx)
			restore, _ = db.BackupRun(ctx, restore.ID)
			if restore.State != "succeeded" || restore.VerifiedAt == 0 {
				t.Fatal("restore", restore.Error)
			}
			if fixture.engine != "redis" {
				if out, e := query("restored_copy", "SELECT body FROM messages", true); e != nil || out != "persisted" {
					t.Fatal("copy data", out, e)
				}
				dump := filepath.Join(t.TempDir(), "native.dump")
				if _, e = r.DumpDatabase(ctx, d, spec, dump); e != nil {
					t.Fatal(e)
				}
				f, e := os.Open(dump)
				if e != nil {
					t.Fatal(e)
				}
				importRun, e := w.ImportDatabaseDump(ctx, app.ID, "imported_copy", f)
				f.Close()
				if e != nil {
					t.Fatal("native import", e)
				}
				w.ProcessBackup(ctx)
				importRun, _ = db.BackupRun(ctx, importRun.ID)
				if importRun.State != "succeeded" {
					t.Fatal("native restore", importRun.Error)
				}
				if out, e := query("imported_copy", "SELECT body FROM messages", true); e != nil || out != "persisted" {
					t.Fatal("imported data", out, e)
				}
				if out, e := query("app", "SELECT count(*) FROM messages", true); e != nil || out != "1" {
					t.Fatal("original changed", out, e)
				}
			} else {
				checkCopy := func(volume string) {
					t.Helper()
					copy := d
					copy.ID = auth.Token()[:24]
					copy.ContainerID = ""
					copy.Config.Networking.Ports = nil
					copy.Config.VolumeSources = map[string]string{"data": volume}
					copy.ContainerID, e = r.Start(ctx, copy)
					if e != nil {
						t.Fatal("Start restored Redis copy", e)
					}
					t.Cleanup(func() { _ = r.removeOwned(context.Background(), copy.ContainerID, app.ID) })
					if e = r.Healthy(ctx, copy); e != nil {
						t.Fatal("Restored Redis readiness", e)
					}
					copyOutput, err := r.execEnv(ctx, copy.ContainerID, []string{"redis-cli", "-h", "127.0.0.1", "--raw", "GET", "shipper_key"}, []string{"REDISCLI_AUTH=" + env["REDIS_PASSWORD"]})
					if err != nil || strings.TrimSpace(string(copyOutput)) != "persisted" {
						t.Fatal("Restored Redis data", err)
					}
					if out, err := query("", "GET shipper_key", true); err != nil || out != "persisted" {
						t.Fatal("Original Redis data changed", err)
					}
					if e = r.removeOwned(ctx, copy.ContainerID, app.ID); e != nil {
						t.Fatal(e)
					}
				}
				checkCopy(restore.Result)
				out, e := r.Client.CopyFromContainer(ctx, d.ContainerID, client.CopyFromContainerOptions{SourcePath: "/data/dump.rdb"})
				if e != nil {
					t.Fatal(e)
				}
				tr := tar.NewReader(out.Content)
				if _, e = tr.Next(); e != nil {
					t.Fatal(e)
				}
				native, e := io.ReadAll(tr)
				out.Content.Close()
				if e != nil {
					t.Fatal(e)
				}
				run, e := w.ImportDatabaseDump(ctx, app.ID, "", bytes.NewReader(native))
				if e != nil {
					t.Fatal(e)
				}
				w.ProcessBackup(ctx)
				run, _ = db.BackupRun(ctx, run.ID)
				if run.State != "succeeded" {
					t.Fatal("RDB import", run.Error)
				}
				checkCopy(run.Result)
			}
		})
	}
}
