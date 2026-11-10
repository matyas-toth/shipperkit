package httpapi

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"shipper/internal/auth"
	"shipper/internal/deploy"
	"shipper/internal/protection"
	"shipper/internal/store"
)

func (s *Server) protectionRoutes(r chi.Router) {
	// Reading protection state and managing per-application plans belongs to
	// developers; archive download, restore, recovery keys and destinations are
	// administrator capabilities.
	r.Group(func(r chi.Router) {
		r.Use(s.requireRole(store.RoleDeveloper))
		for _, base := range []string{"/protection", "/applications/{id}/protection"} {
			r.Get(base, s.protectionInfo)
			r.With(s.csrf, s.limit).Post(base+"/plans", s.saveBackupPlan)
			r.With(s.csrf, s.limit).Post(base+"/plans/{plan}/run", s.runBackupPlan)
			r.With(s.csrf, s.limit).Post(base+"/plans/{plan}/delete", s.deleteBackupPlan)
			r.With(s.csrf, s.limit).Post(base+"/schedule-preview", s.schedulePreview)
		}
	})
	r.Group(func(r chi.Router) {
		r.Use(s.requireRole(store.RoleAdmin))
		for _, base := range []string{"/protection", "/applications/{id}/protection"} {
			r.With(s.csrf, s.limit).Post(base+"/runs/{run}/download", s.downloadBackup)
			r.Get(base+"/runs/{run}/download", s.downloadBackup)
			r.Head(base+"/runs/{run}/download", s.downloadBackup)
			r.With(s.csrf, s.limit).Post(base+"/runs/{run}/restore", s.restoreBackup)
		}
		r.With(s.csrf, s.limit).Post("/protection/key", s.recoveryKey)
		r.With(s.csrf, s.limit).Post("/protection/destinations", s.saveBackupDestination)
		r.With(s.csrf, s.limit).Post("/applications/{id}/protection/import", s.importBackup)
		r.With(s.csrf, s.limit).Post("/applications/{id}/protection/volumes/{volume}/delete", s.deleteProtectedVolume)
		r.With(s.csrf, s.limit).Post("/protection/volumes/{volume}/delete", s.deleteProtectedVolume)
	})
}
func (s *Server) backupScope(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := chi.URLParam(r, "id")
	if id != "" {
		if _, e := s.db.Application(r.Context(), id); e != nil {
			s.resourceError(w, e)
			return "", false
		}
	}
	if s.cfg.Worker == nil {
		s.problem(w, 503, "unavailable", "Protection is unavailable")
		return "", false
	}
	return id, true
}
func (s *Server) protectionInfo(w http.ResponseWriter, r *http.Request) {
	id, ok := s.backupScope(w, r)
	if !ok {
		return
	}
	plans, e := s.db.BackupPlans(r.Context(), id)
	if e != nil {
		s.internal(w, e)
		return
	}
	for i := range plans {
		plans[i].Config.Password = ""
		plans[i].Config.Identity = ""
		plans[i].Config.Remote = nil
	}
	runs, e := s.db.BackupRuns(r.Context(), id)
	if e != nil {
		s.internal(w, e)
		return
	}
	dest, e := s.db.BackupDestinations(r.Context())
	if e != nil {
		s.internal(w, e)
		return
	}
	for i := range dest {
		dest[i].AccessKey = ""
		dest[i].SecretKey = ""
	}
	_, recipient, e := s.cfg.Worker.RecoveryKey(r.Context())
	if e != nil {
		s.internal(w, e)
		return
	}
	volumes := []deploy.ProtectedVolume{}
	volumeError := ""
	revision := ""
	if id != "" {
		a, e := s.db.Application(r.Context(), id)
		if e != nil {
			s.resourceError(w, e)
			return
		}
		revision = s.db.ConfigRevision(a)
	}
	if runtime, ok := s.cfg.Worker.Runtime.(*deploy.DockerRuntime); ok {
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()
		volumes, e = runtime.ProtectedVolumes(ctx, id)
		if e != nil {
			volumeError = "Docker volume inventory is unavailable"
			volumes = []deploy.ProtectedVolume{}
		}
		for i := range volumes {
			volumes[i].ApplicationName = s.db.ProtectionApplicationName(ctx, volumes[i].ApplicationID)
		}
	}
	s.json(w, 200, map[string]any{"plans": plans, "runs": runs, "destinations": dest, "recipient": recipient, "volumes": volumes, "volumeError": volumeError, "revision": revision})
}
func (s *Server) recoveryKey(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Worker == nil {
		s.problem(w, 503, "unavailable", "Protection is unavailable")
		return
	}
	var in struct {
		Confirm string `json:"confirm"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	if in.Confirm != "Download recovery key" {
		s.problem(w, 422, "confirmation", "Confirm recovery key download")
		return
	}
	key, _, e := s.cfg.Worker.RecoveryKey(r.Context())
	if e != nil {
		s.internal(w, e)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="shipper-recovery-key.txt"`)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(key + "\n"))
}
func (s *Server) saveBackupPlan(w http.ResponseWriter, r *http.Request) {
	id, ok := s.backupScope(w, r)
	if !ok {
		return
	}
	var p store.BackupPlan
	if !s.decode(w, r, &p) {
		return
	}
	p.ApplicationID = id
	p.Name = strings.TrimSpace(p.Name)
	if !validName(p.Name) {
		s.problem(w, 422, "invalid_name", "Enter a backup name")
		return
	}
	if p.ID != "" {
		items, e := s.db.BackupPlans(r.Context(), id)
		if e != nil {
			s.internal(w, e)
			return
		}
		found := false
		for _, old := range items {
			if old.ID == p.ID {
				found = true
				if p.Config.Password == "" {
					p.Config.Password = old.Config.Password
				}
			}
		}
		if !found {
			s.problem(w, 404, "not_found", "Backup plan was not found")
			return
		}
	} else {
		p.ID = auth.Token()[:24]
	}
	if (id == "") != (p.Config.Kind == "instance") {
		s.problem(w, 422, "scope", "Choose a backup type for this scope")
		return
	}
	p.Config.Remote = nil
	p.Config.Target = ""
	p.Config.Identity = ""
	if p.Config.Kind == "database" && (p.Config.User == "" || p.Config.Password == "") {
		a, e := s.db.Application(r.Context(), id)
		if e != nil {
			s.resourceError(w, e)
			return
		}
		d, e := s.db.Deployment(r.Context(), a.ActiveDeploymentID)
		if e != nil {
			s.resourceError(w, e)
			return
		}
		runtime, ok := s.cfg.Worker.Runtime.(*deploy.DockerRuntime)
		if !ok {
			s.problem(w, 503, "unavailable", "Docker protection is unavailable")
			return
		}
		if e = runtime.InferBackupCredentials(r.Context(), d, &p.Config); e != nil {
			s.problem(w, 422, "credentials", e.Error())
			return
		}
	}
	if e := deploy.ValidateBackupSpec(&p.Config); e != nil {
		s.problem(w, 422, "invalid_backup", e.Error())
		return
	}
	if p.Timezone == "" {
		p.Timezone = "UTC"
	}
	p.NextAt = 0
	if p.Schedule != "" {
		next, e := deploy.NextSchedule(p.Schedule, p.Timezone, time.Now())
		if e != nil {
			s.problem(w, 422, "invalid_schedule", e.Error())
			return
		}
		p.NextAt = next.Unix()
	}
	if p.Config.Destination != "" {
		items, e := s.db.BackupDestinations(r.Context())
		found := false
		for _, d := range items {
			if d.ID == p.Config.Destination {
				found = true
			}
		}
		if e != nil || !found {
			s.problem(w, 422, "invalid_destination", "Choose an existing destination")
			return
		}
	}
	if e := s.db.SaveBackupPlan(r.Context(), p); e != nil {
		s.internal(w, e)
		return
	}
	s.hub.Publish()
	p.Config.Password = ""
	s.json(w, 200, p)
}
func (s *Server) runBackupPlan(w http.ResponseWriter, r *http.Request) {
	id, ok := s.backupScope(w, r)
	if !ok {
		return
	}
	items, e := s.db.BackupPlans(r.Context(), id)
	if e != nil {
		s.internal(w, e)
		return
	}
	for _, p := range items {
		if p.ID == chi.URLParam(r, "plan") {
			if !p.Enabled {
				s.problem(w, 422, "disabled_plan", "Enable this backup plan before running it")
				return
			}
			v, e := s.db.QueueBackup(r.Context(), store.BackupRun{PlanID: p.ID, ApplicationID: id, Operation: "backup", Config: p.Config})
			if e != nil {
				s.resourceError(w, e)
				return
			}
			s.cfg.Worker.Trigger()
			s.json(w, 202, v)
			return
		}
	}
	s.problem(w, 404, "not_found", "Backup plan was not found")
}
func (s *Server) deleteBackupPlan(w http.ResponseWriter, r *http.Request) {
	id, ok := s.backupScope(w, r)
	if !ok {
		return
	}
	if e := s.db.DeleteBackupPlan(r.Context(), id, chi.URLParam(r, "plan")); e != nil {
		s.internal(w, e)
		return
	}
	s.hub.Publish()
	s.json(w, 200, map[string]bool{"deleted": true})
}
func (s *Server) downloadBackup(w http.ResponseWriter, r *http.Request) {
	id, ok := s.backupScope(w, r)
	if !ok {
		return
	}
	v, e := s.db.BackupRun(r.Context(), chi.URLParam(r, "run"))
	if e != nil {
		s.resourceError(w, e)
		return
	}
	if v.ApplicationID != id || v.Operation != "backup" || v.State != "succeeded" {
		s.problem(w, 404, "not_found", "Completed backup was not found")
		return
	}
	file := s.cfg.Worker.BackupPath(v.ID)
	f, e := os.Open(file)
	if e != nil {
		s.problem(w, 404, "missing_file", "Local copy is unavailable; download the encrypted object from its configured S3 destination")
		return
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		s.internal(w, e)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="shipper-`+v.ID+`.age"`)
	http.ServeContent(w, r, "backup.age", info.ModTime(), f)
}
func (s *Server) restoreBackup(w http.ResponseWriter, r *http.Request) {
	id, ok := s.backupScope(w, r)
	if !ok {
		return
	}
	if id == "" {
		s.problem(w, 422, "offline_recovery", "Use shipper restore with an empty data directory for instance recovery")
		return
	}
	var in struct {
		Target   string `json:"target"`
		Confirm  string `json:"confirm"`
		Revision string `json:"revision"`
		Review   bool   `json:"review"`
		Identity string `json:"identity"`
		Password string `json:"password"`
		User     string `json:"user"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	a, e := s.db.Application(r.Context(), id)
	if e != nil {
		s.resourceError(w, e)
		return
	}
	v, e := s.db.BackupRun(r.Context(), chi.URLParam(r, "run"))
	if e != nil {
		s.resourceError(w, e)
		return
	}
	if v.ApplicationID != id || v.State != "succeeded" || v.Operation != "backup" {
		s.problem(w, 422, "invalid_backup", "Choose a completed backup from this application")
		return
	}
	revision := s.db.ConfigRevision(a) + ":" + v.ID
	if in.Review {
		s.json(w, 200, map[string]any{"revision": revision, "kind": v.Config.Kind, "engine": v.Config.Engine, "database": v.Config.Database, "service": v.Config.Service, "volume": v.Config.Volume})
		return
	}
	if in.Revision != revision || in.Confirm != a.Name {
		s.problem(w, 409, "review_changed", "Review the current application and enter its name to restore a copy")
		return
	}
	c := v.Config
	c.Target = in.Target
	if in.Identity != "" {
		c.Identity = in.Identity
	}
	if in.User != "" {
		c.User = in.User
	}
	if in.Password != "" {
		c.Password = in.Password
	}
	if c.Kind == "database" {
		if e := deploy.ValidateBackupSpec(&c); e != nil {
			s.problem(w, 422, "credentials", e.Error())
			return
		}
	}
	if c.Kind == "database" && (!deploy.ValidRecoveryDatabase(c.Target) || c.Target == c.Database) {
		s.problem(w, 422, "invalid_target", "Enter a new database name")
		return
	}
	job, e := s.db.QueueBackup(r.Context(), store.BackupRun{ApplicationID: id, Operation: "restore", Config: c, SourceID: v.ID})
	if e != nil {
		s.resourceError(w, e)
		return
	}
	s.cfg.Worker.Trigger()
	s.json(w, 202, job)
}
func (s *Server) saveBackupDestination(w http.ResponseWriter, r *http.Request) {
	var d store.BackupDestination
	if !s.decode(w, r, &d) {
		return
	}
	d.Name = strings.TrimSpace(d.Name)
	if !validName(d.Name) {
		s.problem(w, 422, "invalid_name", "Enter a destination name")
		return
	}
	if d.ID == "" {
		d.ID = auth.Token()[:24]
	} else {
		items, e := s.db.BackupDestinations(r.Context())
		if e != nil {
			s.internal(w, e)
			return
		}
		found := false
		for _, v := range items {
			if v.ID == d.ID {
				found = true
				if d.AccessKey == "" {
					d.AccessKey = v.AccessKey
				}
				if d.SecretKey == "" {
					d.SecretKey = v.SecretKey
				}
			}
		}
		if !found {
			s.problem(w, 404, "not_found", "Destination was not found")
			return
		}
	}
	client := protection.S3{Endpoint: d.Endpoint, Region: d.Region, Bucket: d.Bucket, Prefix: d.Prefix, AccessKey: d.AccessKey, SecretKey: d.SecretKey, Private: d.Private}
	if e := client.Validate(); e != nil {
		s.problem(w, 422, "invalid_destination", e.Error())
		return
	}
	if e := s.db.SaveBackupDestination(r.Context(), d); e != nil {
		s.internal(w, e)
		return
	}
	s.hub.Publish()
	d.AccessKey = ""
	d.SecretKey = ""
	s.json(w, 200, d)
}
func (s *Server) importBackup(w http.ResponseWriter, r *http.Request) {
	id, ok := s.backupScope(w, r)
	if !ok {
		return
	}
	select {
	case s.protectionSlots <- struct{}{}:
		defer func() { <-s.protectionSlots }()
	default:
		s.problem(w, 429, "upload_busy", "Another backup import is in progress")
		return
	}
	_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(30 * time.Minute))
	r.Body = http.MaxBytesReader(w, r.Body, protection.MaxBytes+(17<<20))
	reader, e := r.MultipartReader()
	if e != nil {
		s.problem(w, 400, "invalid_upload", "Upload an encrypted .age archive")
		return
	}
	var identity string
	part, e := reader.NextPart()
	if e != nil || part.FormName() != "identity" {
		s.problem(w, 400, "invalid_upload", "Send the recovery key before the encrypted archive")
		return
	}
	raw, e := readBounded(part, 1024)
	if e != nil {
		s.problem(w, 400, "invalid_upload", e.Error())
		return
	}
	identity = string(raw)
	part, e = reader.NextPart()
	if e != nil || part.FormName() != "archive" {
		s.problem(w, 400, "invalid_upload", "Send one encrypted recovery archive")
		return
	}
	v, e := s.cfg.Worker.ImportBackup(r.Context(), id, identity, part)
	if e != nil {
		s.problem(w, 422, "invalid_archive", e.Error())
		return
	}
	s.json(w, 201, v)
}
func (s *Server) deleteProtectedVolume(w http.ResponseWriter, r *http.Request) {
	id, ok := s.backupScope(w, r)
	if !ok {
		return
	}
	var in struct {
		Confirm string `json:"confirm"`
	}
	if !s.decode(w, r, &in) {
		return
	}
	name := chi.URLParam(r, "volume")
	if id == "" {
		runtime, ok := s.cfg.Worker.Runtime.(*deploy.DockerRuntime)
		if !ok {
			s.problem(w, 503, "unavailable", "Docker protection is unavailable")
			return
		}
		var e error
		id, e = runtime.RetainedVolumeApplication(r.Context(), name)
		if e != nil {
			s.problem(w, 422, "volume_ownership", e.Error())
			return
		}
	}
	if in.Confirm != name {
		s.problem(w, 422, "confirmation", "Enter the full retained volume name to delete its data")
		return
	}
	v, e := s.db.QueueBackup(r.Context(), store.BackupRun{ApplicationID: id, Operation: "delete-volume", Config: store.BackupSpec{Volume: name}})
	if e != nil {
		s.resourceError(w, e)
		return
	}
	s.cfg.Worker.Trigger()
	s.json(w, 202, v)
}
func readBounded(r interface{ Read([]byte) (int, error) }, limit int64) ([]byte, error) {
	return protection.ReadBounded(r, limit)
}
