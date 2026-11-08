package deploy

import (
	"archive/tar"
	"context"
	"errors"
	"github.com/moby/moby/client"
	"io"
	"os"
	"shipper/internal/auth"
	"shipper/internal/protection"
	"shipper/internal/store"
)

// DumpRedisVolume requests a native RDB replication snapshot without stopping Redis.
// Restore remains a new retained volume, so the serving database is never replaced.
func (r *DockerRuntime) DumpRedisVolume(ctx context.Context, d store.Deployment, file string) error {
	id, e := r.databaseContainer(ctx, d, store.BackupSpec{})
	if e != nil {
		return e
	}
	path := "/tmp/shipper-" + auth.Token()[:24] + ".rdb"
	defer func() { _, _ = r.exec(context.WithoutCancel(ctx), id, []string{"rm", "-f", path}) }()
	password := databaseEnvironment(d.Config)["REDIS_PASSWORD"]
	if e = r.protectionExec(ctx, id, []string{"redis-cli", "--tls", "--cacert", "/shipper-tls/ca.crt", "--host", "127.0.0.1", "--rdb", path}, []string{"REDISCLI_AUTH=" + password}, nil, io.Discard); e != nil {
		return e
	}
	res, e := r.Client.CopyFromContainer(ctx, id, client.CopyFromContainerOptions{SourcePath: path})
	if e != nil {
		return e
	}
	defer res.Content.Close()
	tr := tar.NewReader(res.Content)
	h, e := tr.Next()
	if e != nil {
		return e
	}
	if h.Size <= 0 || h.Size > protection.MaxBytes {
		return errors.New("Redis snapshot exceeds the 2 GiB limit")
	}
	f, e := os.OpenFile(file, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	tw := tar.NewWriter(f)
	e = tw.WriteHeader(&tar.Header{Name: "dump.rdb", Mode: 0600, Uid: 999, Gid: 999, Size: h.Size})
	if e == nil {
		_, e = io.CopyN(tw, tr, h.Size)
	}
	if ce := tw.Close(); e == nil {
		e = ce
	}
	if ce := f.Close(); e == nil {
		e = ce
	}
	return e
}
