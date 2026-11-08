package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"golang.org/x/net/websocket"
	"shipper/internal/auth"
	"shipper/internal/deploy"
	"shipper/internal/engine"
	"shipper/internal/events"
	"shipper/internal/store"
)

func TestDockerTerminalAuthenticatedRoundtripAndRevoke(t *testing.T) {
	if os.Getenv("SHIPPER_TEST_TERMINAL") != "1" {
		t.Skip("set SHIPPER_TEST_TERMINAL=1 for authenticated Docker websocket acceptance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	engine, err := engine.New()
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	instance, app := auth.Token(), auth.Token()[:24]
	labels := map[string]string{"io.shipper.instance": instance, "io.shipper.application.id": app, "io.shipper.managed": "true"}
	pull, err := engine.Client().ImagePull(ctx, "alpine:3.22", client.ImagePullOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err = pull.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	pull.Close()
	created, err := engine.Client().ContainerCreate(ctx, client.ContainerCreateOptions{Config: &container.Config{Image: "alpine:3.22", Cmd: []string{"sleep", "120"}, Labels: labels}, HostConfig: &container.HostConfig{NetworkMode: "none"}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		clean, stop := context.WithTimeout(context.Background(), 15*time.Second)
		defer stop()
		info, err := engine.Client().ContainerInspect(clean, created.ID, client.ContainerInspectOptions{})
		if err == nil && info.Container.Config.Labels["io.shipper.instance"] == instance {
			if _, err = engine.Client().ContainerRemove(clean, created.ID, client.ContainerRemoveOptions{Force: true}); err != nil {
				t.Error(err)
			}
		}
	}()
	if _, err = engine.Client().ContainerStart(ctx, created.ID, client.ContainerStartOptions{}); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	token, csrf := auth.Token(), auth.Token()
	if err = db.CreateAdmin(ctx, store.User{ID: "owner", Email: "owner@test.invalid", PasswordHash: "unused"}, auth.Hash(token), csrf, time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	if err = db.ReplaceInventory(ctx, []store.Container{{ID: created.ID, Name: "terminal-fixture", ApplicationID: app, Managed: true, State: "running"}}); err != nil {
		t.Fatal(err)
	}
	runtime := &deploy.DockerRuntime{Client: engine.Client(), Instance: instance}
	worker := deploy.NewWorker(db, runtime, events.New(), t.TempDir())
	server := httptest.NewServer(New(db, nil, events.New(), Config{Worker: worker, Assets: http.NotFoundHandler()}))
	defer server.Close()
	post := func(path string, input any) map[string]any {
		t.Helper()
		data, _ := json.Marshal(input)
		req, _ := http.NewRequest("POST", server.URL+path, bytes.NewReader(data))
		req.AddCookie(&http.Cookie{Name: cookieName, Value: token})
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", csrf)
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(response.Body).Decode(&out)
		if response.StatusCode >= 300 {
			t.Fatal(response.StatusCode, out)
		}
		return out
	}
	path := "/api/applications/" + app + "/terminal"
	ticket := post(path, map[string]any{"containerId": created.ID, "minutes": 1})
	id := ticket["id"].(string)
	config, _ := websocket.NewConfig("ws"+strings.TrimPrefix(server.URL, "http")+path+"/"+id, server.URL)
	config.Header.Set("Cookie", cookieName+"="+token)
	ws, err := websocket.DialConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	if err = websocket.JSON.Send(ws, terminalMessage{Type: "connect", Ticket: ticket["ticket"].(string), Cols: 90, Rows: 24}); err != nil {
		t.Fatal(err)
	}
	_ = ws.SetReadDeadline(time.Now().Add(10 * time.Second))
	var message terminalMessage
	if err = websocket.JSON.Receive(ws, &message); err != nil || message.Type != "ready" {
		t.Fatal(err, message)
	}
	if err = websocket.JSON.Send(ws, terminalMessage{Type: "input", Data: "export CHECK_VALUE=authenticated; cd /tmp\n"}); err != nil {
		t.Fatal(err)
	}
	if err = websocket.JSON.Send(ws, terminalMessage{Type: "input", Data: "printf 'ROUNDTRIP:%s:%s\\n' \"$CHECK_VALUE\" \"$PWD\"\n"}); err != nil {
		t.Fatal(err)
	}
	output := ""
	for !strings.Contains(output, "ROUNDTRIP:authenticated:/tmp") {
		if err = websocket.JSON.Receive(ws, &message); err != nil {
			t.Fatal(err, output)
		}
		if message.Type == "output" {
			data, err := base64.StdEncoding.DecodeString(message.Data)
			if err != nil {
				t.Fatal(err)
			}
			output += string(data)
		}
	}
	inspected, err := engine.Client().ContainerInspect(ctx, created.ID, client.ContainerInspectOptions{})
	if err != nil || len(inspected.Container.ExecIDs) != 1 {
		t.Fatal("expected one terminal exec", err)
	}
	execID := inspected.Container.ExecIDs[0]
	post(path+"/"+id+"/revoke", map[string]any{})
	deadline := time.Now().Add(10 * time.Second)
	for {
		info, err := engine.Client().ExecInspect(ctx, execID, client.ExecInspectOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if !info.Running {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("HTTP revocation left shell running")
		}
		time.Sleep(25 * time.Millisecond)
	}
	audit, err := db.TerminalAudit(ctx, app)
	if err != nil || len(audit) != 2 || audit[0].Event != "revoke" {
		t.Fatal("missing start/revoke audit", audit, err)
	}
	t.Log("Authenticated API ticket and websocket retained shell state; HTTP revoke ended the real Docker shell and persisted metadata audit")
}
