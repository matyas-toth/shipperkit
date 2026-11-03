package gitprovider

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"shipper/internal/store"
	"strconv"
	"sync"
	"time"
)

type GitHub struct {
	BaseURL string
	Client  *http.Client
	mu      sync.Mutex
	tokens  map[string]cachedToken
}
type cachedToken struct {
	Value   string
	Expires time.Time
}

func New() *GitHub {
	return &GitHub{BaseURL: "https://api.github.com", Client: &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, tokens: map[string]cachedToken{}}
}
func (g *GitHub) Request(ctx context.Context, method, path, token string, input, out any) error {
	var body io.Reader
	if input != nil {
		raw, e := json.Marshal(input)
		if e != nil {
			return e
		}
		body = bytes.NewReader(raw)
	}
	req, e := http.NewRequestWithContext(ctx, method, g.BaseURL+path, body)
	if e != nil {
		return e
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2026-03-10")
	req.Header.Set("User-Agent", "Shipper")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, e := g.Client.Do(req)
	if e != nil {
		return errors.New("GitHub could not be reached")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		if res.StatusCode == 401 || res.StatusCode == 403 {
			return errors.New("GitHub access was denied; check or reconnect this connection")
		}
		if res.StatusCode == 404 {
			return errors.New("GitHub repository or installation is unavailable to this connection")
		}
		return fmt.Errorf("GitHub request failed (HTTP %d)", res.StatusCode)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(out)
}
func AppJWT(c store.Connection) (string, error) {
	block, _ := pem.Decode([]byte(c.Secret.PrivateKey))
	if block == nil {
		return "", errors.New("Enter a PEM-encoded GitHub App private key")
	}
	key, e := x509.ParsePKCS8PrivateKey(block.Bytes)
	if e != nil {
		key, e = x509.ParsePKCS1PrivateKey(block.Bytes)
	}
	if e != nil {
		return "", errors.New("Invalid GitHub App private key")
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return "", errors.New("GitHub Apps require an RSA private key")
	}
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]any{"iat": time.Now().Add(-time.Minute).Unix(), "exp": time.Now().Add(8 * time.Minute).Unix(), "iss": strconv.FormatInt(c.Secret.AppID, 10)})
	unsigned := header + "." + base64.RawURLEncoding.EncodeToString(claims)
	hash := sha256.Sum256([]byte(unsigned))
	signature, e := rsa.SignPKCS1v15(rand.Reader, rsaKey, crypto.SHA256, hash[:])
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), e
}
func (g *GitHub) Token(ctx context.Context, c store.Connection, repo int64) (string, error) {
	if c.Kind == "github_token" {
		if c.Secret.Token == "" {
			return "", errors.New("Reconnect GitHub")
		}
		return c.Secret.Token, nil
	}
	if c.Kind != "github_app" || c.Secret.InstallationID == 0 {
		return "", errors.New("Install the GitHub App before selecting a repository")
	}
	key := fmt.Sprintf("%s:%d:%d", c.ID, c.UpdatedAt, repo)
	g.mu.Lock()
	cached := g.tokens[key]
	g.mu.Unlock()
	if cached.Value != "" && time.Until(cached.Expires) > 2*time.Minute {
		return cached.Value, nil
	}
	jwt, e := AppJWT(c)
	if e != nil {
		return "", e
	}
	input := map[string]any{"permissions": map[string]string{"contents": "read", "metadata": "read"}}
	if repo > 0 {
		input["repository_ids"] = []int64{repo}
	}
	var result struct {
		Token   string
		Expires time.Time `json:"expires_at"`
	}
	e = g.Request(ctx, "POST", fmt.Sprintf("/app/installations/%d/access_tokens", c.Secret.InstallationID), jwt, input, &result)
	if e != nil {
		return "", e
	}
	if result.Token == "" {
		return "", errors.New("GitHub returned no installation token")
	}
	g.mu.Lock()
	if len(g.tokens) >= 128 {
		g.tokens = map[string]cachedToken{}
	}
	g.tokens[key] = cachedToken{result.Token, result.Expires}
	g.mu.Unlock()
	return result.Token, nil
}

type Repository struct {
	ID            int64  `json:"id"`
	FullName      string `json:"full_name"`
	CloneURL      string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
}

func (g *GitHub) Repositories(ctx context.Context, c store.Connection, page int) ([]Repository, error) {
	token, e := g.Token(ctx, c, 0)
	if e != nil {
		return nil, e
	}
	suffix := fmt.Sprintf("?per_page=100&page=%d", page)
	if c.Kind == "github_app" {
		var result struct{ Repositories []Repository }
		e = g.Request(ctx, "GET", "/installation/repositories"+suffix, token, nil, &result)
		return result.Repositories, e
	}
	var out []Repository
	e = g.Request(ctx, "GET", "/user/repos"+suffix+"&sort=updated", token, nil, &out)
	return out, e
}
func RepositoryPath(raw string) (string, error) {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("Choose a GitHub repository")
	}
	parts := bytes.Split([]byte(u.Path), []byte("/"))
	if len(parts) != 3 || len(parts[1]) == 0 || len(parts[2]) == 0 {
		return "", errors.New("Choose a GitHub repository")
	}
	name := string(parts[2])
	if len(name) > 4 && name[len(name)-4:] == ".git" {
		name = name[:len(name)-4]
	}
	return "/repos/" + url.PathEscape(string(parts[1])) + "/" + url.PathEscape(name), nil
}
func (g *GitHub) Repository(ctx context.Context, c store.Connection, raw string) (Repository, error) {
	path, e := RepositoryPath(raw)
	if e != nil {
		return Repository{}, e
	}
	token, e := g.Token(ctx, c, 0)
	if e != nil {
		return Repository{}, e
	}
	var repo Repository
	e = g.Request(ctx, "GET", path, token, nil, &repo)
	return repo, e
}

type Revision struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	SHA  string `json:"sha"`
}

func (g *GitHub) Revisions(ctx context.Context, c store.Connection, raw string) ([]Revision, error) {
	path, e := RepositoryPath(raw)
	if e != nil {
		return nil, e
	}
	repo, e := g.Repository(ctx, c, raw)
	if e != nil {
		return nil, e
	}
	token, e := g.Token(ctx, c, repo.ID)
	if e != nil {
		return nil, e
	}
	out := []Revision{}
	for _, kind := range []string{"branches", "tags"} {
		for page := 1; page <= 5; page++ {
			var refs []struct {
				Name   string
				Commit struct{ SHA string }
			}
			e = g.Request(ctx, "GET", fmt.Sprintf("%s/%s?per_page=100&page=%d", path, kind, page), token, nil, &refs)
			if e != nil {
				return nil, e
			}
			for _, ref := range refs {
				out = append(out, Revision{ref.Name, kind, ref.Commit.SHA})
			}
			if len(refs) < 100 {
				break
			}
		}
	}
	return out, nil
}
func (g *GitHub) VerifyInstallation(ctx context.Context, c store.Connection, id int64) (string, error) {
	jwt, e := AppJWT(c)
	if e != nil {
		return "", e
	}
	var result struct {
		AppID   int64 `json:"app_id"`
		Account struct{ Login string }
	}
	e = g.Request(ctx, "GET", fmt.Sprintf("/app/installations/%d", id), jwt, nil, &result)
	if e != nil {
		return "", e
	}
	if result.AppID != c.Secret.AppID {
		return "", errors.New("Installation does not belong to this GitHub App")
	}
	return result.Account.Login, nil
}

func (g *GitHub) Head(ctx context.Context, c store.Connection, raw, branch string) (string, error) {
	repo, e := g.Repository(ctx, c, raw)
	if e != nil {
		return "", e
	}
	if branch == "" {
		branch = repo.DefaultBranch
	}
	path, e := RepositoryPath(raw)
	if e != nil {
		return "", e
	}
	token, e := g.Token(ctx, c, repo.ID)
	if e != nil {
		return "", e
	}
	var commit struct{ SHA string }
	e = g.Request(ctx, "GET", path+"/commits/"+url.PathEscape(branch), token, nil, &commit)
	return commit.SHA, e
}
