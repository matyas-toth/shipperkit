package deploy

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"shipper/internal/protection"
	"shipper/internal/store"
)

func (w *Worker) RecoveryKey(ctx context.Context) (string, string, error) {
	initial, _, e := protection.Identity()
	if e != nil {
		return "", "", e
	}
	key, e := w.DB.PrivateSetting(ctx, "recovery_identity", initial)
	if e != nil {
		return "", "", e
	}
	return protection.Recipient(key)
}
func (w *Worker) BackupPath(id string) string {
	return filepath.Join(w.DataDir, "backups", "archives", id+".age")
}
func (w *Worker) cleanBackupWorkspaces() error {
	root, e := filepath.Abs(filepath.Join(w.DataDir, "backups"))
	if e != nil {
		return e
	}
	entries, e := os.ReadDir(root)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		if !strings.HasPrefix(entry.Name(), "work-") && !strings.HasPrefix(entry.Name(), "import-") {
			continue
		}
		if strings.HasPrefix(entry.Name(), "import-") {
			info, e := entry.Info()
			if e != nil {
				return e
			}
			if time.Since(info.ModTime()) < 35*time.Minute {
				continue
			}
		}
		target := filepath.Join(root, entry.Name())
		if !strings.HasPrefix(target, root+string(filepath.Separator)) {
			return errors.New("Invalid backup workspace")
		}
		if e = os.RemoveAll(target); e != nil {
			return e
		}
	}
	return nil
}
func (w *Worker) backupDestination(ctx context.Context, id string) (protection.S3, error) {
	items, e := w.DB.BackupDestinations(ctx)
	if e != nil {
		return protection.S3{}, e
	}
	for _, d := range items {
		if d.ID == id {
			return protection.S3{Endpoint: d.Endpoint, Region: d.Region, Bucket: d.Bucket, Prefix: d.Prefix, AccessKey: d.AccessKey, SecretKey: d.SecretKey, Private: d.Private}, nil
		}
	}
	return protection.S3{}, errors.New("Backup destination is no longer available")
}
func (w *Worker) backupRunDestination(ctx context.Context, c store.BackupSpec) (protection.S3, error) {
	if c.Remote == nil {
		return w.backupDestination(ctx, c.Destination)
	}
	d := *c.Remote
	items, e := w.DB.BackupDestinations(ctx)
	if e != nil {
		return protection.S3{}, e
	}
	for _, current := range items {
		if current.ID == d.ID && current.Endpoint == d.Endpoint && current.Bucket == d.Bucket && current.Region == d.Region {
			d.AccessKey = current.AccessKey
			d.SecretKey = current.SecretKey
		}
	}
	return protection.S3{Endpoint: d.Endpoint, Region: d.Region, Bucket: d.Bucket, Prefix: d.Prefix, AccessKey: d.AccessKey, SecretKey: d.SecretKey, Private: d.Private}, nil
}
func (w *Worker) ScheduleBackups(ctx context.Context) {
	if e := w.EnsureDatabaseBackupPlans(ctx); e != nil {
		slog.Warn("Database backup setup will be retried", "error", e)
	}
	now := time.Now()
	plans, e := w.DB.DueBackupPlans(ctx, now.Unix())
	if e != nil {
		return
	}
	for _, p := range plans {
		next, e := NextSchedule(p.Schedule, p.Timezone, now)
		if e != nil {
			continue
		}
		if _, e = w.DB.QueueBackup(ctx, store.BackupRun{PlanID: p.ID, ApplicationID: p.ApplicationID, Operation: "backup", Config: p.Config}); e != nil && !errors.Is(e, store.ErrBusy) {
			continue
		}
		p.NextAt = next.Unix()
		_ = w.DB.SaveBackupPlan(ctx, p)
	}
}
func (w *Worker) ProcessBackup(ctx context.Context) bool {
	v, e := w.DB.PendingBackup(ctx)
	if errors.Is(e, sql.ErrNoRows) {
		return false
	}
	if e != nil {
		return false
	}
	v.State = "running"
	v.StartedAt = time.Now().Unix()
	if e = w.DB.SaveBackupRun(ctx, v); e != nil {
		return false
	}
	w.changed()
	work, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	e = w.protect(work, &v)
	v.FinishedAt = time.Now().Unix()
	if e != nil {
		// A failed off-server attempt must not accumulate a new multi-gigabyte
		// local artifact on every scheduled retry. Previous completed backups stay.
		if v.Operation == "backup" {
			_ = os.Remove(w.BackupPath(v.ID))
			v.Bytes = 0
			v.Checksum = ""
		}
		v.State = "failed"
		v.Error = "Backup operation failed: " + e.Error()
		for _, secret := range []string{v.Config.Password, v.Config.Identity} {
			if secret != "" {
				v.Error = strings.ReplaceAll(v.Error, secret, "[redacted]")
			}
		}
		if ctx.Err() != nil {
			v.State = "interrupted"
		}
	} else {
		v.State = "succeeded"
	}
	if se := w.DB.SaveBackupRun(context.Background(), v); se != nil {
		slog.Error("Backup result could not be recorded", "operation", v.ID, "error", se)
	}
	w.changed()
	if e == nil && v.Operation == "backup" {
		w.retainBackups(ctx, v)
	}
	return true
}
func (w *Worker) protect(ctx context.Context, v *store.BackupRun) error {
	runtime, ok := w.Runtime.(*DockerRuntime)
	if !ok {
		return errors.New("Docker protection is unavailable")
	}
	root := filepath.Join(w.DataDir, "backups")
	if e := os.MkdirAll(filepath.Join(root, "archives"), 0700); e != nil {
		return e
	}
	work, e := os.MkdirTemp(root, "work-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(work)
	if v.Operation == "delete-volume" {
		if e = w.DB.CheckVolumeUnreferenced(ctx, v.ApplicationID, v.Config.Volume); e != nil {
			return e
		}
		if e = runtime.DeleteRetainedVolume(ctx, v.ApplicationID, v.Config.Volume); e != nil {
			return e
		}
		v.Result = "Retained volume deleted"
		return nil
	}
	if v.Operation == "restore" {
		source, e := w.DB.BackupRun(ctx, v.SourceID)
		if e != nil {
			return e
		}
		if source.ApplicationID != v.ApplicationID || source.State != "succeeded" || source.Operation != "backup" {
			return errors.New("Choose a completed backup from this application")
		}
		file := w.BackupPath(source.ID)
		if _, e = os.Stat(file); os.IsNotExist(e) {
			if source.ObjectKey == "" {
				return errors.New("The local backup file is missing")
			}
			destination, e := w.backupRunDestination(ctx, source.Config)
			if e != nil {
				return e
			}
			file = filepath.Join(work, "remote.age")
			if e = destination.Get(ctx, source.ObjectKey, file, source.Checksum); e != nil {
				return e
			}
		}
		_, h, e := protection.Checksum(file)
		if e != nil || h != source.Checksum {
			return errors.New("Backup checksum does not match its record")
		}
		key := v.Config.Identity
		if key == "" {
			key = source.Config.Identity
		}
		if key == "" {
			key, _, e = w.RecoveryKey(ctx)
			if e != nil {
				return e
			}
		}
		f, e := os.Open(file)
		if e != nil {
			return e
		}
		m, e := protection.Open(f, key, work)
		f.Close()
		if e != nil {
			return e
		}
		if m.Application != v.ApplicationID || m.Kind != v.Config.Kind {
			return errors.New("Backup does not match the restore target")
		}
		if m.Kind == "volume" {
			v.Result, e = runtime.RestoreVolume(ctx, v.ApplicationID, v.ID, filepath.Join(work, "volume.tar"))
			if e == nil {
				v.VerifiedAt = time.Now().Unix()
				source.VerifiedAt = v.VerifiedAt
				_ = w.DB.SaveBackupRun(ctx, source)
			}
			return e
		}
		if m.Kind != "database" {
			return errors.New("Instance recovery is an offline CLI operation")
		}
		a, e := w.DB.Application(ctx, v.ApplicationID)
		if e != nil {
			return e
		}
		d, e := w.DB.Deployment(ctx, a.ActiveDeploymentID)
		if e != nil {
			return e
		}
		if m.Service != v.Config.Service {
			return errors.New("Choose the original database service")
		}
		v.Result = v.Config.Target
		e = runtime.RestoreDatabase(ctx, d, v.Config, m, filepath.Join(work, "database.dump"))
		if e == nil {
			v.VerifiedAt = time.Now().Unix()
			source.VerifiedAt = v.VerifiedAt
			_ = w.DB.SaveBackupRun(ctx, source)
		}
		return e
	}
	key, recipient, e := w.RecoveryKey(ctx)
	if e != nil {
		return e
	}
	m := protection.Manifest{Kind: v.Config.Kind, Instance: runtime.Instance, Application: v.ApplicationID, Service: v.Config.Service, Engine: v.Config.Engine, Database: v.Config.Database, Volume: v.Config.Volume, CreatedAt: time.Now().Unix()}
	files := map[string]string{}
	switch v.Config.Kind {
	case "instance":
		m.Consistency = "SQLite online snapshot; original encryption key; certificate-state snapshot"
		dbfile := filepath.Join(work, "shipper.db")
		if e = w.DB.Snapshot(ctx, dbfile); e != nil {
			return e
		}
		files["shipper.db"] = dbfile
		sources := filepath.Join(work, "sources.tar")
		if e = w.DB.BundleSources(ctx, dbfile, sources); e != nil {
			return e
		}
		files["sources.tar"] = sources
		keyfile := filepath.Join(work, "encryption.key")
		if e = copyPrivate(filepath.Join(w.DataDir, "encryption.key"), keyfile); e != nil {
			return e
		}
		files["encryption.key"] = keyfile
		proxy, e := runtime.EffectiveProxy(ctx)
		if e != nil {
			return e
		}
		if proxy.DashboardContainer != "" {
			proxy.DashboardUpstream = "auto"
		}
		raw, e := json.Marshal(proxy)
		if e != nil {
			return e
		}
		proxyfile := filepath.Join(work, "proxy.json")
		if e = os.WriteFile(proxyfile, raw, 0600); e != nil {
			return e
		}
		files["proxy.json"] = proxyfile
		acme, e := runtime.BackupACME(ctx)
		if e != nil {
			return e
		}
		if len(acme) > 0 {
			p := filepath.Join(work, "acme.json")
			if e = os.WriteFile(p, acme, 0600); e != nil {
				return e
			}
			files["acme.json"] = p
		}
	case "volume":
		m.Consistency = "Cold archive; all mounted containers stopped"
		p := filepath.Join(work, "volume.tar")
		if v.Config.Engine == "redis" {
			a, err := w.DB.Application(ctx, v.ApplicationID)
			if err != nil {
				return err
			}
			d, err := w.DB.Deployment(ctx, a.ActiveDeploymentID)
			if err != nil {
				return err
			}
			if d.Config.Database == nil || d.Config.Database.Engine != "redis" {
				return errors.New("Choose the original managed Redis resource")
			}
			m.Consistency = "Native Redis RDB snapshot"
			e = runtime.DumpRedisVolume(ctx, d, p)
		} else {
			e = runtime.DumpVolume(ctx, v.ApplicationID, v.ID, v.Config.Volume, p)
		}
		if e != nil {
			return e
		}
		files["volume.tar"] = p
	case "database":
		a, e := w.DB.Application(ctx, v.ApplicationID)
		if e != nil {
			return e
		}
		d, e := w.DB.Deployment(ctx, a.ActiveDeploymentID)
		if e != nil {
			return e
		}
		p := filepath.Join(work, "database.dump")
		m.EngineVersion, e = runtime.DumpDatabase(ctx, d, v.Config, p)
		if e != nil {
			return e
		}
		m.Consistency = "Native PostgreSQL snapshot"
		if v.Config.Engine != "postgres" {
			m.Consistency = "Native database dump with global read lock; schema changes must be paused"
		}
		files["database.dump"] = p
	default:
		return errors.New("Unsupported backup kind")
	}
	tmp := filepath.Join(work, "backup.age")
	f, e := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	_, e = protection.Seal(f, recipient, m, files)
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	verify, e := os.MkdirTemp(work, "verify-")
	if e != nil {
		return e
	}
	f, e = os.Open(tmp)
	if e != nil {
		return e
	}
	_, e = protection.Open(f, key, verify)
	f.Close()
	if e != nil {
		return e
	}
	v.Bytes, v.Checksum, e = protection.Checksum(tmp)
	if e != nil {
		return e
	}
	if e = os.Rename(tmp, w.BackupPath(v.ID)); e != nil {
		return e
	}
	if v.Config.Destination != "" {
		destination, e := w.backupRunDestination(ctx, v.Config)
		if e != nil {
			return e
		}
		key := destination.Key(runtime.Instance, v.ID)
		if e = destination.Put(ctx, key, w.BackupPath(v.ID)); e != nil {
			return e
		}
		v.ObjectKey = key
	}
	v.Result = "Encrypted archive verified"
	if m.EngineVersion != "" {
		v.Result += "; " + m.EngineVersion
	}
	return nil
}
func copyPrivate(src, dst string) error {
	in, e := os.Open(src)
	if e != nil {
		return e
	}
	defer in.Close()
	out, e := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	_, e = io.Copy(out, in)
	ce := out.Close()
	if e != nil {
		return e
	}
	return ce
}
func (w *Worker) retainBackups(ctx context.Context, current store.BackupRun) {
	if current.PlanID == "" {
		return
	}
	runs, e := w.DB.BackupRetention(ctx, current.PlanID, current.Config.Keep)
	if e != nil {
		return
	}
	for _, v := range runs {
		if v.PlanID != current.PlanID || v.Operation != "backup" || v.State != "succeeded" {
			continue
		}
		if v.ObjectKey != "" {
			d, e := w.backupRunDestination(ctx, v.Config)
			runtime := w.Runtime.(*DockerRuntime)
			if e != nil || v.ObjectKey != d.Key(runtime.Instance, v.ID) {
				continue
			}
			if e = d.Delete(ctx, v.ObjectKey); e != nil {
				continue
			}
		}
		if e = os.Remove(w.BackupPath(v.ID)); e != nil && !os.IsNotExist(e) {
			continue
		}
		v.State = "expired"
		v.Result = "Removed by retention"
		_ = w.DB.SaveBackupRun(ctx, v)
	}
}

// RecoverInstance stages and validates recovery before installing into an empty target.
func RecoverInstance(ctx context.Context, archive, keyfile, target string) error {
	target, e := filepath.Abs(target)
	if e != nil {
		return e
	}
	if info, e := os.Lstat(target); e == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("Recovery target must be an empty directory")
		}
		entries, e := os.ReadDir(target)
		if e != nil {
			return e
		}
		if len(entries) > 0 {
			return errors.New("Recovery never overwrites an existing installation; choose an empty directory")
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	parent := filepath.Dir(target)
	if e = os.MkdirAll(parent, 0700); e != nil {
		return e
	}
	stage, e := os.MkdirTemp(parent, ".shipper-recovery-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(stage)
	key, e := os.ReadFile(keyfile)
	if e != nil {
		return e
	}
	if len(key) > 1024 {
		return errors.New("Invalid recovery key file")
	}
	f, e := os.Open(archive)
	if e != nil {
		return e
	}
	m, e := protection.Open(f, string(key), stage)
	f.Close()
	if e != nil {
		return e
	}
	if m.Kind != "instance" {
		return errors.New("Choose an instance recovery archive")
	}
	for _, v := range m.Files {
		if v.Name != "shipper.db" && v.Name != "encryption.key" && v.Name != "acme.json" && v.Name != "proxy.json" && v.Name != "sources.tar" {
			return errors.New("Unexpected instance recovery file")
		}
	}
	for _, name := range []string{"shipper.db", "encryption.key"} {
		if _, e = os.Stat(filepath.Join(stage, name)); e != nil {
			return fmt.Errorf("Recovery is missing %s", name)
		}
	}
	if _, e = os.Stat(filepath.Join(stage, "proxy.json")); e == nil {
		if e = os.Rename(filepath.Join(stage, "proxy.json"), filepath.Join(stage, "proxy-recovery.json")); e != nil {
			return e
		}
	}
	if e = store.PrepareRecovery(stage); e != nil {
		return e
	}
	if e = store.RestoreSourceBundle(stage); e != nil {
		return e
	}
	db, e := store.Open(stage)
	if e != nil {
		return e
	}
	id, e := db.Setting(ctx, "instance_id", "")
	if e == nil {
		e = db.VerifyRecoveredSources(ctx)
	}
	if e == nil {
		_, e = db.PrivateSetting(ctx, "recovery_identity", "")
	}
	db.Close()
	if e != nil || id == "" || id != m.Instance {
		return errors.New("Recovery instance identity does not match")
	}
	if _, e = os.Stat(filepath.Join(stage, "acme.json")); e == nil {
		if e = os.Rename(filepath.Join(stage, "acme.json"), filepath.Join(stage, "recovered-acme.json")); e != nil {
			return e
		}
	} // Copy only validated installation files; keep the pre-existing empty directory valid on Windows.
	if e = os.MkdirAll(target, 0700); e != nil {
		return e
	}
	entries, e := os.ReadDir(target)
	if e != nil || len(entries) > 0 {
		return errors.New("Recovery target changed while validating the archive")
	}
	return installRecovery(stage, target)
}
func installRecovery(stage, target string) error {
	entries, e := os.ReadDir(stage)
	if e != nil {
		return e
	}
	for _, v := range entries {
		if !v.IsDir() {
			if e = copyPrivate(filepath.Join(stage, v.Name()), filepath.Join(target, v.Name())); e != nil {
				return e
			}
		}
	}
	return nil
}
func (w *Worker) ImportBackup(ctx context.Context, app, identity string, src io.Reader) (store.BackupRun, error) {
	var v store.BackupRun
	root := filepath.Join(w.DataDir, "backups")
	if e := os.MkdirAll(filepath.Join(root, "archives"), 0700); e != nil {
		return v, e
	}
	work, e := os.MkdirTemp(root, "import-")
	if e != nil {
		return v, e
	}
	defer os.RemoveAll(work)
	file := filepath.Join(work, "import.age")
	f, e := os.OpenFile(file, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return v, e
	}
	n, e := io.Copy(f, io.LimitReader(src, protection.MaxBytes+(16<<20)+1))
	ce := f.Close()
	if e != nil {
		return v, e
	}
	if ce != nil {
		return v, ce
	}
	if n > protection.MaxBytes+(16<<20) {
		return v, errors.New("Archive exceeds the 2 GiB limit")
	}
	if identity == "" {
		identity, _, e = w.RecoveryKey(ctx)
		if e != nil {
			return v, e
		}
	}
	f, e = os.Open(file)
	if e != nil {
		return v, e
	}
	m, e := protection.Open(f, identity, work)
	f.Close()
	if e != nil {
		return v, e
	}
	if m.Application != app || (m.Kind != "database" && m.Kind != "volume") {
		return v, errors.New("Choose a database or volume archive from this application")
	}
	spec := store.BackupSpec{NativeImport: m.Consistency == "Native dump import", Kind: m.Kind, Service: m.Service, Engine: m.Engine, Database: m.Database, Volume: m.Volume, Host: "127.0.0.1", TLS: "prefer", Keep: 7, Identity: identity} // Preserve credentials from the matching local plan, never from untrusted archive metadata.
	plans, e := w.DB.BackupPlans(ctx, app)
	if e != nil {
		return v, e
	}
	for _, p := range plans {
		if p.Config.Kind == spec.Kind && p.Config.Service == spec.Service && p.Config.Database == spec.Database {
			spec.User = p.Config.User
			spec.Password = p.Config.Password
			spec.TLS = p.Config.TLS
			spec.Port = p.Config.Port
			break
		}
	}
	v, e = w.DB.QueueBackup(ctx, store.BackupRun{ApplicationID: app, Operation: "import", Config: spec})
	if e != nil {
		return v, e
	}
	v.Bytes, v.Checksum, e = protection.Checksum(file)
	if e == nil {
		e = os.Rename(file, w.BackupPath(v.ID))
	}
	v.Operation = "backup"
	v.State = "succeeded"
	v.Result = "Imported encrypted archive verified"
	v.FinishedAt = time.Now().Unix()
	if e != nil {
		v.State = "failed"
		v.Error = "Archive import failed"
	}
	if se := w.DB.FinishBackupImport(ctx, v); e == nil {
		e = se
	}
	w.changed()
	return v, e
}
