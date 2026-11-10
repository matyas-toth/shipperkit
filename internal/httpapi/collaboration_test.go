package httpapi

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"shipper/internal/auth"
	"shipper/internal/deploy"
	"shipper/internal/events"
	"shipper/internal/inventory"
	"shipper/internal/store"
)

type fakeEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

type fakeIdentity struct {
	nonce        string
	challenge    string
	subject      string
	email        string
	verified     bool
	issuer       string
	audience     string
	badSignature bool
	emails       []fakeEmail
}

type fakeOAuthProvider struct {
	mu       sync.Mutex
	server   *httptest.Server
	key      *rsa.PrivateKey
	otherKey *rsa.PrivateKey
	base     string
	publicURL string
	google   fakeIdentity
	github   fakeIdentity
	discord  fakeIdentity
	calls    int
}

func newFakeOAuth(t *testing.T) *fakeOAuthProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeOAuthProvider{key: key, otherKey: other}
	fake.google.verified = true
	fake.google.subject = "google-4242"
	fake.discord.verified = true
	fake.github.subject = "4242"
	fake.discord.subject = "discord-4242"
	fake.server = httptest.NewServer(http.HandlerFunc(fake.handle))
	fake.base = fake.server.URL
	t.Cleanup(fake.server.Close)
	return fake
}

func (f *fakeOAuthProvider) callback(provider string) string {
	return strings.TrimSuffix(f.publicURL, "/") + "/auth/oauth/" + provider + "/callback"
}

func (f *fakeOAuthProvider) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/jwks":
		json.NewEncoder(w).Encode(map[string]any{"keys": []any{jwk(f.key.PublicKey, "kid-1")}})
	case "/token":
		f.calls++
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(400)
			return
		}
		switch r.Form.Get("client_id") {
		case "google-client":
			if r.Form.Get("client_secret") != "google-secret-rotated" && r.Form.Get("client_secret") != "google-secret" {
				w.WriteHeader(400)
				return
			}
			if r.Form.Get("redirect_uri") != f.callback("google") {
				w.WriteHeader(400)
				return
			}
			if f.google.challenge != "" && auth.PKCEChallenge(r.Form.Get("code_verifier")) != f.google.challenge {
				w.WriteHeader(400)
				return
			}
			claims := map[string]any{
				"iss": f.issuer("google", f.base), "aud": f.audience("google", "google-client"),
				"exp": time.Now().Add(5 * time.Minute).Unix(), "iat": time.Now().Add(-5 * time.Second).Unix(),
				"nonce": f.google.nonce, "sub": f.google.subject, "email": f.google.email, "email_verified": f.google.verified,
			}
			signKey := f.key
			if f.google.badSignature {
				signKey = f.otherKey
			}
			json.NewEncoder(w).Encode(map[string]any{"access_token": "google-token", "id_token": signJWT(signKey, claims)})
		case "github-client":
			if r.Form.Get("client_secret") != "github-secret" || r.Form.Get("redirect_uri") != f.callback("github") {
				w.WriteHeader(400)
				return
			}
			w.Write([]byte(`{"access_token":"github-token","token_type":"bearer"}`))
		case "discord-client":
			if r.Form.Get("client_secret") != "discord-secret" || r.Form.Get("redirect_uri") != f.callback("discord") {
				w.WriteHeader(400)
				return
			}
			if f.discord.challenge == "" || auth.PKCEChallenge(r.Form.Get("code_verifier")) != f.discord.challenge {
				w.WriteHeader(400)
				return
			}
			w.Write([]byte(`{"access_token":"discord-token","token_type":"bearer"}`))
		default:
			w.WriteHeader(400)
		}
	case "/user":
		if r.Header.Get("Authorization") != "Bearer github-token" {
			w.WriteHeader(401)
			return
		}
		id, _ := strconvAtoi(f.github.subject)
		json.NewEncoder(w).Encode(map[string]any{"id": id, "login": "fixture", "name": "Fixture"})
	case "/user/emails":
		json.NewEncoder(w).Encode(f.github.emails)
	case "/users/@me":
		if r.Header.Get("Authorization") != "Bearer discord-token" {
			w.WriteHeader(401)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"id": f.discord.subject, "email": f.discord.email, "verified": f.discord.verified, "username": "fixture"})
	default:
		w.WriteHeader(404)
	}
}

func strconvAtoi(value string) (int64, bool) {
	var result int64
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, false
		}
		result = result*10 + int64(r-'0')
	}
	return result, true
}

func (f *fakeOAuthProvider) issuer(provider, fallback string) string {
	if f.identity(provider).issuer != "" {
		return f.identity(provider).issuer
	}
	return fallback
}

func (f *fakeOAuthProvider) audience(provider, fallback string) string {
	if f.identity(provider).audience != "" {
		return f.identity(provider).audience
	}
	return fallback
}

func (f *fakeOAuthProvider) identity(provider string) *fakeIdentity {
	switch provider {
	case "google":
		return &f.google
	case "github":
		return &f.github
	}
	return &f.discord
}

func (f *fakeOAuthProvider) configure(provider string, edit func(*fakeIdentity)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	edit(f.identity(provider))
}

func jwk(key rsa.PublicKey, id string) map[string]any {
	return map[string]any{"kid": id, "kty": "RSA", "use": "sig", "alg": "RS256",
		"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}
}

func signJWT(key *rsa.PrivateKey, claims map[string]any) string {
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "kid-1", "typ": "JWT"})
	payload, _ := json.Marshal(claims)
	input := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	sum := sha256.Sum256([]byte(input))
	signature, _ := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	return input + "." + base64.RawURLEncoding.EncodeToString(signature)
}

type authFixture struct {
	db     *store.Store
	server *httptest.Server
	fake   *fakeOAuthProvider
}

func newAuthFixture(t *testing.T) *authFixture {
	t.Helper()
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	hub := events.New()
	monitor := inventory.New(testEngine{}, db, hub)
	if err = monitor.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	fake := newFakeOAuth(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	publicURL := "http://" + listener.Addr().String()
	fake.publicURL = publicURL
	handler := New(db, monitor, hub, Config{
		PublicURL: publicURL, Version: "test", Assets: http.NotFoundHandler(),
		Worker: deploy.NewWorker(db, nil, hub, t.TempDir()),
		OAuth: &OAuthEndpoints{
			GoogleAuthorize: fake.base + "/authorize", GoogleToken: fake.base + "/token", GoogleJWKS: fake.base + "/jwks", GoogleIssuer: fake.base,
			GitHubAuthorize: fake.base + "/authorize", GitHubToken: fake.base + "/token", GitHubAPI: fake.base,
			DiscordAuthorize: fake.base + "/authorize", DiscordToken: fake.base + "/token", DiscordAPI: fake.base,
		},
	})
	server := &httptest.Server{Listener: listener, Config: &http.Server{Handler: handler}}
	server.Start()
	t.Cleanup(server.Close)
	password, _ := auth.Password("administrator-password-123")
	if err = db.CreateAdmin(context.Background(), store.User{ID: "owner", Email: "owner@example.test", PasswordHash: password, Role: store.RoleAdmin}, "", "", 0); err != nil {
		t.Fatal(err)
	}
	return &authFixture{db: db, server: server, fake: fake}
}

type testClient struct {
	client *http.Client
}

func newTestClient(t *testing.T) *testClient {
	jar, _ := cookiejar.New(nil)
	return &testClient{client: &http.Client{Jar: jar, Timeout: 10 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (c *testClient) call(t *testing.T, fx *authFixture, method, path string, body any, csrf string) (int, map[string]any, http.Header) {
	t.Helper()
	var data []byte
	if body != nil {
		data, _ = json.Marshal(body)
	}
	request, _ := http.NewRequest(method, fx.server.URL+path, bytes.NewReader(data))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if csrf != "" {
		request.Header.Set("X-CSRF-Token", csrf)
	}
	response, err := c.client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(response.Body)
	result := map[string]any{}
	_ = json.Unmarshal(raw, &result)
	return response.StatusCode, result, response.Header
}

func (c *testClient) login(t *testing.T, fx *authFixture, email, password string, want int) map[string]any {
	t.Helper()
	code, body, _ := c.call(t, fx, "POST", "/api/auth/login", map[string]string{"email": email, "password": password}, "")
	if code != want {
		t.Fatalf("login %s returned %d %v", email, code, body)
	}
	return body
}

func (c *testClient) bearer(t *testing.T, fx *authFixture, path, token string) int {
	t.Helper()
	request, _ := http.NewRequest("GET", fx.server.URL+path, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := c.client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	return response.StatusCode
}

func sessionOf(body map[string]any) map[string]any {
	user, _ := body["user"].(map[string]any)
	return user
}

func (fx *authFixture) provision(t *testing.T, admin *testClient, csrf, email, role, password string) map[string]any {
	t.Helper()
	code, body, _ := admin.call(t, fx, "POST", "/api/users", map[string]any{"email": email, "role": role, "password": password}, csrf)
	if code != 201 {
		t.Fatalf("provision %s returned %d %v", email, code, body)
	}
	return body
}

func (fx *authFixture) providerSettings(t *testing.T, admin *testClient, csrf string) {
	t.Helper()
	for _, item := range []struct{ provider, client, secret string }{
		{"google", "google-client", "google-secret"},
		{"github", "github-client", "github-secret"},
		{"discord", "discord-client", "discord-secret"},
	} {
		code, body, _ := admin.call(t, fx, "POST", "/api/auth/providers/"+item.provider, map[string]any{"clientId": item.client, "clientSecret": item.secret, "enabled": true}, csrf)
		if code != 200 {
			t.Fatalf("save provider %s returned %d %v", item.provider, code, body)
		}
	}
}

func (c *testClient) oauth(t *testing.T, fx *authFixture, provider, email string) string {
	t.Helper()
	query := c.oauthStart(t, fx, provider)
	fx.fake.configure(provider, func(identity *fakeIdentity) {
		identity.email = email
		if provider == "github" {
			identity.emails = []fakeEmail{{Email: email, Primary: true, Verified: true}}
		}
	})
	return c.oauthFinish(t, fx, provider, query.Get("state"))
}

func (c *testClient) oauthStart(t *testing.T, fx *authFixture, provider string) url.Values {
	t.Helper()
	response, err := c.client.Get(fx.server.URL + "/auth/oauth/" + provider + "/start")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("%s start returned %d", provider, response.StatusCode)
	}
	authorize, err := url.Parse(response.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	query := authorize.Query()
	if query.Get("state") == "" {
		t.Fatal("start did not include state")
	}
	fx.fake.configure(provider, func(identity *fakeIdentity) {
		identity.nonce = query.Get("nonce")
		identity.challenge = query.Get("code_challenge")
	})
	return query
}

func (c *testClient) oauthFinish(t *testing.T, fx *authFixture, provider, state string) string {
	t.Helper()
	callback := fx.server.URL + "/auth/oauth/" + provider + "/callback?state=" + url.QueryEscape(state) + "&code=fixture-code"
	reply, err := c.client.Get(callback)
	if err != nil {
		t.Fatal(err)
	}
	reply.Body.Close()
	return reply.Header.Get("Location")
}

func (c *testClient) session(t *testing.T, fx *authFixture) map[string]any {
	t.Helper()
	code, body, _ := c.call(t, fx, "GET", "/api/auth/session", nil, "")
	if code != 200 {
		t.Fatalf("session returned %d %v", code, body)
	}
	return body
}

func TestProvisioningPasswordGatesAndProviderIdentity(t *testing.T) {
	fx := newAuthFixture(t)
	admin := newTestClient(t)
	owner := admin.login(t, fx, "owner@example.test", "administrator-password-123", 200)
	csrf := owner["csrfToken"].(string)
	provisioned := fx.provision(t, admin, csrf, "person@example.test", "developer", "temporary-password-123")

	person := newTestClient(t)
	first := person.login(t, fx, "person@example.test", "temporary-password-123", 200)
	if first["authStep"] != "password_change" {
		t.Fatalf("temporary password did not gate access: %v", first)
	}
	personCsrf := first["csrfToken"].(string)
	if code, _, _ := person.call(t, fx, "GET", "/api/applications", nil, ""); code != 403 {
		t.Fatalf("temporary password reached applications: %d", code)
	}
	if code, body, _ := person.call(t, fx, "POST", "/api/auth/password", map[string]string{"currentPassword": "temporary-password-123", "newPassword": "temporary-password-123"}, personCsrf); code != 422 {
		t.Fatalf("temporary password reuse allowed: %d %v", code, body)
	}
	changed := map[string]any{}
	if code, body, _ := person.call(t, fx, "POST", "/api/auth/password", map[string]string{"currentPassword": "temporary-password-123", "newPassword": "permanent-password-123"}, personCsrf); code != 200 {
		t.Fatalf("password change failed: %d %v", code, body)
	} else {
		changed = body
	}
	if changed["authStep"] != nil || changed["user"].(map[string]any)["passwordChangeRequired"] != nil {
		t.Fatalf("permanent password did not clear the gate: %v", changed)
	}
	personCsrf = changed["csrfToken"].(string)
	if code, _, _ := person.call(t, fx, "GET", "/api/applications", nil, ""); code != 200 {
		t.Fatalf("permanent password did not reach applications: %d", code)
	}

	fx.providerSettings(t, admin, csrf)
	if location := person.oauth(t, fx, "google", "person@example.test"); location != "/" {
		t.Fatalf("google callback redirected to %q", location)
	}
	googleSession := person.session(t, fx)
	if googleSession["user"].(map[string]any)["id"] != provisioned["id"] || googleSession["user"].(map[string]any)["role"] != "developer" {
		t.Fatalf("google did not resolve the provisioned account: %v", googleSession)
	}
	if code, _, _ := person.call(t, fx, "GET", "/api/applications", nil, ""); code != 200 {
		t.Fatalf("google login did not grant application access: %d", code)
	}

	if location := person.oauth(t, fx, "github", "person@example.test"); location != "/" {
		t.Fatalf("github callback redirected to %q", location)
	}
	githubSession := person.session(t, fx)
	if githubSession["user"].(map[string]any)["id"] != provisioned["id"] {
		t.Fatalf("github did not resolve the same account: %v", githubSession)
	}
	if location := person.oauth(t, fx, "discord", "person@example.test"); location != "/" {
		t.Fatalf("discord callback redirected to %q", location)
	}
	discordSession := person.session(t, fx)
	if discordSession["user"].(map[string]any)["id"] != provisioned["id"] {
		t.Fatalf("discord did not resolve the same account: %v", discordSession)
	}

	// A linked stable subject wins even when the provider starts reporting a new
	// address that belongs to a second provisioned account.
	fx.provision(t, admin, csrf, "other@example.test", "viewer", "temporary-password-456")
	fx.fake.configure("github", func(identity *fakeIdentity) {
		identity.emails = []fakeEmail{{Email: "other@example.test", Primary: true, Verified: true}}
	})
	if location := person.oauth(t, fx, "github", "other@example.test"); location != "/" {
		t.Fatalf("stable github login redirected to %q", location)
	}
	stable := person.session(t, fx)
	if stable["user"].(map[string]any)["id"] != provisioned["id"] {
		t.Fatalf("changed provider email moved the identity: %v", stable)
	}

	// Unknown and unverified identities fail safely and never create accounts.
	unknown := newTestClient(t)
	fx.fake.configure("github", func(identity *fakeIdentity) { identity.subject = "9999" })
	if location := unknown.oauth(t, fx, "github", "stranger@example.test"); !strings.Contains(location, "auth_error=unprovisioned") {
		t.Fatalf("unprovisioned sign-in redirected to %q", location)
	}
	fx.fake.configure("github", func(identity *fakeIdentity) { identity.subject = "4242" })
	fx.fake.configure("discord", func(identity *fakeIdentity) { identity.verified = false })
	if location := unknown.oauth(t, fx, "discord", "stranger@example.test"); !strings.Contains(location, "auth_error=email_unverified") {
		t.Fatalf("unverified sign-in redirected to %q", location)
	}
	fx.fake.configure("google", func(identity *fakeIdentity) { identity.email = "person@example.test"; identity.verified = false })
	if location := unknown.oauth(t, fx, "google", "person@example.test"); !strings.Contains(location, "auth_error=email_unverified") {
		t.Fatalf("unverified google claim redirected to %q", location)
	}
	fx.fake.configure("google", func(identity *fakeIdentity) { identity.verified = true })

	// Administrator reset makes the replacement temporary and revokes sessions.
	reset := newTestClient(t)
	resetLogin := reset.login(t, fx, "person@example.test", "permanent-password-123", 200)
	resetCsrf := resetLogin["csrfToken"].(string)
	if code, body, _ := admin.call(t, fx, "POST", "/api/users/"+provisioned["id"].(string)+"/password", map[string]string{"password": "reset-password-123"}, csrf); code != 200 {
		t.Fatalf("admin password reset failed: %d %v", code, body)
	}
	if code, _, _ := reset.call(t, fx, "GET", "/api/applications", nil, resetCsrf); code != 401 {
		t.Fatalf("reset did not revoke the existing session: %d", code)
	}
	if code, body, _ := reset.call(t, fx, "POST", "/api/auth/login", map[string]string{"email": "person@example.test", "password": "permanent-password-123"}, ""); code != 401 {
		t.Fatalf("old password survived reset: %d %v", code, body)
	}
	if body := reset.login(t, fx, "person@example.test", "reset-password-123", 200); body["authStep"] != "password_change" {
		t.Fatalf("reset password was not temporary: %v", body)
	}

	// Removing the password denies password login but keeps the provider route.
	if code, body, _ := admin.call(t, fx, "POST", "/api/users/"+provisioned["id"].(string)+"/password/remove", map[string]any{}, csrf); code != 200 {
		t.Fatalf("remove password failed: %d %v", code, body)
	}
	newTestClient(t).login(t, fx, "person@example.test", "reset-password-123", 401)

	// A password added to a provider-only or provider-linked account is
	// temporary, so the gate follows the account across login methods.
	if code, body, _ := admin.call(t, fx, "POST", "/api/users/"+provisioned["id"].(string)+"/password", map[string]string{"password": "later-password-123"}, csrf); code != 200 {
		t.Fatalf("adding a password failed: %d %v", code, body)
	}
	if body := newTestClient(t).login(t, fx, "person@example.test", "later-password-123", 200); body["authStep"] != "password_change" {
		t.Fatalf("password added later was not temporary: %v", body)
	}
	if location := person.oauth(t, fx, "google", "person@example.test"); location != "/" {
		t.Fatalf("provider login redirected to %q", location)
	}
	if gate := person.session(t, fx); gate["authStep"] != "password_change" {
		t.Fatalf("provider login bypassed the temporary password gate: %v", gate)
	}
	if code, body, _ := admin.call(t, fx, "POST", "/api/users/"+provisioned["id"].(string)+"/password/remove", map[string]any{}, csrf); code != 200 {
		t.Fatalf("second password removal failed: %d %v", code, body)
	}

	// Provider removal unlinks identities and stops new sign-ins.
	if code, body, _ := admin.call(t, fx, "POST", "/api/auth/providers/google/remove", map[string]any{}, csrf); code != 200 {
		t.Fatalf("provider removal failed: %d %v", code, body)
	}
	if response, err := newTestClient(t).client.Get(fx.server.URL + "/auth/oauth/google/start"); err != nil {
		t.Fatal(err)
	} else {
		response.Body.Close()
		if !strings.Contains(response.Header.Get("Location"), "auth_error=provider_disabled") {
			t.Fatalf("removed provider still started a sign-in: %q", response.Header.Get("Location"))
		}
	}

	// Disabling revokes sessions and refuses every method.
	person.login(t, fx, "person@example.test", "reset-password-123", 401)
	if code, body, _ := admin.call(t, fx, "POST", "/api/users/"+provisioned["id"].(string)+"/disable", map[string]any{}, csrf); code != 200 {
		t.Fatalf("disable failed: %d %v", code, body)
	}
	if code, _, _ := person.call(t, fx, "GET", "/api/auth/session", nil, ""); code != 200 {
		t.Fatalf("session endpoint failed: %d", code)
	}
	if person.session(t, fx)["user"] != nil {
		t.Fatal("disabled account kept its session")
	}
	fx.fake.configure("discord", func(identity *fakeIdentity) { identity.verified = true })
	if location := newTestClient(t).oauth(t, fx, "discord", "person@example.test"); !strings.Contains(location, "auth_error=account_disabled") {
		t.Fatalf("disabled account signed in through discord: %q", location)
	}
}

func TestOAuthAmbiguousAndMissingIdentity(t *testing.T) {
	fx := newAuthFixture(t)
	admin := newTestClient(t)
	owner := admin.login(t, fx, "owner@example.test", "administrator-password-123", 200)
	csrf := owner["csrfToken"].(string)
	fx.provision(t, admin, csrf, "first@example.test", "viewer", "first-password-123")
	fx.provision(t, admin, csrf, "second@example.test", "admin", "second-password-123")
	fx.providerSettings(t, admin, csrf)

	client := newTestClient(t)
	query := client.oauthStart(t, fx, "github")
	fx.fake.configure("github", func(identity *fakeIdentity) {
		identity.subject = "7777"
		identity.emails = []fakeEmail{
			{Email: "first@example.test", Primary: false, Verified: true},
			{Email: "second@example.test", Primary: true, Verified: true},
		}
	})
	if location := client.oauthFinish(t, fx, "github", query.Get("state")); !strings.Contains(location, "auth_error=ambiguous") {
		t.Fatalf("ambiguous identity was not rejected: %s", location)
	}

	query = client.oauthStart(t, fx, "github")
	fx.fake.configure("github", func(identity *fakeIdentity) {
		identity.subject = "7778"
		identity.emails = []fakeEmail{{Email: "first@example.test", Primary: true, Verified: false}}
	})
	if location := client.oauthFinish(t, fx, "github", query.Get("state")); !strings.Contains(location, "auth_error=email_unverified") {
		t.Fatalf("unverified email was not rejected: %s", location)
	}

	// Identity and role authority never come from the provider profile.
	query = client.oauthStart(t, fx, "github")
	fx.fake.configure("github", func(identity *fakeIdentity) {
		identity.subject = "7779"
		identity.emails = []fakeEmail{{Email: "first@example.test", Primary: true, Verified: true}}
	})
	if location := client.oauthFinish(t, fx, "github", query.Get("state")); location != "/" {
		t.Fatalf("verified email did not link: %s", location)
	}
	session := client.session(t, fx)
	if session["user"].(map[string]any)["role"] != "viewer" {
		t.Fatalf("provider profile changed the Shipper role: %v", session)
	}

	// A provider-only account has no password credential. Blank is not a
	// password and provider sign-in is the only successful method.
	providerOnly := fx.provision(t, admin, csrf, "provider@example.test", "viewer", "")
	_ = providerOnly
	newTestClient(t).login(t, fx, "provider@example.test", "", 401)
	providerClient := newTestClient(t)
	if location := providerClient.oauth(t, fx, "discord", "provider@example.test"); location != "/" {
		t.Fatalf("provider-only account could not sign in: %s", location)
	}
	if session := providerClient.session(t, fx); session["user"].(map[string]any)["id"] != providerOnly["id"] {
		t.Fatalf("provider-only account resolved incorrectly: %v", session)
	}
}

func TestProviderCallbackAndSecretMasking(t *testing.T) {
	fx := newAuthFixture(t)
	admin := newTestClient(t)
	owner := admin.login(t, fx, "owner@example.test", "administrator-password-123", 200)
	csrf := owner["csrfToken"].(string)
	fx.providerSettings(t, admin, csrf)

	code, body, _ := admin.call(t, fx, "GET", "/api/auth/providers", nil, "")
	if code != 200 {
		t.Fatalf("provider list returned %d", code)
	}
	raw, _ := json.Marshal(body)
	if strings.Contains(string(raw), "google-secret") || strings.Contains(string(raw), "github-secret") {
		t.Fatalf("provider list exposed a secret: %s", raw)
	}
	providers := body["providers"].([]any)
	callback := providers[0].(map[string]any)["callback"].(string)
	if !strings.HasPrefix(callback, "http://127.0.0.1:") || !strings.HasSuffix(callback, "/auth/oauth/google/callback") {
		t.Fatalf("callback was not derived from the canonical origin: %s", callback)
	}

	// Anonymous method discovery exposes only enabled and configured methods.
	code, methods, _ := newTestClient(t).call(t, fx, "GET", "/api/auth/methods", nil, "")
	if code != 200 || methods["password"] != true || methods["google"] != true || methods["github"] != true || methods["discord"] != true {
		t.Fatalf("auth methods incorrect: %d %v", code, methods)
	}

	// The settings test performs the complete provider round trip and returns
	// to Settings with a result instead of creating a session.
	code, test, _ := admin.call(t, fx, "POST", "/api/auth/providers/google/test", map[string]any{}, csrf)
	if code != 200 {
		t.Fatalf("provider test start failed: %d %v", code, test)
	}
	authorize, _ := url.Parse(test["url"].(string))
	fx.fake.configure("google", func(identity *fakeIdentity) {
		identity.nonce = authorize.Query().Get("nonce")
		identity.challenge = authorize.Query().Get("code_challenge")
		identity.email = "owner@example.test"
		identity.verified = true
	})
	finish, _ := admin.client.Get(fx.server.URL + "/auth/oauth/google/callback?state=" + url.QueryEscape(authorize.Query().Get("state")) + "&code=test")
	finish.Body.Close()
	if !strings.Contains(finish.Header.Get("Location"), "oauth_result=success") {
		t.Fatalf("provider test did not return success: %s", finish.Header.Get("Location"))
	}

	code, test, _ = admin.call(t, fx, "POST", "/api/auth/providers/google/test", map[string]any{}, csrf)
	if code != 200 {
		t.Fatalf("provider test restart failed: %d %v", code, test)
	}
	fx.provision(t, admin, csrf, "other@example.test", "viewer", "other-password-123")
	authorize, _ = url.Parse(test["url"].(string))
	fx.fake.configure("google", func(identity *fakeIdentity) {
		identity.nonce = authorize.Query().Get("nonce")
		identity.challenge = authorize.Query().Get("code_challenge")
		identity.email = "other@example.test"
		identity.verified = true
	})
	finish, _ = admin.client.Get(fx.server.URL + "/auth/oauth/google/callback?state=" + url.QueryEscape(authorize.Query().Get("state")) + "&code=test")
	finish.Body.Close()
	if !strings.Contains(finish.Header.Get("Location"), "oauth_code=wrong_account") {
		t.Fatalf("provider test accepted another account: %s", finish.Header.Get("Location"))
	}
}

func TestOAuthStateReplayPKCEAndClaims(t *testing.T) {
	fx := newAuthFixture(t)
	admin := newTestClient(t)
	owner := admin.login(t, fx, "owner@example.test", "administrator-password-123", 200)
	csrf := owner["csrfToken"].(string)
	fx.provision(t, admin, csrf, "person@example.test", "developer", "temporary-password-123")
	fx.providerSettings(t, admin, csrf)

	client := newTestClient(t)
	start, _ := client.client.Get(fx.server.URL + "/auth/oauth/google/start")
	start.Body.Close()
	authorize, _ := url.Parse(start.Header.Get("Location"))
	state := authorize.Query().Get("state")
	fx.fake.configure("google", func(identity *fakeIdentity) {
		identity.nonce = authorize.Query().Get("nonce")
		identity.challenge = authorize.Query().Get("code_challenge")
		identity.email = "person@example.test"
		identity.verified = true
	})
	callback := fx.server.URL + "/auth/oauth/google/callback?state=" + url.QueryEscape(state) + "&code=one"
	first, _ := client.client.Get(callback)
	first.Body.Close()
	if first.StatusCode != 303 || first.Header.Get("Location") != "/" {
		t.Fatalf("first callback failed: %d %s", first.StatusCode, first.Header.Get("Location"))
	}
	// One-use state prevents replay.
	replay, _ := client.client.Get(callback)
	replay.Body.Close()
	if !strings.Contains(replay.Header.Get("Location"), "state_invalid") {
		t.Fatalf("state replay was accepted: %s", replay.Header.Get("Location"))
	}

	cases := []struct {
		name  string
		apply func(*fakeIdentity)
	}{
		{"nonce", func(identity *fakeIdentity) { identity.nonce = "wrong-nonce" }},
		{"audience", func(identity *fakeIdentity) { identity.audience = "another-client" }},
		{"issuer", func(identity *fakeIdentity) { identity.issuer = "https://evil.example" }},
		{"signature", func(identity *fakeIdentity) { identity.badSignature = true }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			flow := newTestClient(t)
			response, _ := flow.client.Get(fx.server.URL + "/auth/oauth/google/start")
			response.Body.Close()
			query, _ := url.Parse(response.Header.Get("Location"))
			fx.fake.configure("google", func(identity *fakeIdentity) {
				identity.nonce = query.Query().Get("nonce")
				identity.challenge = query.Query().Get("code_challenge")
				identity.email = "person@example.test"
				identity.verified = true
				identity.issuer = ""
				identity.audience = ""
				identity.badSignature = false
				tc.apply(identity)
			})
			callback := fx.server.URL + "/auth/oauth/google/callback?state=" + url.QueryEscape(query.Query().Get("state")) + "&code=bad"
			result, _ := flow.client.Get(callback)
			result.Body.Close()
			if !strings.Contains(result.Header.Get("Location"), "auth_error=provider_error") {
				t.Fatalf("%s was accepted: %s", tc.name, result.Header.Get("Location"))
			}
		})
	}
}

func TestRoleIsolationAndLastAdminProtection(t *testing.T) {
	fx := newAuthFixture(t)
	admin := newTestClient(t)
	owner := admin.login(t, fx, "owner@example.test", "administrator-password-123", 200)
	csrf := owner["csrfToken"].(string)
	developer := fx.provision(t, admin, csrf, "developer@example.test", "developer", "developer-password-123")
	viewer := fx.provision(t, admin, csrf, "viewer@example.test", "viewer", "viewer-password-123")
	_ = developer
	_ = viewer

	// A minimal managed application gives mutation routes something to address.
	app := store.Application{ID: "role-app", Name: "Role app", SourceKind: "image", State: "running", Method: "image", Config: store.Config{Image: "nginx:alpine", Port: 80}}
	if _, err := fx.db.Enqueue(context.Background(), app, store.Deployment{ID: "role-deploy", ApplicationID: "role-app", Operation: "deploy", State: "succeeded", Config: app.Config}, "", true); err != nil {
		t.Fatal(err)
	}

	dev := newTestClient(t)
	devSession := dev.login(t, fx, "developer@example.test", "developer-password-123", 200)
	if devSession["authStep"] != "password_change" {
		t.Fatalf("developer was not gated: %v", devSession)
	}
	devCsrf := devSession["csrfToken"].(string)
	// Complete the temporary password so the role checks are the only gate.
	if code, body, _ := dev.call(t, fx, "POST", "/api/auth/password", map[string]string{"currentPassword": "developer-password-123", "newPassword": "developer-permanent-123"}, devCsrf); code != 200 {
		t.Fatalf("developer password change failed: %d %v", code, body)
	}
	devSession = dev.session(t, fx)
	devCsrf = devSession["csrfToken"].(string)

	view := newTestClient(t)
	viewSession := view.login(t, fx, "viewer@example.test", "viewer-password-123", 200)
	viewCsrf := viewSession["csrfToken"].(string)
	if code, body, _ := view.call(t, fx, "POST", "/api/auth/password", map[string]string{"currentPassword": "viewer-password-123", "newPassword": "viewer-permanent-123"}, viewCsrf); code != 200 {
		t.Fatalf("viewer password change failed: %d %v", code, body)
	}
	viewCsrf = view.session(t, fx)["csrfToken"].(string)

	for _, tc := range []struct {
		name   string
		client *testClient
		csrf   string
		method string
		path   string
		body   any
		want   int
	}{
		{"viewer read", view, "", "GET", "/api/applications", nil, 200},
		{"viewer mutate", view, viewCsrf, "POST", "/api/applications/role-app/name", map[string]string{"name": "Nope"}, 403},
		{"viewer secrets", view, viewCsrf, "POST", "/api/applications/role-app/variables/reveal", map[string]string{"name": "SECRET"}, 403},
		{"viewer terminal", view, viewCsrf, "POST", "/api/applications/role-app/terminal", map[string]any{}, 403},
		{"viewer system", view, "", "GET", "/api/system", nil, 403},
		{"viewer users", view, "", "GET", "/api/users", nil, 403},
		{"developer read", dev, "", "GET", "/api/applications", nil, 200},
		{"developer mutate", dev, devCsrf, "POST", "/api/applications/role-app/name", map[string]string{"name": "Renamed"}, 200},
		{"developer users", dev, "", "GET", "/api/users", nil, 403},
		{"admin users", admin, "", "GET", "/api/users", nil, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, body, _ := tc.client.call(t, fx, tc.method, tc.path, tc.body, tc.csrf)
			if code != tc.want {
				t.Fatalf("%s returned %d %v, want %d", tc.name, code, body, tc.want)
			}
		})
	}

	// The last active administrator cannot be removed, demoted, disabled or
	// stripped of every sign-in route.
	ownerID := owner["user"].(map[string]any)["id"].(string)
	for _, tc := range []struct {
		path string
		body any
	}{
		{"/api/users/" + ownerID + "/role", map[string]string{"role": "developer"}},
		{"/api/users/" + ownerID + "/disable", map[string]any{}},
		{"/api/users/" + ownerID + "/delete", map[string]any{}},
		{"/api/users/" + ownerID + "/password/remove", map[string]any{}},
		{"/api/auth/password-policy", map[string]bool{"enabled": false}},
	} {
		code, body, _ := admin.call(t, fx, "POST", tc.path, tc.body, csrf)
		if code != 409 || body["error"].(map[string]any)["code"] != "last_admin" {
			t.Fatalf("%s did not protect the last admin: %d %v", tc.path, code, body)
		}
	}

	// A second administrator keeps the instance recoverable while the first
	// administrator can move to an OAuth-only route.
	second := fx.provision(t, admin, csrf, "second@example.test", "admin", "second-password-123")
	_ = second
	if code, body, _ := admin.call(t, fx, "POST", "/api/auth/providers/google", map[string]any{"clientId": "google-client", "clientSecret": "google-secret", "enabled": true}, csrf); code != 200 {
		t.Fatalf("provider save failed: %d %v", code, body)
	}
	if location := admin.oauth(t, fx, "google", "owner@example.test"); location != "/" {
		t.Fatalf("owner google login redirected to %q", location)
	}
	csrf = admin.session(t, fx)["csrfToken"].(string)
	code, body, _ := admin.call(t, fx, "POST", "/api/auth/password-policy", map[string]bool{"enabled": false}, csrf)
	if code != 200 {
		t.Fatalf("password login could not be disabled with a provider route: %d %v", code, body)
	}
	if code, body, _ := newTestClient(t).call(t, fx, "POST", "/api/auth/login", map[string]string{"email": "second@example.test", "password": "second-password-123"}, ""); code != 403 || body["error"].(map[string]any)["code"] != "password_login_disabled" {
		t.Fatalf("password login survived the instance toggle: %d %v", code, body)
	}
	// The only remaining route for the owner is Google, so removing it would
	// strand the owner and is rejected.
	if code, body, _ := admin.call(t, fx, "POST", "/api/auth/providers/google/remove", map[string]any{}, csrf); code != 409 {
		t.Fatalf("removing the last admin route was allowed: %d %v", code, body)
	}
}

func TestTwoFactorEnforcementWithOAuthOnlyAccount(t *testing.T) {
	fx := newAuthFixture(t)
	admin := newTestClient(t)
	owner := admin.login(t, fx, "owner@example.test", "administrator-password-123", 200)
	csrf := owner["csrfToken"].(string)
	fx.providerSettings(t, admin, csrf)

	code, tokenBody, _ := admin.call(t, fx, "POST", "/api/operation-tokens", map[string]any{"name": "Before enforcement", "scopes": []string{"read"}, "applications": []string{}, "days": 1}, csrf)
	if code != 201 {
		t.Fatalf("token creation failed: %d %v", code, tokenBody)
	}
	token := tokenBody["token"].(string)
	if status := admin.bearer(t, fx, "/api/v1/capabilities", token); status != 200 {
		t.Fatalf("unaffected token was denied: %d", status)
	}

	// Enable enforcement. The current administrator is immediately gated.
	policy := admin.call
	code, body, _ := policy(t, fx, "POST", "/api/auth/factor/change", map[string]any{"action": "policy", "required": true, "password": "administrator-password-123"}, csrf)
	if code != 200 || body["authStep"] != "enroll" {
		t.Fatalf("enforcement did not gate the administrator: %d %v", code, body)
	}
	if status := admin.bearer(t, fx, "/api/v1/capabilities", token); status != 401 {
		t.Fatalf("personal token bypassed required enrollment: %d", status)
	}
	start := map[string]any{}
	code, start, _ = admin.call(t, fx, "POST", "/api/auth/factor/start", map[string]string{"password": "administrator-password-123"}, body["csrfToken"].(string))
	if code != 200 {
		t.Fatalf("administrator enrollment start failed: %d %v", code, start)
	}
	confirmed := map[string]any{}
	code, confirmed, _ = admin.call(t, fx, "POST", "/api/auth/factor/confirm", map[string]string{"code": testAuthenticatorCode(start["key"].(string), time.Now())}, body["csrfToken"].(string))
	if code != 200 || confirmed["authStep"] != nil {
		t.Fatalf("administrator enrollment failed: %d %v", code, confirmed)
	}
	csrf = confirmed["csrfToken"].(string)
	if status := admin.bearer(t, fx, "/api/v1/capabilities", token); status != 200 {
		t.Fatalf("token did not recover after enrollment: %d", status)
	}

	// An OAuth-only user is provisioned without a password and must enroll.
	fx.provision(t, admin, csrf, "oauth@example.test", "developer", "")
	person := newTestClient(t)
	if location := person.oauth(t, fx, "google", "oauth@example.test"); location != "/" {
		t.Fatalf("oauth-only login redirected to %q", location)
	}
	oauthSession := person.session(t, fx)
	if oauthSession["authStep"] != "enroll" {
		t.Fatalf("oauth-only account bypassed enforcement: %v", oauthSession)
	}
	if code, _, _ := person.call(t, fx, "GET", "/api/applications", nil, ""); code != 403 {
		t.Fatalf("oauth-only account reached applications before enrollment: %d", code)
	}
	personCsrf := oauthSession["csrfToken"].(string)
	personStart := map[string]any{}
	code, personStart, _ = person.call(t, fx, "POST", "/api/auth/factor/start", map[string]string{}, personCsrf)
	if code != 200 {
		t.Fatalf("passwordless enrollment start failed: %d %v", code, personStart)
	}
	personConfirmed := map[string]any{}
	code, personConfirmed, _ = person.call(t, fx, "POST", "/api/auth/factor/confirm", map[string]string{"code": testAuthenticatorCode(personStart["key"].(string), time.Now())}, personCsrf)
	if code != 200 || personConfirmed["authStep"] != nil {
		t.Fatalf("passwordless enrollment failed: %d %v", code, personConfirmed)
	}
	if code, _, _ := person.call(t, fx, "GET", "/api/applications", nil, ""); code != 200 {
		t.Fatalf("enrolled oauth-only account was denied: %d", code)
	}
	personCsrf = personConfirmed["csrfToken"].(string)
	if code, body, _ := person.call(t, fx, "POST", "/api/auth/factor/change", map[string]any{"action": "disable", "code": testAuthenticatorCode(personStart["key"].(string), time.Now())}, personCsrf); code != 409 {
		t.Fatalf("required factor was removed: %d %v", code, body)
	}

	// Existing sessions are re-evaluated and a new password login goes straight
	// into enrollment when the account has no factor yet.
	fx.provision(t, admin, csrf, "fresh@example.test", "viewer", "fresh-password-123")
	fresh := newTestClient(t)
	freshLogin := fresh.login(t, fx, "fresh@example.test", "fresh-password-123", 200)
	if freshLogin["authStep"] != "password_change" {
		t.Fatalf("fresh account did not get the temporary password gate first: %v", freshLogin)
	}
}
