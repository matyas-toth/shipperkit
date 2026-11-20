package httpapi

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"shipper/internal/store"
)

func TestCopiedConnectionStringsConnectAndDeletedConnectionsDrop(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	database := store.Application{ID: "aaaaaaaaaaaaaaaaaaaaaaaa", Name: "Mongo", Config: store.Config{Method: "image", Image: "mongo:8", Database: &store.DatabaseResource{Engine: "mongodb", Version: "8"}}}
	if _, err = db.Enqueue(ctx, database, store.Deployment{ID: "database-release", Operation: "deploy"}, "", true); err != nil {
		t.Fatal(err)
	}
	s := &Server{db: db}
	deleted := "bbbbbbbbbbbbbbbbbbbbbbbb"
	c := store.Config{
		Variables:  []store.Variable{{Name: "DATABASE_URL", Value: "mongodb://root:secret@db-aaaaaaaaaaaaaaaaaaaaaaaa:27017/app?authSource=admin", Scope: "runtime"}},
		Networking: store.NetworkAccess{Applications: []string{deleted}},
	}
	recorder := httptest.NewRecorder()
	if !s.validateNetworkingConfig(recorder, &c, "cccccccccccccccccccccccc") {
		t.Fatalf("validation refused: %s", recorder.Body.String())
	}
	if !reflect.DeepEqual(c.Networking.Applications, []string{database.ID}) {
		t.Fatalf("connections %v", c.Networking.Applications)
	}
	changes := s.describeConnections(ctx, []string{"Environment variables", "Networking"}, store.NetworkAccess{Applications: []string{deleted}}, c.Networking)
	if !reflect.DeepEqual(changes, []string{"Environment variables", "Connects privately to Mongo", "Disconnects from a deleted application"}) {
		t.Fatalf("changes %v", changes)
	}
	// A host naming the application itself never links it to itself.
	self := store.Config{Variables: []store.Variable{{Name: "SELF", Value: "http://app-aaaaaaaaaaaaaaaaaaaaaaaa:8080"}}}
	if !s.validateNetworkingConfig(httptest.NewRecorder(), &self, database.ID) || len(self.Networking.Applications) != 0 {
		t.Fatalf("self link %v", self.Networking.Applications)
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
