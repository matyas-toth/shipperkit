package httpapi

import (
	"context"
	"errors"
	"github.com/distribution/reference"
	"github.com/go-chi/chi/v5"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"shipper/internal/auth"
	"shipper/internal/deploy"
	"shipper/internal/store"
	"strconv"
	"strings"
)

func (s *Server) connections(w http.ResponseWriter, r *http.Request) {
	items, e := s.db.Connections(r.Context())
	if e != nil {
		s.internal(w, e)
		return
	}
	s.json(w, 200, map[string]any{"items": items})
}
func (s *Server) saveConnection(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ID     string                 `json:"id"`
		Name   string                 `json:"name"`
		Kind   string                 `json:"kind"`
		Host   string                 `json:"host"`
		Secret store.ConnectionSecret `json:"secret"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	c := store.Connection{ID: input.ID, Name: strings.TrimSpace(input.Name), Kind: input.Kind, Host: strings.ToLower(input.Host), Secret: input.Secret}
	if !validName(c.Name) {
		s.problem(w, 422, "invalid_name", "Enter a connection name")
		return
	}
	if c.ID == "" {
		c.ID = auth.Token()[:24]
	} else {
		previous, e := s.db.Connection(r.Context(), c.ID)
		if e != nil {
			s.resourceError(w, e)
			return
		}
		if previous.Kind != c.Kind {
			s.problem(w, 422, "invalid_kind", "Create a new connection to change its type")
			return
		}
		if c.Secret.Token == "" {
			c.Secret.Token = previous.Secret.Token
		}
		if c.Secret.PrivateKey == "" {
			c.Secret.PrivateKey = previous.Secret.PrivateKey
		}
		if c.Secret.Password == "" {
			c.Secret.Password = previous.Secret.Password
		}
		if c.Secret.AppID == 0 {
			c.Secret.AppID = previous.Secret.AppID
		}
		if c.Secret.InstallationID == 0 {
			c.Secret.InstallationID = previous.Secret.InstallationID
		}
		c.Secret.Slug = previous.Secret.Slug
		c.Secret.WebhookSecret = previous.Secret.WebhookSecret
	}
	switch c.Kind {
	case "github_token":
		c.Host = "github.com"
		if _, e := s.github.Repositories(r.Context(), c, 1); e != nil {
			s.problem(w, 422, "github_denied", e.Error())
			return
		}
	case "github_app":
		c.Host = "github.com"
		name, e := s.github.VerifyInstallation(r.Context(), c, c.Secret.InstallationID)
		if e != nil {
			s.problem(w, 422, "github_denied", e.Error())
			return
		}
		if c.Name == "" {
			c.Name = name
		}
	case "registry":
		if c.Host == "" || strings.ContainsAny(c.Host, "/@?# \r\n") || strings.Contains(c.Host, "://") {
			s.problem(w, 422, "invalid_registry", "Enter a registry hostname and optional port")
			return
		}
		host := c.Host
		if parsed, _, e := net.SplitHostPort(host); e == nil {
			host = parsed
		}
		ips, e := net.DefaultResolver.LookupNetIP(r.Context(), "ip", host)
		if e != nil {
			s.problem(w, 422, "invalid_registry", "The registry hostname could not be resolved")
			return
		}
		for _, ip := range ips {
			if !deploy.PublicAddress(ip) && !c.Secret.AllowPrivate {
				s.problem(w, 422, "private_registry", "Enable private registry access for this host explicitly")
				return
			}
		}
	default:
		s.problem(w, 422, "invalid_connection", "Choose GitHub or a container registry")
		return
	}
	if c.Secret.WebhookSecret == "" {
		c.Secret.WebhookSecret = auth.Token()
	}
	if e := s.db.SaveConnection(r.Context(), c); e != nil {
		s.internal(w, e)
		return
	}
	s.json(w, 200, c)
}
func (s *Server) deleteConnection(w http.ResponseWriter, r *http.Request) {
	if e := s.db.DeleteConnection(r.Context(), chi.URLParam(r, "connection")); e != nil {
		s.problem(w, 409, "connection_in_use", e.Error())
		return
	}
	s.json(w, 200, map[string]bool{"deleted": true})
}
func (s *Server) repositories(w http.ResponseWriter, r *http.Request) {
	c, e := s.db.Connection(r.Context(), chi.URLParam(r, "connection"))
	if e != nil {
		s.resourceError(w, e)
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	if page > 100 {
		s.problem(w, 422, "invalid_page", "Repository page is outside its range")
		return
	}
	items, e := s.github.Repositories(r.Context(), c, page)
	if e != nil {
		s.problem(w, 422, "github_denied", e.Error())
		return
	}
	s.json(w, 200, map[string]any{"items": items, "nextPage": func() int {
		if len(items) == 100 {
			return page + 1
		}
		return 0
	}()})
}
func (s *Server) sourceBranches(w http.ResponseWriter, r *http.Request) {
	var input struct {
		SourceID   string `json:"sourceId"`
		Repository string `json:"repository"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	if input.SourceID == "" {
		result, e := deploy.PublicBranches(r.Context(), input.Repository)
		if e != nil {
			s.problem(w, 422, "repository_unavailable", e.Error())
			return
		}
		s.json(w, 200, result)
		return
	}
	c, e := s.db.Connection(r.Context(), input.SourceID)
	if e != nil {
		s.resourceError(w, e)
		return
	}
	repo, e := s.github.Repository(r.Context(), c, input.Repository)
	if e != nil {
		s.problem(w, 422, "github_denied", e.Error())
		return
	}
	refs, e := s.github.Revisions(r.Context(), c, input.Repository)
	if e != nil {
		s.problem(w, 422, "github_denied", e.Error())
		return
	}
	names := []string{}
	for _, ref := range refs {
		names = append(names, ref.Name)
	}
	s.json(w, 200, map[string]any{"items": names, "revisions": refs, "default": repo.DefaultBranch})
}
func (s *Server) validateSource(ctx context.Context, c *store.Config) error {
	if p, _ := ctx.Value(automationKey).(*store.OperationToken); p != nil && len(p.Applications) > 0 {
		id, _ := ctx.Value(automationAppKey).(string)
		a, e := s.db.Application(ctx, id)
		if e != nil {
			return errors.New("Choose an authorized application")
		}
		normalize := func(v string) string {
			return strings.TrimSuffix(strings.TrimSuffix(strings.ToLower(strings.TrimSpace(v)), "/"), ".git")
		}
		if c.SourceID != "" && (c.SourceID != a.Config.SourceID || normalize(c.Repository) != normalize(a.Config.Repository)) {
			return errors.New("An application token can only use its existing private repository connection")
		}
		if c.RegistryID != "" && c.RegistryID != a.Config.RegistryID {
			return errors.New("An application token can only use its existing registry connection")
		}
		if c.RegistryID != "" && c.Method == "image" {
			next, err := deploy.ImageReference(c.Image)
			old, oldErr := deploy.ImageReference(a.Config.Image)
			if err != nil || oldErr != nil || next.Name() != old.Name() {
				return errors.New("An application token can only change tags or digests within its existing private image repository")
			}
		}
	}
	if c.LocalSource != "" {
		v, e := s.db.SourceSnapshot(ctx, c.LocalSource)
		if e != nil {
			return errors.New("Local source is unavailable")
		}
		if session, ok := ctx.Value(sessionKey).(store.Session); ok && v.OwnerID != session.User.ID {
			return errors.New("Local source is unavailable to this account")
		}
		if p, _ := ctx.Value(automationKey).(*store.OperationToken); p != nil && !p.Allows("deploy", v.ApplicationID) {
			return errors.New("Local source is outside this token's applications")
		}
		if c.Method == "image" || c.Method == "inline" || c.Method == "compose" && c.ComposeSource == "paste" {
			return errors.New("Choose a build method for the local source")
		}
		return nil
	}
	if c.Method == "inline" {
		return nil
	}
	if c.Method == "image" {
		ref, e := deploy.ImageReference(c.Image)
		if e != nil {
			return e
		}
		if c.RegistryID != "" {
			connection, e := s.db.Connection(ctx, c.RegistryID)
			if e != nil || connection.Kind != "registry" || connection.Host != reference.Domain(ref) {
				return errors.New("Choose a registry connection matching the image host")
			}
		}
		return nil
	}
	if c.SourceID == "" {
		return nil
	}
	connection, e := s.db.Connection(ctx, c.SourceID)
	if e != nil {
		return errors.New("Choose an available GitHub connection")
	}
	repo, e := s.github.Repository(ctx, connection, c.Repository)
	if e != nil {
		return e
	}
	if c.RepositoryID != 0 && c.RepositoryID != repo.ID {
		return errors.New("Repository identity changed; select it again")
	}
	c.RepositoryID = repo.ID
	return nil
}
func (s *Server) beginGitHub(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Organization string `json:"organization"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	organization := strings.TrimSpace(input.Organization)
	if organization != "" && (strings.Contains(organization, "--") || !regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$`).MatchString(organization)) {
		s.problem(w, 422, "invalid_organization", "Enter a GitHub organization name")
		return
	}
	origin := s.publicURL()
	if origin == "" {
		origin = requestOrigin(r)
	}
	parsed, e := url.Parse(origin)
	if e != nil || parsed.Host == "" {
		s.problem(w, 422, "invalid_origin", "Configure Shipper's public URL")
		return
	}
	id, state := auth.Token()[:24], auth.Token()
	if e = s.db.CreateProviderFlow(r.Context(), auth.Hash(state), id, origin); e != nil {
		s.internal(w, e)
		return
	}
	active := parsed.Scheme == "https"
	manifest := map[string]any{"name": "Shipper " + id[:8], "url": origin, "public": false, "redirect_url": origin + "/connect/github/manifest", "setup_url": origin + "/connect/github/installed?state=" + state, "hook_attributes": map[string]any{"url": origin + "/hooks/github/" + id, "active": active}, "default_events": []string{"push"}, "default_permissions": map[string]string{"contents": "read", "metadata": "read"}}
	action := "https://github.com/settings/apps/new"
	if organization != "" {
		action = "https://github.com/organizations/" + url.PathEscape(organization) + "/settings/apps/new"
	}
	s.json(w, 200, map[string]any{"action": action + "?state=" + state, "manifest": manifest})
}
func (s *Server) githubManifest(w http.ResponseWriter, r *http.Request) {
	state, code := r.URL.Query().Get("state"), r.URL.Query().Get("code")
	if len(state) != 64 || !regexp.MustCompile(`^[A-Za-z0-9]{1,256}$`).MatchString(code) {
		s.problem(w, 422, "invalid_callback", "Invalid GitHub callback")
		return
	}
	flow, e := s.db.AdvanceProviderFlow(r.Context(), auth.Hash(state), "manifest", "exchanging")
	if e != nil {
		s.problem(w, 403, "expired_callback", "GitHub setup expired; connect again")
		return
	}
	var result struct {
		ID            int64
		Slug          string
		PEM           string
		WebhookSecret string `json:"webhook_secret"`
	}
	e = s.github.Request(r.Context(), "POST", "/app-manifests/"+url.PathEscape(code)+"/conversions", "", nil, &result)
	if e != nil {
		s.problem(w, 422, "github_denied", e.Error())
		return
	}
	if !regexp.MustCompile(`^[A-Za-z0-9-]{1,100}$`).MatchString(result.Slug) {
		s.problem(w, 422, "invalid_app", "GitHub returned an invalid application")
		return
	}
	c := store.Connection{ID: flow.ConnectionID, Name: result.Slug, Kind: "github_app", Host: "github.com", Secret: store.ConnectionSecret{AppID: result.ID, PrivateKey: result.PEM, WebhookSecret: result.WebhookSecret, Slug: result.Slug}}
	if e = s.db.SaveConnection(r.Context(), c); e != nil {
		s.internal(w, e)
		return
	}
	if _, e = s.db.AdvanceProviderFlow(r.Context(), auth.Hash(state), "exchanging", "install"); e != nil {
		s.internal(w, e)
		return
	}
	http.Redirect(w, r, "https://github.com/apps/"+result.Slug+"/installations/new?state="+state, 303)
}
func (s *Server) githubInstalled(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	id, e := strconv.ParseInt(r.URL.Query().Get("installation_id"), 10, 64)
	if e != nil || len(state) != 64 {
		s.problem(w, 422, "invalid_callback", "Invalid installation callback")
		return
	}
	flow, e := s.db.AdvanceProviderFlow(r.Context(), auth.Hash(state), "install", "done")
	if e != nil {
		s.problem(w, 403, "expired_callback", "GitHub setup expired; connect again")
		return
	}
	c, e := s.db.Connection(r.Context(), flow.ConnectionID)
	if e != nil {
		s.resourceError(w, e)
		return
	}
	name, e := s.github.VerifyInstallation(r.Context(), c, id)
	if e != nil {
		s.problem(w, 422, "github_denied", e.Error())
		return
	}
	c.Secret.InstallationID = id
	c.Name = name
	if e = s.db.SaveConnection(r.Context(), c); e != nil {
		s.internal(w, e)
		return
	}
	http.Redirect(w, r, flow.Origin+"/settings#sources", 303)
}
func (s *Server) inlineSource(w http.ResponseWriter, r *http.Request) {
	a, e := s.db.Application(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		s.resourceError(w, e)
		return
	}
	s.json(w, 200, map[string]string{"content": a.Config.InlineDockerfile})
}

func (s *Server) connectionWebhook(w http.ResponseWriter, r *http.Request) {
	c, e := s.db.Connection(r.Context(), chi.URLParam(r, "connection"))
	if e != nil {
		s.resourceError(w, e)
		return
	}
	s.json(w, 200, map[string]string{"secret": c.Secret.WebhookSecret, "path": "/hooks/github/" + c.ID})
}
