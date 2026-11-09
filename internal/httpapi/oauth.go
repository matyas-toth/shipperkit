package httpapi

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"shipper/internal/auth"
	"shipper/internal/store"
)

const oauthStateCookie = "shipper_oauth_state"

// OAuthEndpoints is overridable so automated tests can exercise complete
// authorization flows against local provider doubles. Defaults are the public
// provider endpoints documented in docs/authentication.md.
type OAuthEndpoints struct {
	GoogleAuthorize string
	GoogleToken     string
	GoogleJWKS      string
	GoogleIssuer    string
	GitHubAuthorize string
	GitHubToken     string
	GitHubAPI       string
	DiscordAuthorize string
	DiscordToken     string
	DiscordAPI       string
	Client           *http.Client
}

func DefaultOAuthEndpoints() *OAuthEndpoints {
	return &OAuthEndpoints{
		GoogleAuthorize:  "https://accounts.google.com/o/oauth2/v2/auth",
		GoogleToken:      "https://oauth2.googleapis.com/token",
		GoogleJWKS:       "https://www.googleapis.com/oauth2/v3/certs",
		GoogleIssuer:     "https://accounts.google.com",
		GitHubAuthorize:  "https://github.com/login/oauth/authorize",
		GitHubToken:      "https://github.com/login/oauth/access_token",
		GitHubAPI:        "https://api.github.com",
		DiscordAuthorize: "https://discord.com/oauth2/authorize",
		DiscordToken:     "https://discord.com/api/oauth2/token",
		DiscordAPI:       "https://discord.com/api",
	}
}

func (s *Server) oauthClient() *http.Client {
	if s.oauth != nil && s.oauth.Client != nil {
		return s.oauth.Client
	}
	return &http.Client{Timeout: 12 * time.Second}
}

type oauthSpec struct {
	Provider  string
	Authorize string
	Token     string
	API       string
	JWKS      string
	Issuer    string
	Scopes    []string
	PKCE      bool
}

func (s *Server) oauthSpec(provider string) (oauthSpec, bool) {
	if s.oauth == nil {
		s.oauth = DefaultOAuthEndpoints()
	}
	switch provider {
	case "google":
		return oauthSpec{Provider: provider, Authorize: s.oauth.GoogleAuthorize, Token: s.oauth.GoogleToken, JWKS: s.oauth.GoogleJWKS, Issuer: s.oauth.GoogleIssuer, Scopes: []string{"openid", "email", "profile"}, PKCE: true}, true
	case "github":
		return oauthSpec{Provider: provider, Authorize: s.oauth.GitHubAuthorize, Token: s.oauth.GitHubToken, API: s.oauth.GitHubAPI, Scopes: []string{"read:user", "user:email"}}, true
	case "discord":
		return oauthSpec{Provider: provider, Authorize: s.oauth.DiscordAuthorize, Token: s.oauth.DiscordToken, API: s.oauth.DiscordAPI, Scopes: []string{"identify", "email"}, PKCE: true}, true
	}
	return oauthSpec{}, false
}

func (s *Server) oauthCallbackURL(provider string) string {
	base := strings.TrimSuffix(s.publicURL(), "/")
	if base == "" {
		base = "https://localhost"
	}
	return base + "/auth/oauth/" + provider + "/callback"
}

func (s *Server) setOAuthStateCookie(w http.ResponseWriter, r *http.Request, state string) {
	secure := s.secureRequest(r) || strings.HasPrefix(s.publicURL(), "https")
	http.SetCookie(w, &http.Cookie{Name: oauthStateCookie, Value: state, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: 600, Expires: time.Now().Add(10 * time.Minute)})
}

func (s *Server) clearOAuthStateCookie(w http.ResponseWriter, r *http.Request) {
	secure := s.secureRequest(r) || strings.HasPrefix(s.publicURL(), "https")
	http.SetCookie(w, &http.Cookie{Name: oauthStateCookie, Value: "", Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0)})
}

// oauthBegin records an expiring one-use state and returns the provider URL and
// the raw state value that must be kept in the browser state cookie.
func (s *Server) oauthBegin(ctx context.Context, spec oauthSpec, info store.AuthProvider, mode, userID string) (string, string, error) {
	callback := s.oauthCallbackURL(spec.Provider)
	raw, nonce := auth.Token(), auth.RandomBase64(24)
	verifier := ""
	query := url.Values{"client_id": {info.ClientID}, "redirect_uri": {callback}, "response_type": {"code"}, "scope": {strings.Join(spec.Scopes, " ")}, "state": {raw}}
	if spec.PKCE {
		verifier = auth.PKCEVerifier()
		query.Set("code_challenge", auth.PKCEChallenge(verifier))
		query.Set("code_challenge_method", "S256")
	}
	if spec.Provider == "google" {
		query.Set("nonce", nonce)
		query.Set("access_type", "online")
		query.Set("prompt", "select_account")
	}
	now := time.Now()
	state := store.OAuthState{Provider: spec.Provider, Nonce: nonce, Verifier: verifier, Redirect: callback, Mode: mode, UserID: userID, ExpiresAt: now.Add(10 * time.Minute).Unix()}
	if err := s.db.SaveOAuthState(ctx, auth.Hash(raw), state, now); err != nil {
		return "", "", err
	}
	return spec.Authorize + "?" + query.Encode(), raw, nil
}

func (s *Server) oauthStart(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	spec, ok := s.oauthSpec(provider)
	if !ok {
		s.oauthFail(w, r, provider, "login", "provider_not_found")
		return
	}
	info, err := s.db.AuthProvider(r.Context(), provider)
	if err != nil {
		s.oauthFail(w, r, provider, "login", "provider_error")
		return
	}
	if !info.Enabled || info.ClientID == "" || !info.HasSecret {
		s.oauthFail(w, r, provider, "login", "provider_disabled")
		return
	}
	target, state, err := s.oauthBegin(r.Context(), spec, info, "login", "")
	if err != nil {
		s.oauthFail(w, r, provider, "login", "provider_error")
		return
	}
	s.setOAuthStateCookie(w, r, state)
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, target, http.StatusSeeOther)
}

func (s *Server) oauthFail(w http.ResponseWriter, r *http.Request, provider, mode, code string) {
	s.clearOAuthStateCookie(w, r)
	w.Header().Set("Cache-Control", "no-store")
	if mode == "test" {
		http.Redirect(w, r, "/settings?oauth="+url.QueryEscape(provider)+"&oauth_result=error&oauth_code="+url.QueryEscape(code)+"#authentication", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/?auth_error="+url.QueryEscape(code), http.StatusSeeOther)
}

func (s *Server) oauthCallback(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	spec, ok := s.oauthSpec(provider)
	if !ok {
		s.oauthFail(w, r, provider, "login", "provider_not_found")
		return
	}
	query := r.URL.Query()
	raw := query.Get("state")
	cookie, err := r.Cookie(oauthStateCookie)
	if err != nil || raw == "" || !auth.Equal(raw, cookie.Value) {
		s.oauthFail(w, r, provider, "login", "state_invalid")
		return
	}
	state, err := s.db.ConsumeOAuthState(r.Context(), auth.Hash(raw), time.Now())
	if err != nil {
		s.oauthFail(w, r, provider, "login", "state_invalid")
		return
	}
	if state.Provider != provider {
		s.oauthFail(w, r, provider, state.Mode, "state_invalid")
		return
	}
	if query.Get("error") != "" || query.Get("code") == "" {
		s.oauthFail(w, r, provider, state.Mode, "access_denied")
		return
	}
	info, err := s.db.AuthProvider(r.Context(), provider)
	if err != nil || !info.Enabled || info.ClientID == "" || !info.HasSecret {
		s.oauthFail(w, r, provider, state.Mode, "provider_disabled")
		return
	}
	identity, err := s.exchangeOAuth(r.Context(), spec, state, query.Get("code"))
	if err != nil {
		s.oauthFail(w, r, provider, state.Mode, oauthErrorCode(err))
		return
	}
	user, pending, err := s.resolveOAuthIdentity(r.Context(), identity)
	if err != nil {
		s.oauthFail(w, r, provider, state.Mode, oauthErrorCode(err))
		return
	}
	if state.Mode == "test" {
		if user.ID != state.UserID {
			s.oauthFail(w, r, provider, state.Mode, "wrong_account")
			return
		}
		if err = s.db.RecordProviderTest(r.Context(), user.ID, provider, time.Now()); err != nil {
			s.oauthFail(w, r, provider, state.Mode, "provider_error")
			return
		}
		s.clearOAuthStateCookie(w, r)
		http.Redirect(w, r, "/settings?oauth="+url.QueryEscape(provider)+"&oauth_result=success#authentication", http.StatusSeeOther)
		return
	}
	if user.Disabled {
		s.oauthFail(w, r, provider, state.Mode, "account_disabled")
		return
	}
	factor, err := s.db.FactorState(r.Context(), user.ID)
	if err != nil {
		s.oauthFail(w, r, provider, state.Mode, "provider_error")
		return
	}
	restricted := user.PasswordChangeRequired || factor.Enrolled || factor.Required
	lifetime := 7 * 24 * time.Hour
	if restricted {
		lifetime = 10 * time.Minute
	}
	token, csrf := auth.Token(), auth.Token()
	hash := auth.Hash(token)
	if err = s.db.CreateSession(r.Context(), hash, user.ID, csrf, time.Now().Add(lifetime).Unix(), provider, pending); err != nil {
		s.oauthFail(w, r, provider, state.Mode, "provider_error")
		return
	}
	if !restricted {
		if err = s.db.FinalizeSessionIdentity(r.Context(), hash, time.Now()); err != nil {
			_ = s.db.DeleteSession(r.Context(), hash)
			s.oauthFail(w, r, provider, state.Mode, oauthErrorCode(err))
			return
		}
	}
	if err = s.db.RecordLogin(r.Context(), user.ID, provider, time.Now()); err != nil {
		s.oauthFail(w, r, provider, state.Mode, "provider_error")
		return
	}
	s.clearOAuthStateCookie(w, r)
	s.cookie(w, r, token, time.Now().Add(lifetime).Unix())
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func oauthErrorCode(err error) string {
	var coded oauthCodeError
	if errors.As(err, &coded) {
		return string(coded)
	}
	switch {
	case errors.Is(err, store.ErrIdentityConflict), errors.Is(err, store.ErrProviderLinked):
		return "identity_conflict"
	}
	return "provider_error"
}

type oauthCodeError string

func (e oauthCodeError) Error() string { return string(e) }

const (
	errOAuthEmailMissing  oauthCodeError = "email_missing"
	errOAuthEmailUnverified oauthCodeError = "email_unverified"
	errOAuthUnprovisioned oauthCodeError = "unprovisioned"
	errOAuthAmbiguous     oauthCodeError = "ambiguous"
	errOAuthAccountDisabled oauthCodeError = "account_disabled"
)

type oauthIdentity struct {
	Provider string
	Subject  string
	Email    string
	Emails   []string
	Name     string
}

func (s *Server) resolveOAuthIdentity(ctx context.Context, identity oauthIdentity) (store.User, *store.Identity, error) {
	if identity.Subject == "" {
		return store.User{}, nil, oauthCodeError("provider_error")
	}
	user, err := s.db.IdentityUser(ctx, identity.Provider, identity.Subject)
	if err == nil {
		if user.Disabled {
			return user, nil, errOAuthAccountDisabled
		}
		_ = s.db.TouchIdentity(ctx, identity.Provider, identity.Subject, time.Now())
		return user, nil, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return store.User{}, nil, err
	}
	emails := normalizedEmails(identity.Emails)
	if len(emails) == 0 {
		return store.User{}, nil, errOAuthEmailMissing
	}
	users, err := s.db.UsersByEmail(ctx, emails)
	if err != nil {
		return store.User{}, nil, err
	}
	if len(users) == 0 {
		return store.User{}, nil, errOAuthUnprovisioned
	}
	if len(users) > 1 {
		return store.User{}, nil, errOAuthAmbiguous
	}
	existing, err := s.db.UserIdentity(ctx, users[0].ID, identity.Provider)
	if err == nil && existing.Subject != identity.Subject {
		return store.User{}, nil, errOAuthAmbiguous
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return store.User{}, nil, err
	}
	email := identity.Email
	if email == "" {
		email = emails[0]
	}
	return users[0], &store.Identity{Provider: identity.Provider, Subject: identity.Subject, Email: email}, nil
}

func normalizedEmails(values []string) []string {
	seen := map[string]bool{}
	items := []string{}
	for _, value := range values {
		email := normalizeEmail(value)
		if !validEmail(email) || seen[email] {
			continue
		}
		seen[email] = true
		items = append(items, email)
	}
	return items
}

func (s *Server) exchangeOAuth(ctx context.Context, spec oauthSpec, state store.OAuthState, code string) (oauthIdentity, error) {
	secret, err := s.db.AuthProviderSecret(ctx, spec.Provider)
	if err != nil || secret == "" {
		return oauthIdentity{}, oauthCodeError("provider_error")
	}
	info, err := s.db.AuthProvider(ctx, spec.Provider)
	if err != nil || info.ClientID == "" {
		return oauthIdentity{}, oauthCodeError("provider_error")
	}
	switch spec.Provider {
	case "google":
		return s.googleIdentity(ctx, spec, info.ClientID, secret, state, code)
	case "github":
		return s.gitHubIdentity(ctx, spec, info.ClientID, secret, state, code)
	case "discord":
		return s.discordIdentity(ctx, spec, info.ClientID, secret, state, code)
	}
	return oauthIdentity{}, oauthCodeError("provider_not_found")
}

func (s *Server) postForm(ctx context.Context, endpoint string, values url.Values, accept string) (map[string]any, error) {
	request, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", accept)
	request.Header.Set("User-Agent", "Shipper")
	response, err := s.oauthClient().Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("provider token exchange returned %d", response.StatusCode)
	}
	valuesOut := map[string]any{}
	if err = json.Unmarshal(body, &valuesOut); err != nil {
		return nil, err
	}
	return valuesOut, nil
}

func (s *Server) getJSON(ctx context.Context, endpoint, accessToken string, out any) error {
	request, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Accept", "application/vnd.github+json, application/json")
	request.Header.Set("User-Agent", "Shipper")
	response, err := s.oauthClient().Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("provider identity request returned %d", response.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(out)
}

func stringValue(value any) string {
	switch item := value.(type) {
	case string:
		return item
	case json.Number:
		return item.String()
	case float64:
		return strconv.FormatInt(int64(item), 10)
	}
	return ""
}

func (s *Server) googleIdentity(ctx context.Context, spec oauthSpec, clientID, secret string, state store.OAuthState, code string) (oauthIdentity, error) {
	values := url.Values{"code": {code}, "client_id": {clientID}, "client_secret": {secret}, "redirect_uri": {state.Redirect}, "grant_type": {"authorization_code"}}
	if state.Verifier != "" {
		values.Set("code_verifier", state.Verifier)
	}
	result, err := s.postForm(ctx, spec.Token, values, "application/json")
	if err != nil {
		return oauthIdentity{}, err
	}
	idToken := stringValue(result["id_token"])
	if idToken == "" {
		return oauthIdentity{}, oauthCodeError("provider_error")
	}
	claims, err := s.verifyGoogleToken(ctx, spec, clientID, state.Nonce, idToken)
	if err != nil {
		return oauthIdentity{}, err
	}
	if !emailVerified(claims["email_verified"]) {
		return oauthIdentity{}, errOAuthEmailUnverified
	}
	email := normalizeEmail(stringValue(claims["email"]))
	if !validEmail(email) {
		return oauthIdentity{}, errOAuthEmailMissing
	}
	subject := stringValue(claims["sub"])
	if subject == "" {
		return oauthIdentity{}, oauthCodeError("provider_error")
	}
	name := stringValue(claims["name"])
	return oauthIdentity{Provider: "google", Subject: subject, Email: email, Emails: []string{email}, Name: name}, nil
}

func (s *Server) gitHubIdentity(ctx context.Context, spec oauthSpec, clientID, secret string, state store.OAuthState, code string) (oauthIdentity, error) {
	values := url.Values{"client_id": {clientID}, "client_secret": {secret}, "code": {code}, "redirect_uri": {state.Redirect}}
	result, err := s.postForm(ctx, spec.Token, values, "application/json")
	if err != nil {
		return oauthIdentity{}, err
	}
	token := stringValue(result["access_token"])
	if token == "" {
		return oauthIdentity{}, oauthCodeError("provider_error")
	}
	var profile struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
	}
	if err = s.getJSON(ctx, strings.TrimSuffix(spec.API, "/")+"/user", token, &profile); err != nil {
		return oauthIdentity{}, err
	}
	if profile.ID == 0 {
		return oauthIdentity{}, oauthCodeError("provider_error")
	}
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err = s.getJSON(ctx, strings.TrimSuffix(spec.API, "/")+"/user/emails", token, &emails); err != nil {
		return oauthIdentity{}, err
	}
	verified := []string{}
	primary := ""
	for _, item := range emails {
		if !item.Verified {
			continue
		}
		email := normalizeEmail(item.Email)
		if !validEmail(email) {
			continue
		}
		verified = append(verified, email)
		if item.Primary {
			primary = email
		}
	}
	if len(verified) == 0 {
		return oauthIdentity{}, errOAuthEmailUnverified
	}
	return oauthIdentity{Provider: "github", Subject: strconv.FormatInt(profile.ID, 10), Email: primary, Emails: verified, Name: profile.Name}, nil
}

func (s *Server) discordIdentity(ctx context.Context, spec oauthSpec, clientID, secret string, state store.OAuthState, code string) (oauthIdentity, error) {
	values := url.Values{"client_id": {clientID}, "client_secret": {secret}, "grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {state.Redirect}}
	if state.Verifier != "" {
		values.Set("code_verifier", state.Verifier)
	}
	result, err := s.postForm(ctx, spec.Token, values, "application/json")
	if err != nil {
		return oauthIdentity{}, err
	}
	token := stringValue(result["access_token"])
	if token == "" {
		return oauthIdentity{}, oauthCodeError("provider_error")
	}
	var profile struct {
		ID       string `json:"id"`
		Email    string `json:"email"`
		Verified bool   `json:"verified"`
		Username string `json:"username"`
	}
	if err = s.getJSON(ctx, strings.TrimSuffix(spec.API, "/")+"/users/@me", token, &profile); err != nil {
		return oauthIdentity{}, err
	}
	email := normalizeEmail(profile.Email)
	if email == "" {
		return oauthIdentity{}, errOAuthEmailMissing
	}
	if !profile.Verified {
		return oauthIdentity{}, errOAuthEmailUnverified
	}
	if !validEmail(email) || profile.ID == "" {
		return oauthIdentity{}, oauthCodeError("provider_error")
	}
	return oauthIdentity{Provider: "discord", Subject: profile.ID, Email: email, Emails: []string{email}, Name: profile.Username}, nil
}

func emailVerified(value any) bool {
	switch item := value.(type) {
	case bool:
		return item
	case string:
		return strings.EqualFold(item, "true")
	}
	return false
}

// GOOGLE ID TOKEN VALIDATION

type jwksKey struct {
	Key rsa.PublicKey
}

func (s *Server) verifyGoogleToken(ctx context.Context, spec oauthSpec, clientID, nonce, raw string) (map[string]any, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, oauthCodeError("provider_error")
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, oauthCodeError("provider_error")
	}
	var header struct {
		Algorithm string `json:"alg"`
		KeyID     string `json:"kid"`
	}
	if json.Unmarshal(headerJSON, &header) != nil || header.Algorithm != "RS256" || header.KeyID == "" {
		return nil, oauthCodeError("provider_error")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, oauthCodeError("provider_error")
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	keys, err := s.googleKeys(ctx, spec)
	if err != nil {
		return nil, err
	}
	key, ok := keys[header.KeyID]
	if !ok {
		return nil, oauthCodeError("provider_error")
	}
	if err = rsa.VerifyPKCS1v15(&key.Key, crypto.SHA256, sum[:], signature); err != nil {
		return nil, oauthCodeError("provider_error")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, oauthCodeError("provider_error")
	}
	claims := map[string]any{}
	if err = json.Unmarshal(payload, &claims); err != nil {
		return nil, oauthCodeError("provider_error")
	}
	issuer := stringValue(claims["iss"])
	if issuer != spec.Issuer && issuer != "accounts.google.com" && issuer != "https://accounts.google.com" {
		return nil, oauthCodeError("provider_error")
	}
	if !audienceMatches(claims["aud"], clientID) {
		return nil, oauthCodeError("provider_error")
	}
	expires := int64(numberValue(claims["exp"]))
	issued := int64(numberValue(claims["iat"]))
	now := time.Now().Unix()
	if expires == 0 || expires < now-30 || issued > now+60 {
		return nil, oauthCodeError("provider_error")
	}
	if subtle.ConstantTimeCompare([]byte(stringValue(claims["nonce"])), []byte(nonce)) != 1 {
		return nil, oauthCodeError("provider_error")
	}
	return claims, nil
}

func audienceMatches(value any, clientID string) bool {
	switch item := value.(type) {
	case string:
		return item == clientID
	case []any:
		for _, entry := range item {
			if stringValue(entry) == clientID {
				return true
			}
		}
	}
	return false
}

func numberValue(value any) float64 {
	switch item := value.(type) {
	case float64:
		return item
	case json.Number:
		value, _ := item.Float64()
		return value
	case int64:
		return float64(item)
	}
	return 0
}

func (s *Server) googleKeys(ctx context.Context, spec oauthSpec) (map[string]jwksKey, error) {
	s.jwksMu.Lock()
	defer s.jwksMu.Unlock()
	if s.jwks != nil && time.Since(s.jwksFetched) < 15*time.Minute {
		return s.jwks, nil
	}
	request, err := http.NewRequestWithContext(ctx, "GET", spec.JWKS, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "Shipper")
	response, err := s.oauthClient().Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, oauthCodeError("provider_error")
	}
	var document struct {
		Keys []struct {
			KeyID string `json:"kid"`
			Type  string `json:"kty"`
			Use   string `json:"use"`
			N     string `json:"n"`
			E     string `json:"e"`
		} `json:"keys"`
	}
	if err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&document); err != nil {
		return nil, err
	}
	keys := map[string]jwksKey{}
	for _, item := range document.Keys {
		if item.Type != "RSA" || (item.Use != "" && item.Use != "sig") {
			continue
		}
		n, err := base64.RawURLEncoding.DecodeString(item.N)
		if err != nil {
			continue
		}
		e, err := base64.RawURLEncoding.DecodeString(item.E)
		if err != nil || len(e) == 0 || len(e) > 4 {
			continue
		}
		exponent := 0
		for _, value := range e {
			exponent = exponent<<8 | int(value)
		}
		if exponent < 3 {
			continue
		}
		keys[item.KeyID] = jwksKey{Key: rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exponent}}
	}
	if len(keys) == 0 {
		return nil, oauthCodeError("provider_error")
	}
	s.jwks, s.jwksFetched = keys, time.Now()
	return keys, nil
}
