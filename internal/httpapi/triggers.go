package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"io"
	"net/http"
	"path"
	"regexp"
	"shipper/internal/auth"
	"shipper/internal/store"
	"strings"
)

func matchWatch(pattern, file string) bool {
	a, b := strings.Split(pattern, "/"), strings.Split(file, "/")
	memo := map[[2]int]bool{}
	seen := map[[2]int]bool{}
	var match func(int, int) bool
	match = func(i, j int) (result bool) {
		key := [2]int{i, j}
		if seen[key] {
			return memo[key]
		}
		seen[key] = true
		defer func() { memo[key] = result }()
		if i == len(a) {
			return j == len(b)
		}
		if a[i] == "**" {
			return match(i+1, j) || (j < len(b) && match(i, j+1))
		}
		if j >= len(b) {
			return false
		}
		ok, _ := path.Match(a[i], b[j])
		return ok && match(i+1, j+1)
	}
	return match(0, 0)
}
func (s *Server) githubWebhook(w http.ResponseWriter, r *http.Request) {
	c, e := s.db.Connection(r.Context(), chi.URLParam(r, "connection"))
	if e != nil || c.Secret.WebhookSecret == "" {
		s.problem(w, 404, "not_found", "Webhook not found")
		return
	}
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
	if e != nil {
		s.problem(w, 413, "payload_limit", "Webhook is too large")
		return
	}
	signature, e := hex.DecodeString(strings.TrimPrefix(r.Header.Get("X-Hub-Signature-256"), "sha256="))
	mac := hmac.New(sha256.New, []byte(c.Secret.WebhookSecret))
	mac.Write(raw)
	if e != nil || !hmac.Equal(signature, mac.Sum(nil)) {
		s.problem(w, 403, "invalid_signature", "Webhook signature did not match")
		return
	}
	delivery := r.Header.Get("X-GitHub-Delivery")
	if !keyPattern.MatchString(delivery) {
		s.problem(w, 422, "invalid_delivery", "Webhook delivery ID is invalid")
		return
	}
	if r.Header.Get("X-GitHub-Event") == "ping" {
		s.json(w, 200, map[string]bool{"accepted": true})
		return
	}
	if r.Header.Get("X-GitHub-Event") != "push" {
		s.json(w, 202, map[string]bool{"ignored": true})
		return
	}
	var event struct {
		Ref          string
		After        string
		Deleted      bool
		Size         int
		Installation struct{ ID int64 }
		Repository   struct {
			ID            int64
			DefaultBranch string `json:"default_branch"`
		}
		HeadCommit struct{ Message string } `json:"head_commit"`
		Commits    []struct{ Added, Modified, Removed []string }
	}
	if json.Unmarshal(raw, &event) != nil || !regexp.MustCompile(`^[a-fA-F0-9]{40}$`).MatchString(event.After) {
		s.problem(w, 422, "invalid_event", "Push payload is invalid")
		return
	}
	if c.Kind == "github_app" && event.Installation.ID != c.Secret.InstallationID {
		s.problem(w, 403, "invalid_installation", "Webhook installation did not match")
		return
	}
	jobs := []store.DeployIntent{}
	if !event.Deleted && event.After != strings.Repeat("0", 40) && !strings.Contains(strings.ToLower(event.HeadCommit.Message), "[skip shipper]") {
		apps, e := s.db.Applications(r.Context())
		if e != nil {
			s.internal(w, e)
			return
		}
		for _, a := range apps {
			cfg := a.Config
			if !cfg.AutoDeploy.Enabled || cfg.SourceID != c.ID || cfg.RepositoryID != event.Repository.ID {
				continue
			}
			branch := cfg.Branch
			if branch == "" {
				branch = event.Repository.DefaultBranch
			}
			if event.Ref != "refs/heads/"+branch {
				continue
			}
			matched := len(cfg.AutoDeploy.Paths) == 0 || event.Size > len(event.Commits) || len(event.Commits) == 0
			for _, commit := range event.Commits {
				for _, file := range append(append(commit.Added, commit.Modified...), commit.Removed...) {
					for _, pattern := range cfg.AutoDeploy.Paths {
						if matchWatch(pattern, file) {
							matched = true
						}
					}
				}
			}
			if matched {
				jobs = append(jobs, store.DeployIntent{ApplicationID: a.ID, Revision: strings.ToLower(event.After), Actor: "github:push"})
			}
		}
	}
	accepted, e := s.db.AcceptWebhook(r.Context(), c.ID, delivery, jobs)
	if e != nil {
		s.internal(w, e)
		return
	}
	if s.cfg.Worker != nil {
		s.cfg.Worker.Trigger()
	}
	s.json(w, 202, map[string]any{"accepted": accepted, "applications": len(jobs)})
}
func (s *Server) deployToken(w http.ResponseWriter, r *http.Request) {
	a, e := s.db.Application(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		s.resourceError(w, e)
		return
	}
	var input struct {
		Revoke bool `json:"revoke"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	token := ""
	hash := ""
	if !input.Revoke {
		token = auth.Token()
		hash = auth.Hash(token)
	}
	if e = s.db.RotateDeployToken(r.Context(), a.ID, hash); e != nil {
		s.internal(w, e)
		return
	}
	s.json(w, 200, map[string]string{"token": token, "path": "/api/v1/applications/" + a.ID + "/deploy"})
}
func (s *Server) tokenDeploy(w http.ResponseWriter, r *http.Request) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") || len(header) != 71 {
		s.problem(w, 401, "invalid_token", "A deploy token is required")
		return
	}
	id, e := s.db.DeployToken(r.Context(), auth.Hash(strings.TrimPrefix(header, "Bearer ")))
	if e != nil || id != chi.URLParam(r, "id") {
		s.problem(w, 403, "token_scope", "This token cannot deploy this application")
		return
	}
	if !s.mutationAvailable(w) {
		return
	}
	a, e := s.db.Application(r.Context(), id)
	if e != nil {
		s.resourceError(w, e)
		return
	}
	var input struct {
		Revision string `json:"revision"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	if input.Revision != "" && !regexp.MustCompile(`^(?:[a-fA-F0-9]{40}|[a-fA-F0-9]{64})$`).MatchString(input.Revision) {
		s.problem(w, 422, "invalid_revision", "Use a full commit SHA")
		return
	}
	key := r.Header.Get("Idempotency-Key")
	if key != "" && !keyPattern.MatchString(key) {
		s.problem(w, 422, "invalid_idempotency_key", "Use a valid idempotency key")
		return
	}
	d, e := s.db.Enqueue(r.Context(), a, store.Deployment{ID: auth.Token()[:24], Operation: "deploy", Actor: "deploy-token", Commit: input.Revision}, key, false)
	if e != nil {
		s.problem(w, 409, "operation_conflict", e.Error())
		return
	}
	s.cfg.Worker.Trigger()
	s.json(w, 202, store.PublicDeployment(d))
}
func (s *Server) cancelDeployment(w http.ResponseWriter, r *http.Request) {
	if e := s.db.CancelDeployment(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "deployment")); e != nil {
		s.problem(w, 409, "not_cancellable", e.Error())
		return
	}
	s.hub.Publish()
	s.json(w, 202, map[string]bool{"cancelRequested": true})
}
func (s *Server) retryDeployment(w http.ResponseWriter, r *http.Request) {
	if !s.mutationAvailable(w) {
		return
	}
	a, e := s.db.Application(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		s.resourceError(w, e)
		return
	}
	old, e := s.db.Deployment(r.Context(), chi.URLParam(r, "deployment"))
	if e != nil || old.ApplicationID != a.ID {
		s.problem(w, 404, "not_found", "Deployment was not found")
		return
	}
	if (old.State != "failed" && old.State != "cancelled") || (old.Operation != "deploy" && old.Operation != "apply") {
		s.problem(w, 409, "not_retryable", "This operation cannot be retried")
		return
	}
	if old.Commit == "" && old.Config.Method != "image" && old.Config.Method != "inline" {
		s.problem(w, 409, "revision_unknown", "Redeploy to resolve a revision for this source")
		return
	}
	a.Config = old.Config
	if a.Config.Method == "image" && old.Digest != "" {
		a.Config.Image = old.Digest
	}
	s.enqueueJob(w, r, a, store.Deployment{Operation: "deploy", Commit: old.Commit}, false)
}
