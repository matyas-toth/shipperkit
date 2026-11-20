package deploy

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"shipper/internal/store"
)

// MongoReplicaSet names the single-node replica set every managed MongoDB runs.
// Transactions, change streams and Prisma all require a replica set.
const MongoReplicaSet = "rs0"

const mongoKeyFile = "/shipper-mongo/keyfile"

func managedMongo(d store.Deployment) bool {
	return d.Config.Database != nil && d.Config.Database.Engine == "mongodb" && d.TaskCommand == nil
}

// mongoMember is the member name clients learn from the replica set. It is the
// stable private alias connected applications already resolve, and the
// container hostname so the server recognizes itself before any alias exists.
func mongoMember(app string) string { return "db-" + app + ":27017" }

// configureMongo keeps the official entrypoint, which creates the root user on a
// first start without these flags. The health check initiates the replica set
// once and only passes on a writable primary, so traffic never switches early.
func configureMongo(d store.Deployment, c *container.Config) {
	if !managedMongo(d) {
		return
	}
	c.Hostname = "db-" + d.ApplicationID
	c.Entrypoint = nil
	c.Cmd = []string{"mongod", "--bind_ip_all", "--replSet", MongoReplicaSet, "--keyFile", mongoKeyFile, "--oplogSize", "1024"}
	script := `try { rs.status() } catch (e) { if (e.codeName !== "NotYetInitialized") throw e; rs.initiate({ _id: "` + MongoReplicaSet + `", members: [{ _id: 0, host: "` + mongoMember(d.ApplicationID) + `" }] }) }
if (!db.hello().isWritablePrimary) quit(1)`
	health := `mongosh --quiet --norc "mongodb://127.0.0.1:27017/admin?directConnection=true" --username "$MONGO_INITDB_ROOT_USERNAME" --password "$MONGO_INITDB_ROOT_PASSWORD" --authenticationDatabase admin --eval '` + script + `' >/dev/null`
	c.Healthcheck = &container.HealthConfig{Test: []string{"CMD-SHELL", health}, Interval: 30 * time.Second, StartInterval: 2 * time.Second, Timeout: 10 * time.Second, Retries: 3, StartPeriod: 240 * time.Second}
}

// seedMongoKey writes the internal member key before mongod starts. A single
// member never authenticates to another, so a fresh key on each start is fine.
func (r *DockerRuntime) seedMongoKey(ctx context.Context, d store.Deployment, id string) error {
	if !managedMongo(d) {
		return nil
	}
	raw := make([]byte, 756)
	if _, e := rand.Read(raw); e != nil {
		return e
	}
	key := []byte(base64.StdEncoding.EncodeToString(raw))
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if e := tw.WriteHeader(&tar.Header{Name: "shipper-mongo", Typeflag: tar.TypeDir, Mode: 0700, Uid: 999, Gid: 999}); e != nil {
		return e
	}
	if e := tw.WriteHeader(&tar.Header{Name: "shipper-mongo/keyfile", Mode: 0400, Size: int64(len(key)), Uid: 999, Gid: 999}); e != nil {
		return e
	}
	if _, e := tw.Write(key); e != nil {
		return e
	}
	if e := tw.Close(); e != nil {
		return e
	}
	_, e := r.Client.CopyToContainer(ctx, id, client.CopyToContainerOptions{DestinationPath: "/", Content: &buf, CopyUIDGID: true})
	return e
}

// configureClickHouse only promotes a server that answers queries; it can start
// listening and still abort seconds later.
func configureClickHouse(d store.Deployment, c *container.Config) {
	if d.Config.Database == nil || d.Config.Database.Engine != "clickhouse" || d.TaskCommand != nil {
		return
	}
	health := `clickhouse-client --host=127.0.0.1 --user="$CLICKHOUSE_USER" --password="$CLICKHOUSE_PASSWORD" --query='SELECT 1' >/dev/null`
	c.Healthcheck = &container.HealthConfig{Test: []string{"CMD-SHELL", health}, Interval: 30 * time.Second, StartInterval: 2 * time.Second, Timeout: 10 * time.Second, Retries: 3, StartPeriod: 240 * time.Second}
}
