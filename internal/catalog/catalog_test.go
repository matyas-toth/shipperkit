package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedCatalogIsComplete(t *testing.T) {
	catalog := Embedded()
	if len(catalog.Services) < 360 {
		t.Fatalf("expected at least 360 services, got %d", len(catalog.Services))
	}
	seen := map[string]bool{}
	logos := 0
	for _, service := range catalog.Services {
		if seen[service.ID] {
			t.Fatalf("duplicate service id %q", service.ID)
		}
		seen[service.ID] = true
		if service.Name == "" || service.Description == "" || service.Compose == "" || service.MainService == "" {
			t.Fatalf("incomplete service %q", service.ID)
		}
		if service.Logo != "" {
			logos++
			path := filepath.Join("..", "..", "web", "public", strings.TrimPrefix(service.Logo, "/"))
			if _, err := os.Stat(path); err != nil {
				t.Logf("logo fallback for %s: %v", service.ID, err)
			}
		}
		if strings.Contains(service.Compose, "${$") {
			t.Fatalf("service %q contains nested interpolation", service.ID)
		}
		found := false
		for _, part := range service.Services {
			if part.Name == service.MainService {
				found = true
			}
		}
		if !found {
			t.Fatalf("service %q has no main service %q", service.ID, service.MainService)
		}
		for _, domain := range service.Domains {
			if domain.Service == "" || domain.Port < 0 {
				t.Fatalf("service %q has an invalid domain", service.ID)
			}
		}
	}
	index := catalog.Index(1, false)
	if len(index.Items) != len(catalog.Services) {
		t.Fatalf("index has %d items, want %d", len(index.Items), len(catalog.Services))
	}
	if logos < 350 {
		t.Fatalf("expected logo paths for the library, got %d", logos)
	}
	if len(index.Categories) < 10 {
		t.Fatalf("expected categories, got %d", len(index.Categories))
	}
	// Every category groups several services and reads as a label, not template data.
	for _, category := range index.Categories {
		if category.Count < 2 || strings.ContainsAny(category.Name, ",") || category.Name != strings.TrimSpace(category.Name) {
			t.Errorf("category %q (%d) should be folded into another", category.Name, category.Count)
		}
	}
	for raw, want := range map[string]string{"Database,observability,developer Tools": "Databases", "Mcp": "AI", "Devtools": "Developer tools", "Vpn": "Networking", "Mail": "Email", "": "Other", "homelab": "Homelab"} {
		if got := normalizeCategory(raw); got != want {
			t.Errorf("normalizeCategory(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestManagerRefreshAndCache(t *testing.T) {
	payload := Manifest{
		Version: "test", Source: "test", Generated: "today",
		Services: []Service{{
			ID: "example", Name: "Example", Description: "Example service", Category: "Tools",
			Logomark: "EX", Source: "shipper", MainService: "app", Port: 3000, HealthPath: "/",
			Support: "ready", Compose: "services:\n  app:\n    image: example/app:1\n",
			Services: []ServiceInfo{{Name: "app", Image: "example/app:1", Role: "Application", Port: 3000}},
			Fields:   []Field{},
		}},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write(data)
	}))
	defer server.Close()
	dir := t.TempDir()
	manager := NewManager(dir)
	if err := manager.Refresh(context.Background(), server.URL); err != nil {
		t.Fatal(err)
	}
	index := manager.Index()
	if len(index.Items) != 1 || index.Items[0].ID != "example" {
		t.Fatalf("unexpected index %#v", index)
	}
	if _, err := os.Stat(filepath.Join(dir, "service-catalog.json")); err != nil {
		t.Fatalf("cache was not written: %v", err)
	}
	reloaded := NewManager(dir)
	if got := reloaded.Index(); len(got.Items) != 1 || got.Items[0].Name != "Example" {
		t.Fatalf("cache was not loaded: %#v", got.Items)
	}
}

func TestManagerKeepsEmbeddedCatalogOnBadPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"services":[]}`))
	}))
	defer server.Close()
	manager := NewManager("")
	before := len(manager.Current().Services)
	if err := manager.Refresh(context.Background(), server.URL); err == nil {
		t.Fatal("expected invalid catalog to fail")
	}
	if got := len(manager.Current().Services); got != before {
		t.Fatalf("catalog was replaced by an invalid payload: %d -> %d", before, got)
	}
}
