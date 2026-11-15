package deploy

import (
	"archive/tar"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"shipper/internal/protection"
	"shipper/internal/store"
	"strconv"
	"strings"
)

// redis-cli cannot truncate a pipe. For diskless replication it leaves the
// 40-byte EOF marker on stdout and reports the actual RDB size on stderr.
func finishRedisSnapshot(raw *os.File, diagnostic string) (int64, error) {
	info, e := raw.Stat()
	if e != nil {
		return 0, e
	}
	for _, line := range strings.Split(diagnostic, "\n") {
		if line == "Transfer finished with success." {
			return info.Size(), nil
		}
		count, ok := strings.CutPrefix(line, "Transfer finished with success after ")
		if !ok {
			continue
		}
		count, ok = strings.CutSuffix(count, " bytes")
		size, err := strconv.ParseInt(count, 10, 64)
		if !ok || err != nil || size <= 0 || size > protection.MaxBytes || info.Size()-size != 40 {
			break
		}
		if e = raw.Truncate(size); e != nil {
			return 0, e
		}
		return size, nil
	}
	return 0, errors.New("Redis did not finish a native RDB transfer")
}

// Managed Redis enables append-only persistence. Seed its base snapshot and
// manifest together so Redis 7 and 8 load the recovered data on normal startup.
func redisRestoreArchive(dst io.Writer, src io.Reader, size int64) error {
	if size <= 0 || size > protection.MaxBytes {
		return errors.New("Redis snapshot exceeds the 2 GiB limit")
	}
	tw := tar.NewWriter(dst)
	if e := tw.WriteHeader(&tar.Header{Name: "appendonlydir/", Typeflag: tar.TypeDir, Mode: 0700, Uid: 999, Gid: 999}); e != nil {
		return e
	}
	if e := tw.WriteHeader(&tar.Header{Name: "appendonlydir/appendonly.aof.1.base.rdb", Mode: 0600, Uid: 999, Gid: 999, Size: size}); e != nil {
		return e
	}
	if _, e := io.CopyN(tw, src, size); e != nil {
		return e
	}
	manifest := []byte("file appendonly.aof.1.base.rdb seq 1 type b\n")
	if e := tw.WriteHeader(&tar.Header{Name: "appendonlydir/appendonly.aof.manifest", Mode: 0600, Uid: 999, Gid: 999, Size: int64(len(manifest))}); e != nil {
		return e
	}
	if _, e := tw.Write(manifest); e != nil {
		return e
	}
	return tw.Close()
}

// DumpRedisVolume requests a native RDB replication snapshot without stopping Redis.
// Restore remains a new retained volume, so the serving database is never replaced.
func (r *DockerRuntime) DumpRedisVolume(ctx context.Context, d store.Deployment, file string) error {
	id, e := r.databaseContainer(ctx, d, store.BackupSpec{})
	if e != nil {
		return e
	}
	// Docker's archive API cannot read the container's tmpfs. Stream the native
	// snapshot to a bounded private host file instead of staging it in /tmp.
	raw, e := os.CreateTemp(filepath.Dir(file), "redis-*.rdb")
	if e != nil {
		return e
	}
	defer os.Remove(raw.Name())
	defer raw.Close()
	password := databaseEnvironment(d.Config)["REDIS_PASSWORD"]
	var diagnostic cappedBuffer
	if e = r.protectionExecDiagnostics(ctx, id, []string{"redis-cli", "--tls", "--cacert", "/shipper-tls/ca.crt", "-h", "127.0.0.1", "--rdb", "-"}, []string{"REDISCLI_AUTH=" + password}, nil, &limitFile{w: raw}, &diagnostic); e != nil {
		return e
	}
	size, e := finishRedisSnapshot(raw, diagnostic.String())
	if e != nil {
		return e
	}
	if size > protection.MaxBytes {
		return errors.New("Redis snapshot exceeds the 2 GiB limit")
	}
	if _, e = raw.Seek(0, io.SeekStart); e != nil {
		return e
	}
	var magic [5]byte
	if _, e = io.ReadFull(raw, magic[:]); e != nil || string(magic[:]) != "REDIS" {
		return errors.New("Redis did not return a native RDB snapshot")
	}
	if _, e = raw.Seek(0, io.SeekStart); e != nil {
		return e
	}
	f, e := os.OpenFile(file, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	e = redisRestoreArchive(f, raw, size)
	if ce := f.Close(); e == nil {
		e = ce
	}
	return e
}
