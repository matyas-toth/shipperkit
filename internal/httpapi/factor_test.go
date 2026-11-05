package httpapi

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"shipper/internal/auth"
	"shipper/internal/events"
	"shipper/internal/inventory"
	"shipper/internal/store"
)

func testAuthenticatorCode(secret string, at time.Time) string {
	key, _ := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(at.Unix()/30))
	m := hmac.New(sha1.New, key)
	m.Write(b[:])
	sum := m.Sum(nil)
	pos := sum[len(sum)-1] & 15
	return fmt.Sprintf("%06d", (binary.BigEndian.Uint32(sum[pos:pos+4])&0x7fffffff)%1000000)
}

func TestFactorGateAndSingleUseRecovery(t *testing.T) {
	ctx := context.Background()
	db, e := store.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	password := "fixture-long-password"
	hash, e := auth.Password(password)
	if e != nil {
		t.Fatal(e)
	}
	if e = db.CreateAdmin(ctx, store.User{ID: "owner", Email: "owner@example.test", PasswordHash: hash}, "", "", 0); e != nil {
		t.Fatal(e)
	}
	if v, e := db.ACMEContact(ctx); e != nil || v != "owner@example.test" {
		t.Fatal("First administrator contact was not persisted", v, e)
	}
	hub := events.New()
	monitor := inventory.New(testEngine{}, db, hub)
	server := httptest.NewServer(New(db, monitor, hub, Config{Assets: http.NotFoundHandler()}))
	defer server.Close()
	newClient := func() *http.Client {
		jar, _ := cookiejar.New(nil)
		return &http.Client{Jar: jar, Timeout: 5 * time.Second}
	}
	type sessionJSON struct {
		CSRF  string   `json:"csrfToken"`
		Step  string   `json:"authStep"`
		Key   string   `json:"key"`
		QR    string   `json:"qr"`
		Codes []string `json:"recoveryCodes"`
	}
	call := func(c *http.Client, method, route, csrf string, body any, want int) sessionJSON {
		t.Helper()
		data, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, server.URL+route, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", csrf)
		r, err := c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Body.Close()
		if r.StatusCode != want {
			t.Fatalf("%s %s returned %d, want %d", method, route, r.StatusCode, want)
		}
		var result sessionJSON
		_ = json.NewDecoder(r.Body).Decode(&result)
		return result
	}
	login := func(c *http.Client) sessionJSON {
		return call(c, "POST", "/api/auth/login", "", map[string]string{"email": "owner@example.test", "password": password}, 200)
	}
	c, existing := newClient(), newClient()
	s := login(c)
	login(existing)
	call(c, "GET", "/api/applications", "", nil, 200)
	policy := call(c, "POST", "/api/auth/factor/change", s.CSRF, map[string]any{"action": "policy", "required": true, "password": password}, 200)
	if policy.Step != "enroll" {
		t.Fatal("Policy did not gate the current session")
	}
	call(existing, "GET", "/api/applications", "", nil, 403)
	call(existing, "GET", "/api/events", "", nil, 403)
	start := call(c, "POST", "/api/auth/factor/start", s.CSRF, map[string]string{"password": password}, 200)
	if start.Key == "" || !strings.HasPrefix(start.QR, "data:image/png;base64,") {
		t.Fatal("Authenticator setup did not return a QR and key")
	}
	code := testAuthenticatorCode(start.Key, time.Now())
	confirmed := call(c, "POST", "/api/auth/factor/confirm", s.CSRF, map[string]string{"code": code}, 200)
	if len(confirmed.Codes) != 8 || confirmed.Step != "" {
		t.Fatal("Confirmation did not complete enrollment")
	}
	call(c, "GET", "/api/applications", "", nil, 200)
	call(existing, "GET", "/api/applications", "", nil, 401)
	challenge := login(existing)
	if challenge.Step != "verify" {
		t.Fatal("Password granted access without the authenticator")
	}
	call(existing, "GET", "/api/applications", "", nil, 403)
	call(existing, "POST", "/api/auth/factor/verify", challenge.CSRF, map[string]string{"code": code}, 422)
	call(existing, "POST", "/api/auth/factor/verify", challenge.CSRF, map[string]string{"code": confirmed.Codes[0]}, 200)
	call(existing, "GET", "/api/applications", "", nil, 200)
	challenge = login(existing)
	call(existing, "POST", "/api/auth/factor/verify", challenge.CSRF, map[string]string{"code": confirmed.Codes[0]}, 422)
	call(c, "POST", "/api/auth/factor/change", confirmed.CSRF, map[string]string{"action": "disable", "password": password, "code": code}, 409)
	if err := db.RecoverFactor(ctx, "owner@example.test"); err != nil {
		t.Fatal(err)
	}
	call(c, "GET", "/api/applications", "", nil, 401)
	if result := login(c); result.Step != "enroll" {
		t.Fatal("Offline recovery disabled enforcement")
	}
}
