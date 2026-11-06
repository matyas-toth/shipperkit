package deploy

import (
	"context"
	"testing"
)

func TestNormalizeDashboardDomain(t *testing.T) {
	for _, raw := range []string{"shipper.example.com", " Shipper.Example.COM "} {
		domain, e := NormalizeDashboardDomain(raw)
		if e != nil || domain != "shipper.example.com" {
			t.Fatal(raw, domain, e)
		}
	}
	for _, raw := range []string{"https://shipper.example.com", "shipper.example.com/path", "*.example.com", "localhost", "127.0.0.1"} {
		if domain, e := NormalizeDashboardDomain(raw); e == nil {
			t.Fatal("accepted", raw, domain)
		}
	}
	if domain, e := NormalizeDashboardDomain(""); e != nil || domain != "" {
		t.Fatal("empty domain should clear the setting", domain, e)
	}
}

func TestDashboardDomainOverridesProxyConfig(t *testing.T) {
	runtime := &DockerRuntime{
		Instance: "0123456789abcdef",
		Config: ProxyConfig{
			DashboardContainer: "shipper-container",
			DashboardUpstream:  "http://dashboard-0123456789ab:8080",
		},
		DashboardDomain: func(context.Context) string { return "shipper.example.com" },
	}
	config, e := runtime.EffectiveProxy(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if config.DashboardURL != "https://shipper.example.com" || config.DashboardUpstream != "http://dashboard-0123456789ab:8080" {
		t.Fatal(config)
	}
	runtime.DashboardDomain = func(context.Context) string { return "" }
	config, e = runtime.EffectiveProxy(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if config.DashboardURL != "" || config.DashboardUpstream != "" {
		t.Fatal(config)
	}
}
