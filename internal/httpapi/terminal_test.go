package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/net/websocket"
	"shipper/internal/auth"
	"shipper/internal/deploy"
	"shipper/internal/events"
	"shipper/internal/store"
)

type testTerminal struct {
	reader *io.PipeReader
	writer *io.PipeWriter
	closed chan struct{}
	once   sync.Once
}

func (t *testTerminal) Read(p []byte) (int, error)               { return t.reader.Read(p) }
func (t *testTerminal) Write(p []byte) (int, error)              { return len(p), nil }
func (t *testTerminal) Resize(context.Context, uint, uint) error { return nil }
func (t *testTerminal) Close() error {
	t.once.Do(func() { t.reader.Close(); t.writer.Close(); close(t.closed) })
	return nil
}

type terminalRuntime struct {
	deploy.Runtime
	mu        sync.Mutex
	shell     *testTerminal
	ownership bool
}

func (t *terminalRuntime) CheckTerminal(context.Context, string, string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.ownership {
		return io.ErrClosedPipe
	}
	return nil
}
func (t *terminalRuntime) OpenTerminal(context.Context, string, string, string, uint, uint) (deploy.Terminal, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	r, w := io.Pipe()
	t.shell = &testTerminal{reader: r, writer: w, closed: make(chan struct{})}
	return t.shell, nil
}

func TestTerminalPermissionsRevocationAndAudit(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	token, csrf := auth.Token(), auth.Token()
	ctx := context.Background()
	if err = db.CreateAdmin(ctx, store.User{ID: "owner", Email: "owner@test.invalid", PasswordHash: "unused"}, auth.Hash(token), csrf, time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	inventory := []store.Container{{ID: "owned", Name: "api", ApplicationID: "app", Managed: true, State: "running"}, {ID: "other", Name: "foreign", ApplicationID: "other-app", Managed: true, State: "running"}, {ID: "host", Name: "host", State: "running"}}
	if err = db.ReplaceInventory(ctx, inventory); err != nil {
		t.Fatal(err)
	}
	runtime := &terminalRuntime{ownership: true}
	worker := deploy.NewWorker(db, runtime, events.New(), t.TempDir())
	server := httptest.NewServer(New(db, nil, events.New(), Config{Worker: worker, Assets: http.NotFoundHandler()}))
	defer server.Close()
	request := func(path string, body any, credential, csrfValue, origin string) (int, map[string]any) {
		t.Helper()
		data, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", server.URL+path, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		if credential != "" {
			req.AddCookie(&http.Cookie{Name: cookieName, Value: credential})
		}
		req.Header.Set("X-CSRF-Token", csrfValue)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(response.Body).Decode(&out)
		return response.StatusCode, out
	}
	path := "/api/applications/app/terminal"
	for _, test := range []struct {
		name, container, token, csrf, origin string
		status                               int
	}{
		{"anonymous", "owned", "", "", "", 401}, {"csrf", "owned", token, "", "", 403}, {"cross-origin", "owned", token, csrf, "https://evil.invalid", 403}, {"other-application", "other", token, csrf, "", 403}, {"unmanaged", "host", token, csrf, "", 403},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, _ := request(path, map[string]any{"containerId": test.container}, test.token, test.csrf, test.origin)
			if status != test.status {
				t.Fatalf("got %d want %d", status, test.status)
			}
		})
	}
	open := func() (*websocket.Conn, string) {
		t.Helper()
		status, body := request(path, map[string]any{"containerId": "owned", "minutes": 1}, token, csrf, "")
		if status != 201 {
			t.Fatalf("start: %d %v", status, body)
		}
		id := body["id"].(string)
		config, err := websocket.NewConfig("ws"+strings.TrimPrefix(server.URL, "http")+path+"/"+id, server.URL)
		if err != nil {
			t.Fatal(err)
		}
		config.Header.Set("Cookie", cookieName+"="+token)
		ws, err := websocket.DialConfig(config)
		if err != nil {
			t.Fatal(err)
		}
		if err = websocket.JSON.Send(ws, terminalMessage{Type: "connect", Ticket: body["ticket"].(string), Cols: 80, Rows: 24}); err != nil {
			t.Fatal(err)
		}
		_ = ws.SetReadDeadline(time.Now().Add(3 * time.Second))
		var response terminalMessage
		if err = websocket.JSON.Receive(ws, &response); err != nil || response.Type != "ready" {
			t.Fatalf("connect: %v %#v", err, response)
		}
		return ws, id
	}
	awaitClosed := func() {
		t.Helper()
		runtime.mu.Lock()
		shell := runtime.shell
		runtime.mu.Unlock()
		select {
		case <-shell.closed:
		case <-time.After(5 * time.Second):
			t.Fatal("terminal shell did not close")
		}
	}
	t.Run("explicit-revoke", func(t *testing.T) {
		ws, id := open()
		defer ws.Close()
		status, _ := request(path+"/"+id+"/revoke", map[string]any{}, token, csrf, "")
		if status != 200 {
			t.Fatal(status)
		}
		awaitClosed()
		events, err := db.TerminalAudit(ctx, "app")
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 2 || events[0].Event != "revoke" || events[1].Event != "start" {
			t.Fatalf("audit: %#v", events)
		}
		config, _ := websocket.NewConfig("ws"+strings.TrimPrefix(server.URL, "http")+path+"/"+id, server.URL)
		config.Header.Set("Cookie", cookieName+"="+token)
		if reused, err := websocket.DialConfig(config); err == nil {
			reused.Close()
			t.Fatal("revoked session reconnected")
		}
	})
	t.Run("continuous-container-ownership", func(t *testing.T) {
		ws, _ := open()
		defer ws.Close()
		runtime.mu.Lock()
		runtime.ownership = false
		runtime.mu.Unlock()
		awaitClosed()
		runtime.mu.Lock()
		runtime.ownership = true
		runtime.mu.Unlock()
	})
	t.Run("logout-revokes-open-shell", func(t *testing.T) {
		ws, _ := open()
		defer ws.Close()
		if err = db.DeleteSession(ctx, auth.Hash(token)); err != nil {
			t.Fatal(err)
		}
		awaitClosed()
	})
}

func TestTerminalTicketsAreSingleUseAndOriginBound(t *testing.T) {
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	token := auth.Token()
	ctx := context.Background()
	if err = db.CreateAdmin(ctx, store.User{ID: "owner", Email: "owner@test.invalid", PasswordHash: "unused"}, auth.Hash(token), "csrf", time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	_ = db.ReplaceInventory(ctx, []store.Container{{ID: "owned", Name: "api", ApplicationID: "app", Managed: true, State: "running"}})
	runtime := &terminalRuntime{ownership: true}
	s := &Server{db: db, cfg: Config{Worker: deploy.NewWorker(db, runtime, events.New(), t.TempDir())}, terminals: map[string]*terminalSession{}}
	record := store.TerminalSession{ID: auth.Token()[:24], UserID: "owner", AuthHash: auth.Hash(token), ApplicationID: "app", ContainerID: "owned", ExpiresAt: time.Now().Add(time.Minute).Unix()}
	if err = db.CreateTerminal(ctx, record); err != nil {
		t.Fatal(err)
	}
	s.terminals[record.ID] = &terminalSession{record: record, ticket: "one-use", connectBy: time.Now().Add(time.Minute)}
	router := chi.NewRouter()
	router.Get("/applications/{id}/terminal/{terminal}", s.connectTerminal)
	server := httptest.NewServer(router)
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/applications/app/terminal/" + record.ID
	config, _ := websocket.NewConfig(wsURL, "https://evil.invalid")
	config.Header.Set("Cookie", cookieName+"="+token)
	if ws, err := websocket.DialConfig(config); err == nil {
		ws.Close()
		t.Fatal("cross-origin websocket allowed")
	}
	config, _ = websocket.NewConfig(wsURL, server.URL)
	config.Header.Set("Cookie", cookieName+"="+token)
	ws, err := websocket.DialConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	_ = websocket.JSON.Send(ws, terminalMessage{Type: "connect", Ticket: "one-use"})
	_ = ws.SetReadDeadline(time.Now().Add(3 * time.Second))
	var response terminalMessage
	if err = websocket.JSON.Receive(ws, &response); err != nil || response.Type != "ready" {
		t.Fatal(err, response)
	}
	second, err := websocket.DialConfig(config)
	if err == nil {
		defer second.Close()
		_ = websocket.JSON.Send(second, terminalMessage{Type: "connect", Ticket: "one-use"})
		_ = second.SetReadDeadline(time.Now().Add(time.Second))
		if websocket.JSON.Receive(second, &response) == nil {
			t.Fatal("ticket reused")
		}
	}
	ws.Close()
}
