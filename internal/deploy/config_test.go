package deploy

import (
	"shipper/internal/store"
	"strings"
	"testing"
)

func TestDomainPoliciesAndPublicSuffixes(t *testing.T) {
	for _, name := range []string{"example.com", "example.co.uk"} {
		if !CanIncludeWWW(name) {
			t.Fatal("root domain rejected", name)
		}
	}
	for _, name := range []string{"www.example.com", "app.example.co.uk", "test.localhost", "127.0.0.1", "co.uk"} {
		if CanIncludeWWW(name) {
			t.Fatal("non-root domain accepted", name)
		}
	}
	ds, e := NormalizeDomains([]store.Domain{{ID: "root", Hostname: "EXAMPLE.COM", Primary: true}, {ID: "www", Hostname: "www.example.com", HTTPS: true, Redirect: true}})
	if e != nil || ds[0].Hostname != "example.com" {
		t.Fatal(ds, e)
	}
	for _, ds := range [][]store.Domain{{{Hostname: "example.com"}, {Hostname: "EXAMPLE.COM"}}, {{Hostname: "test.localhost", HTTPS: true}}, {{Hostname: "example.com", Primary: true}, {Hostname: "other.com", Primary: true}}, {{Hostname: "example.com/path"}}} {
		if _, e = NormalizeDomains(ds); e == nil {
			t.Fatal("invalid domains accepted", ds)
		}
	}
}
func TestBuildValidationAndDotenv(t *testing.T) {
	c := store.Config{Method: "dockerfile", RootDirectory: "../escape"}
	if ValidateBuild(&c) == nil {
		t.Fatal("directory escape accepted")
	}
	c = store.Config{Method: "railpack", Variables: []store.Variable{{Name: "TOKEN", Value: "secret", Scope: "both", Secret: true}}, Volumes: []store.Volume{{Name: "data", Path: "/app/data"}}}
	if e := ValidateBuild(&c); e != nil {
		t.Fatal(e)
	}
	c.Volumes[0].Path = "/var/run/docker.sock"
	if ValidateBuild(&c) == nil {
		t.Fatal("socket mount accepted")
	}
	vars, e := ParseDotenv("# comment\nexport TOKEN='hello world'\nURL=https://example.com/#hash\nEXPRESSION=$(not-executed)\n")
	if e != nil || len(vars) != 3 || vars[0].Value != "hello world" || vars[2].Value != "$(not-executed)" {
		t.Fatal(vars, e)
	}
	if _, e = ParseDotenv("A=1\nA=2"); e == nil {
		t.Fatal("duplicate variable accepted")
	}
}
func TestBoundedLogsDoNotEmitSecretFragments(t *testing.T) {
	for _, runtime := range []bool{false, true} {
		var lines []string
		emit := func(s string) { lines = append(lines, s) }
		data := strings.Repeat("x", 65530) + "sensitive-token-123456789" + "\nnext\n"
		if runtime {
			w := &logWriter{emit: func(s string) error { emit(s); return nil }}
			w.Write([]byte(data[:65540]))
			w.Write([]byte(data[65540:]))
		} else {
			w := &gitLogWriter{log: emit}
			w.Write([]byte(data[:65540]))
			w.Write([]byte(data[65540:]))
			w.flush()
		}
		if len(lines) != 2 || lines[0] != "[Log line exceeded 64 KiB]" || lines[1] != "next" {
			t.Fatal("oversized line was split", lines)
		}
	}
}
