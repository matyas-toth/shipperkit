package httpapi

import (
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"shipper/internal/auth"
	"shipper/internal/deploy"
	"shipper/internal/protection"
)

// restoreInstance stages a validated instance archive on the first-run screen.
// The next start installs it before the store opens, so live instances are never
// rewritten underneath running applications.
func (s *Server) restoreInstance(w http.ResponseWriter, r *http.Request) {
	configured, err := s.db.Configured(r.Context())
	if err != nil {
		s.internal(w, err)
		return
	}
	if configured {
		s.problem(w, 409, "setup_complete", "Shipper already has an administrator")
		return
	}
	claim := s.cfg.SetupToken
	if s.cfg.SetupClaim != nil {
		claim, err = s.cfg.SetupClaim()
		if err != nil {
			s.problem(w, 403, "setup_token_expired", "Get a fresh setup token on the server")
			return
		}
	}
	if claim == "" || !auth.Equal(r.Header.Get("X-Setup-Token"), claim) {
		s.problem(w, 403, "setup_token_invalid", "The setup token is invalid")
		return
	}
	kind, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if kind != "application/octet-stream" {
		s.problem(w, 415, "invalid_upload", "Send the encrypted archive as octet-stream")
		return
	}
	key := strings.TrimSpace(r.Header.Get("X-Recovery-Key"))
	if key == "" || len(key) > 1024 {
		s.problem(w, 422, "invalid_recovery_key", "Paste the recovery key that matches this archive")
		return
	}
	if s.cfg.DataDir == "" {
		s.problem(w, 503, "unavailable", "Instance recovery is unavailable")
		return
	}
	select {
	case s.protectionSlots <- struct{}{}:
		defer func() { <-s.protectionSlots }()
	default:
		s.problem(w, 429, "upload_busy", "Another archive operation is running")
		return
	}
	// The server's 15 second read timeout is far too short for a multi-gigabyte
	// recovery archive. Extend this request only, keeping the global timeout.
	_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(2 * time.Hour))
	r.Body = http.MaxBytesReader(w, r.Body, protection.MaxBytes+(64<<20))
	stage, err := os.MkdirTemp(s.cfg.DataDir, ".restore-")
	if err != nil {
		s.internal(w, err)
		return
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.RemoveAll(stage)
		}
	}()
	archive := filepath.Join(stage, "archive.age")
	file, err := os.OpenFile(archive, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		s.internal(w, err)
		return
	}
	size, copyErr := io.Copy(file, r.Body)
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil {
		if copyErr != nil && strings.Contains(copyErr.Error(), "request body too large") {
			s.problem(w, 413, "archive_too_large", "The archive exceeds the 2 GiB limit")
		} else {
			s.problem(w, 400, "invalid_upload", "The archive could not be read")
		}
		return
	}
	if size == 0 {
		s.problem(w, 422, "invalid_archive", "Choose an encrypted instance archive")
		return
	}
	identity := filepath.Join(stage, "identity.txt")
	if err = os.WriteFile(identity, []byte(key), 0600); err != nil {
		s.internal(w, err)
		return
	}
	if err = deploy.RecoverInstance(r.Context(), archive, identity, filepath.Join(stage, "instance")); err != nil {
		s.problem(w, 422, "invalid_archive", err.Error())
		return
	}
	_ = os.Remove(archive)
	_ = os.Remove(identity)
	if err = deploy.WritePendingRestore(s.cfg.DataDir, stage); err != nil {
		s.internal(w, err)
		return
	}
	keep = true
	s.json(w, 202, map[string]any{"restarting": s.cfg.Restart != nil})
	if s.cfg.Restart != nil {
		time.AfterFunc(600*time.Millisecond, s.cfg.Restart)
	}
}
