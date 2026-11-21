package deploy

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/moby/moby/client"
	"shipper/internal/auth"
	"shipper/internal/engine"
	"shipper/internal/events"
	"shipper/internal/store"
)

// Each engine signs in with the rotated password after the follow-up release,
// and no longer with the old one.
func TestDockerDatabasePasswordRotation(t *testing.T) {
	if os.Getenv("SHIPPER_TEST_DATABASE_RESOURCES") != "1" {
		t.Skip("set SHIPPER_TEST_DATABASE_RESOURCES=1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
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
	_ = NewWorker(db, r, events.New(), dir)
	t.Cleanup(func() {
		clean, stop := context.WithTimeout(context.Background(), time.Minute)
		defer stop()
		cs, _ := r.Client.ContainerList(clean, client.ContainerListOptions{All: true, Filters: make(client.Filters).Add("label", "io.shipper.instance="+r.Instance)})
		for _, c := range cs.Items {
			_ = r.removeOwned(clean, c.ID, "")
		}
		vs, _ := r.Client.VolumeList(clean, client.VolumeListOptions{Filters: make(client.Filters).Add("label", "io.shipper.instance="+r.Instance)})
		for _, v := range vs.Items {
			_, _ = r.Client.VolumeRemove(clean, v.Name, client.VolumeRemoveOptions{})
		}
		ns, _ := r.Client.NetworkList(clean, client.NetworkListOptions{Filters: make(client.Filters).Add("label", "io.shipper.instance="+r.Instance)})
		for _, n := range ns.Items {
			_, _ = r.Client.NetworkRemove(clean, n.ID, client.NetworkRemoveOptions{})
		}
	})
	// Each check signs in over TCP with the password it is given.
	checks := map[string][]string{
		"postgresql": {"sh", "-c", `PGPASSWORD="$CHECK" PGSSLMODE=require psql --host=127.0.0.1 --username="$POSTGRES_USER" --dbname="$POSTGRES_DB" -qAtc 'SELECT 1'`},
		"mysql":      {"sh", "-c", `MYSQL_PWD="$CHECK" mysql --host=127.0.0.1 --user="$MYSQL_USER" --ssl-mode=REQUIRED --execute='SELECT 1' "$MYSQL_DATABASE"`},
		"mariadb":    {"sh", "-c", `MYSQL_PWD="$CHECK" mariadb --host=127.0.0.1 --user="$MARIADB_USER" --ssl --ssl-verify-server-cert=0 --execute='SELECT 1' "$MARIADB_DATABASE"`},
		"clickhouse": {"sh", "-c", `clickhouse-client --host=127.0.0.1 --user="$CLICKHOUSE_USER" --password="$CHECK" --query='SELECT 1'`},
		"redis":      {"sh", "-c", `REDISCLI_AUTH="$CHECK" redis-cli -h 127.0.0.1 PING | grep -qx PONG`},
	}
	for _, fixture := range []struct{ engine, version string }{{"postgresql", "18"}, {"mysql", "8.4"}, {"mariadb", "11.8"}, {"clickhouse", "25.3"}, {"redis", "8"}} {
		t.Run(fixture.engine, func(t *testing.T) {
			c, e := NamedDatabaseResourceConfig(fixture.engine, fixture.version, "shop", "orders")
			if e != nil {
				t.Fatal(e)
			}
			if e = r.pull(ctx, c.Image, func(string) {}); e != nil {
				t.Fatal(e)
			}
			app := auth.Token()[:24]
			start := func(config store.Config) store.Deployment {
				t.Helper()
				d := store.Deployment{ID: auth.Token()[:24], ApplicationID: app, Config: config, ImageID: config.Image, ResolvedMethod: "image", RuntimePort: config.Port}
				if d.ContainerID, e = r.Start(ctx, d); e != nil {
					t.Fatal(e)
				}
				if e = r.Healthy(ctx, d); e != nil {
					t.Fatal("database did not become ready", e)
				}
				return d
			}
			signIn := func(d store.Deployment, password string) error {
				deadline := time.Now().Add(90 * time.Second)
				for {
					_, err := r.execEnv(ctx, d.ContainerID, checks[fixture.engine], []string{"CHECK=" + password})
					if err == nil || time.Now().After(deadline) {
						return err
					}
					time.Sleep(time.Second)
				}
			}
			d := start(c)
			key := DatabasePasswordKey(fixture.engine)
			previous := databaseEnvironment(c)[key]
			if e = signIn(d, previous); e != nil {
				t.Fatal("initial sign in", e)
			}
			password := auth.Token()
			if e = r.SetDatabasePassword(ctx, d, password); e != nil {
				t.Fatal(e)
			}
			next, e := WithDatabasePassword(c, password)
			if e != nil {
				t.Fatal(e)
			}
			if e = r.removeOwned(ctx, d.ContainerID, app); e != nil {
				t.Fatal(e)
			}
			rotated := start(next)
			if e = signIn(rotated, password); e != nil {
				t.Fatal("rotated sign in", e)
			}
			if _, e = r.execEnv(ctx, rotated.ContainerID, checks[fixture.engine], []string{"CHECK=" + previous}); e == nil {
				t.Fatal("the old password still signs in")
			}
		})
	}
}
