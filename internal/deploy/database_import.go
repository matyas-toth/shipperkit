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
	"strings"
	"time"
)

const MaxDatabaseImportBytes int64 = 512 << 20

// ImportDatabaseDump freezes native uploads into an encrypted, checksummed archive,
// then queues a restore copy through the normal durable protection worker.
func (w *Worker) ImportDatabaseDump(ctx context.Context, app, target string, src io.Reader) (store.BackupRun, error) {
	var empty store.BackupRun
	a, e := w.DB.Application(ctx, app)
	if e != nil {
		return empty, e
	}
	d, e := w.DB.Deployment(ctx, a.ActiveDeploymentID)
	if e != nil {
		return empty, e
	}
	spec, e := DatabaseBackupSpec(d)
	if e != nil {
		return empty, e
	}
	if spec.Kind == "volume" {
		spec.Volume = runtimeVolumeName(w.Runtime, app)
	}
	if spec.Kind == "database" && (!ValidRecoveryDatabase(target) || target == spec.Database) {
		return empty, errors.New("Enter a new database name")
	}
	runtime, ok := w.Runtime.(*DockerRuntime)
	if !ok {
		return empty, errors.New("Docker protection is unavailable")
	}
	root := filepath.Join(w.DataDir, "backups")
	if e = os.MkdirAll(root, 0700); e != nil {
		return empty, e
	}
	work, e := os.MkdirTemp(root, "import-")
	if e != nil {
		return empty, e
	}
	defer os.RemoveAll(work)
	raw := filepath.Join(work, "native.dump")
	f, e := os.OpenFile(raw, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if e != nil {
		return empty, e
	}
	defer f.Close()
	n, e := io.Copy(f, io.LimitReader(src, MaxDatabaseImportBytes+1))
	if e != nil {
		return empty, e
	}
	if n == 0 || n > MaxDatabaseImportBytes {
		return empty, errors.New("Choose a dump smaller than 512 MiB")
	}
	if _, e = f.Seek(0, 0); e != nil {
		return empty, e
	}
	magic := make([]byte, 5)
	io.ReadFull(f, magic)
	f.Seek(0, 0)
	m := protection.Manifest{Kind: spec.Kind, Instance: runtime.Instance, Application: app, Engine: spec.Engine, Database: spec.Database, CreatedAt: time.Now().Unix(), Consistency: "Native dump import"}
	files := map[string]string{"database.dump": raw}
	if spec.Kind == "database" {
		if spec.Engine == "postgres" && string(magic) != "PGDMP" {
			return empty, errors.New("Upload a PostgreSQL custom-format dump created with pg_dump -Fc")
		}
		if spec.Engine != "postgres" {
			// A copy restore cannot permit a dump to select or create another database.
			data, e := os.ReadFile(raw)
			if e != nil {
				return empty, e
			}
			upper := strings.ToUpper(string(data))
			for _, word := range []string{"USE ", "CREATE DATABASE", "DROP DATABASE", "CONNECT ", "SOURCE ", "\\!", "\\."} {
				if strings.Contains(upper, word) {
					return empty, errors.New("Export one database without database selection, client commands or database creation statements")
				}
			}
		}
		id, e := runtime.databaseContainer(ctx, d, spec)
		if e != nil {
			return empty, e
		}
		m.EngineVersion, e = runtime.DatabaseVersion(ctx, id, spec)
		if e != nil {
			return empty, e
		}
	} else {
		if string(magic) != "REDIS" {
			return empty, errors.New("Upload a Redis RDB snapshot")
		}
		rawTar := filepath.Join(work, "volume.tar")
		out, e := os.OpenFile(rawTar, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return empty, e
		}
		tw := tar.NewWriter(out)
		e = tw.WriteHeader(&tar.Header{Name: "dump.rdb", Mode: 0600, Size: n, Uid: 999, Gid: 999})
		if e == nil {
			_, e = io.Copy(tw, f)
		}
		if ce := tw.Close(); e == nil {
			e = ce
		}
		if ce := out.Close(); e == nil {
			e = ce
		}
		if e != nil {
			return empty, e
		}
		files = map[string]string{"volume.tar": rawTar}
	}
	_, recipient, e := w.RecoveryKey(ctx)
	if e != nil {
		return empty, e
	}
	// Seal streams through a private file so the upload does not consume its size in RAM.
	archive, e := os.OpenFile(filepath.Join(work, "native.age"), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if e != nil {
		return empty, e
	}
	defer archive.Close()
	if _, e = protection.Seal(archive, recipient, m, files); e != nil {
		return empty, e
	}
	archive.Seek(0, 0)
	backup, e := w.ImportBackup(ctx, app, "", archive)
	if e != nil {
		return empty, e
	}
	spec.Target = target
	spec.NativeImport = true
	run, e := w.DB.QueueBackup(ctx, store.BackupRun{ApplicationID: app, Operation: "restore", SourceID: backup.ID, Config: spec})
	if e == nil {
		w.Trigger()
	}
	return run, e
}

func runtimeVolumeName(runtime Runtime, app string) string {
	if r, ok := runtime.(*DockerRuntime); ok {
		return r.prefix() + "-" + app + "-data"
	}
	return ""
}
