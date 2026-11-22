package httpapi

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"shipper/internal/store"
)

func TestSharedNetworkIgnoresLinksAndClaimsDatabaseDomains(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	database := store.Application{ID: "aaaaaaaaaaaaaaaaaaaaaaaa", Name: "Orders", Config: store.Config{Method: "image", Image: "postgres:18", Database: &store.DatabaseResource{Engine: "postgresql", Version: "18", Domain: "db.example.test"}}}
	if _, err = db.Enqueue(ctx, database, store.Deployment{ID: "database-release", Operation: "deploy"}, "", true); err != nil {
		t.Fatal(err)
	}
	if err = db.SetActive(ctx, database.ID, "database-release"); err != nil {
		t.Fatal(err)
	}
	s := &Server{db: db}
	// The proxy holds 5432 for this database's own domain, so changing anything
	// else on it must not try to reserve the port again.
	own := database.Config
	own.Database = &store.DatabaseResource{Engine: "postgresql", Version: "18", Domain: "db.example.test"}
	own.MemoryMB = 256
	if recorder := httptest.NewRecorder(); !s.validateNetworkingConfig(recorder, &own, database.ID) {
		t.Fatalf("the serving domain blocked its own database: %d %s", recorder.Code, recorder.Body.String())
	}
	// Older clients still send the applications they connected. Every
	// application shares one network, so the list is dropped.
	c := store.Config{Networking: store.NetworkAccess{Applications: []string{database.ID}}}
	recorder := httptest.NewRecorder()
	if !s.validateNetworkingConfig(recorder, &c, "cccccccccccccccccccccccc") {
		t.Fatalf("validation refused: %s", recorder.Body.String())
	}
	if c.Networking.Applications != nil {
		t.Fatalf("connections %v", c.Networking.Applications)
	}
	// A second PostgreSQL database cannot take the same domain.
	other := store.Config{Method: "image", Database: &store.DatabaseResource{Engine: "postgresql", Version: "18", Domain: " DB.example.test. "}}
	recorder = httptest.NewRecorder()
	if s.validateNetworkingConfig(recorder, &other, "dddddddddddddddddddddddd") || recorder.Code != 409 || !strings.Contains(recorder.Body.String(), "Another database already uses this domain") {
		t.Fatalf("duplicate domain %d %s", recorder.Code, recorder.Body.String())
	}
	if other.Database.Domain != "db.example.test" {
		t.Fatalf("domain not normalized: %q", other.Database.Domain)
	}
	// The proxy serves PostgreSQL domains on 5432, so no application can publish it.
	published := store.Config{Method: "image", Networking: store.NetworkAccess{Ports: []store.PublishedPort{{Protocol: "tcp", BindIP: "0.0.0.0", HostPort: 5432, ContainerPort: 5432}}}}
	recorder = httptest.NewRecorder()
	if s.validateNetworkingConfig(recorder, &published, "eeeeeeeeeeeeeeeeeeeeeeee") || recorder.Code != 409 || !strings.Contains(recorder.Body.String(), "Port 5432 serves the public domain of Orders") {
		t.Fatalf("published domain port %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestDeployHookAcceptsABarePost(t *testing.T) {
	s := &Server{}
	for _, tc := range []struct {
		body string
		ok   bool
		rev  string
	}{{"", true, ""}, {"  \n", true, ""}, {`{"revision":"abc"}`, true, "abc"}, {"{", false, ""}} {
		var input struct {
			Revision string `json:"revision"`
		}
		req := httptest.NewRequest("POST", "/api/v1/applications/app/deploy", strings.NewReader(tc.body))
		if got := s.decodeOptional(httptest.NewRecorder(), req, &input); got != tc.ok || input.Revision != tc.rev {
			t.Fatalf("%q: ok %v revision %q", tc.body, got, input.Revision)
		}
	}
}
