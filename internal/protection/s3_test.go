package protection

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestS3SignedStorageAndTamper(t *testing.T) {
	var object []byte
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if len(r.Header.Get("X-Amz-Content-Sha256")) != 64 {
			t.Error("missing signed payload hash")
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential=access/") || !strings.Contains(r.URL.Path, "/bucket/shipper/instance/run.age") {
			t.Error("unsigned or incorrectly scoped object")
		}
		switch r.Method {
		case "PUT":
			object, _ = io.ReadAll(r.Body)
		case "GET":
			w.Write(object)
		case "DELETE":
			object = nil
		default:
			t.Error(r.Method)
		}
	}))
	defer server.Close()
	s := S3{Endpoint: server.URL, Region: "us-east-1", Bucket: "bucket", AccessKey: "access", SecretKey: "test-secret", Private: true}
	file := filepath.Join(t.TempDir(), "data")
	os.WriteFile(file, []byte("encrypted bytes"), 0600)
	_, hash, _ := Checksum(file)
	key := s.Key("instance", "run")
	if e := s.Put(context.Background(), key, file); e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(t.TempDir(), "download")
	if e := s.Get(context.Background(), key, out, hash); e != nil {
		t.Fatal(e)
	}
	mu.Lock()
	object[0] ^= 1
	mu.Unlock()
	if e := s.Get(context.Background(), key, filepath.Join(t.TempDir(), "tampered"), hash); e == nil {
		t.Fatal("tampered object accepted")
	}
	s.Private = false
	if e := s.Put(context.Background(), key, file); e == nil {
		t.Fatal("HTTP private endpoint allowed without opt-in")
	}
	s.Private = true
	e := s.Delete(context.Background(), key)
	mu.Lock()
	remaining := len(object)
	mu.Unlock()
	if e != nil || remaining != 0 {
		t.Fatal(e)
	}
}
func TestS3DoesNotFollowRedirects(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("credentials followed redirect") }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	s := S3{Endpoint: server.URL, Region: "region", Bucket: "bucket", AccessKey: "access", SecretKey: "secret", Private: true}
	file := filepath.Join(t.TempDir(), "file")
	os.WriteFile(file, []byte("x"), 0600)
	if e := s.Put(context.Background(), s.Key("i", "b"), file); e == nil {
		t.Fatal("redirect accepted")
	}
}
