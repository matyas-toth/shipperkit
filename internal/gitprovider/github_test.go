package gitprovider

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"shipper/internal/store"
	"strings"
	"testing"
	"time"
)

func TestAppTokenScopeCachingAndJWT(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	c := store.Connection{ID: "source", Kind: "github_app", UpdatedAt: 1, Secret: store.ConnectionSecret{AppID: 5, InstallationID: 12, PrivateKey: string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))}}
	jwt, e := AppJWT(c)
	if e != nil {
		t.Fatal(e)
	}
	parts := strings.Split(jwt, ".")
	raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var claims map[string]any
	json.Unmarshal(raw, &claims)
	if claims["iss"] != "5" || claims["exp"].(float64) > float64(time.Now().Add(10*time.Minute).Unix()) {
		t.Fatal(claims)
	}
	sig, _ := base64.RawURLEncoding.DecodeString(parts[2])
	hash := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, hash[:], sig) != nil {
		t.Fatal("invalid app signature")
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/app/installations/12/access_tokens" || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ey") {
			t.Error("wrong token endpoint")
		}
		var input struct {
			RepositoryIDs []int64 `json:"repository_ids"`
			Permissions   map[string]string
		}
		json.NewDecoder(r.Body).Decode(&input)
		if len(input.RepositoryIDs) != 1 || input.RepositoryIDs[0] != 55 || input.Permissions["contents"] != "read" {
			t.Error(input)
		}
		calls++
		fmt.Fprintf(w, `{"token":"short-lived-token","expires_at":%q}`, time.Now().Add(time.Hour).Format(time.RFC3339))
	}))
	defer server.Close()
	g := New()
	g.BaseURL = server.URL
	for range 2 {
		token, e := g.Token(context.Background(), c, 55)
		if e != nil || token != "short-lived-token" {
			t.Fatal(token, e)
		}
	}
	if calls != 1 {
		t.Fatal("token was not cached")
	}
	c.UpdatedAt++
	if _, e = g.Token(context.Background(), c, 55); e != nil || calls != 2 {
		t.Fatal("rotation reused old token", e)
	}
}
func TestRepositoryOriginCannotLeakCredentials(t *testing.T) {
	for _, url := range []string{"http://github.com/a/b", "https://github.com.evil.test/a/b", "https://token@github.com/a/b", "https://github.com/a/b?token=secret", "https://github.com/a/b/tree/main"} {
		if _, e := RepositoryPath(url); e == nil {
			t.Fatal("unsafe source accepted", url)
		}
	}
}
