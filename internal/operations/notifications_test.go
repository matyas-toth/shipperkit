package operations

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"shipper/internal/store"
)

func TestWebhookPrivateOptInAndNoRedirect(t *testing.T) {
	calls := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("X-Shipper-Event") != "deployment" {
			t.Error("Missing event identity")
		}
		w.WriteHeader(204)
	}))
	defer target.Close()
	config := NotificationConfig{Enabled: true, Kind: "webhook", Webhook: target.URL, Events: []string{"deployment"}}
	event := store.OperationEvent{Kind: "deployment", State: "failed", Summary: "Deployment failed"}
	if e := Send(context.Background(), config, event); e == nil || calls != 0 {
		t.Fatal("Private HTTP endpoint accepted without opt-in")
	}
	config.Private = true
	if e := Send(context.Background(), config, event); e != nil || calls != 1 {
		t.Fatal(calls, e)
	}
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) }))
	defer redirect.Close()
	config.Webhook = redirect.URL
	if e := Send(context.Background(), config, event); e == nil || calls != 1 {
		t.Fatal("Redirect followed", calls, e)
	}
	config.Kind = "email"
	config.SMTP = "smtp.example.test:25"
	if e := config.Validate(); e == nil {
		t.Fatal("Plain SMTP accepted")
	}
}
