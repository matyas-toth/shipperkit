package operations

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf16"

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

func TestWebhookPayloadFormats(t *testing.T) {
	event := store.OperationEvent{Kind: "deployment", State: "failed", Summary: "Build failed @everyone", ApplicationID: "shop", Reference: "release-42"}
	for _, destination := range []string{"https://discord.com/api/webhooks/123/token", "https://discord.com/api/v10/webhooks/123/token?thread_id=456", "https://discordapp.com/api/webhooks/123/token", "https://canary.discord.com/api/webhooks/123/token", "https://ptb.discord.com/api/webhooks/123/token"} {
		var payload struct {
			Content         string
			AllowedMentions struct {
				Parse []string `json:"parse"`
			} `json:"allowed_mentions"`
		}
		if err := json.Unmarshal(webhookPayload(destination, event), &payload); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(payload.Content, "Build failed @everyone") || !strings.Contains(payload.Content, "Application: shop") || !strings.Contains(payload.Content, "Reference: release-42") || payload.AllowedMentions.Parse == nil || len(payload.AllowedMentions.Parse) != 0 {
			t.Fatalf("Invalid Discord payload: %+v", payload)
		}
	}
	for _, destination := range []string{"https://hooks.example.com/events", "https://discord.com.evil.test/api/webhooks/123/token", "https://discord.com/api/webhooks/123/token/github", "https://discord.com/other", "http://discord.com/api/webhooks/123/token"} {
		var payload struct {
			Version string
			Event   store.OperationEvent
		}
		if err := json.Unmarshal(webhookPayload(destination, event), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Version != "1" || payload.Event != event {
			t.Fatalf("Generic webhook changed for %s: %+v", destination, payload)
		}
	}
	event.Summary = strings.Repeat("\U0001f680", 2500)
	var payload struct{ Content string }
	if err := json.Unmarshal(webhookPayload("https://discord.com/api/webhooks/123/token", event), &payload); err != nil {
		t.Fatal(err)
	}
	if len(utf16.Encode([]rune(payload.Content))) > 2000 || !strings.HasSuffix(payload.Content, "…") {
		t.Fatal("Discord message limit exceeded")
	}
}
