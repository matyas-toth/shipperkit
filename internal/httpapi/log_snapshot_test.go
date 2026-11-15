package httpapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"shipper/internal/auth"
	"shipper/internal/deploy"
	"shipper/internal/events"
	"shipper/internal/store"
)

type snapshotLogRuntime struct{ deploy.Runtime }

func (snapshotLogRuntime) Logs(_ context.Context, _, _ string, _ int, emit func(string) error) error {
	return emit("live log")
}
func (snapshotLogRuntime) LogSnapshot(ctx context.Context, _, _ string, tail int, emit func(string) error) error {
	for i := 0; i < tail; i++ {
		if err := emit(fmt.Sprintf("secret-token line %d", i)); err != nil {
			return err
		}
	}
	return nil
}

func TestFiniteRuntimeLogsFinishRedactAndPreserveTheRequestedTail(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	cookie := auth.Token()
	if err = db.CreateAdmin(ctx, store.User{ID: "owner", Email: "owner@test.invalid", PasswordHash: "unused"}, auth.Hash(cookie), "csrf", time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	app := store.Application{ID: "app", Name: "App", SourceKind: "image", Method: "image", Config: store.Config{Image: "nginx:alpine", Variables: []store.Variable{{Name: "TOKEN", Value: "secret-token", Secret: true}}}}
	if _, err = db.Enqueue(ctx, app, store.Deployment{ID: "deployment", ApplicationID: app.ID, Operation: "deploy", Config: app.Config}, "", true); err != nil {
		t.Fatal(err)
	}
	hub := events.New()
	worker := deploy.NewWorker(db, snapshotLogRuntime{}, hub, t.TempDir())
	server := httptest.NewServer(New(db, nil, hub, Config{Worker: worker, Assets: http.NotFoundHandler()}))
	defer server.Close()
	for _, tail := range []int{0, 1, 500} {
		req, _ := http.NewRequest("GET", server.URL+"/api/applications/app/logs?container=owned&follow=false&tail="+strconv.Itoa(tail), nil)
		req.AddCookie(&http.Cookie{Name: cookieName, Value: cookie})
		client := &http.Client{Timeout: 5 * time.Second}
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != 200 || strings.Count(string(body), "event: log\n") != tail || !strings.Contains(string(body), "event: end\n") || strings.Contains(string(body), "secret-token") || strings.Contains(string(body), "live log") {
			t.Fatalf("incorrect finite logs (%d): %d %s", tail, res.StatusCode, body)
		}
	}
}
