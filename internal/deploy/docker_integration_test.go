package deploy

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"shipper/internal/auth"
	"shipper/internal/engine"
	"shipper/internal/events"
	"shipper/internal/store"
)

// Opt-in: creates only resources carrying this run's random instance label.
// Cleanup removes only this run's containers, images, network and proxy-data
// volume. It never prunes globally or touches application data volumes.
func TestDockerStaticPipeline(t *testing.T) {
	if os.Getenv("SHIPPER_TEST_DOCKER") != "1" {
		t.Skip("set SHIPPER_TEST_DOCKER=1 for real Docker integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	docker, e := engine.New()
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { docker.Close() })
	instance := auth.Token()
	freePort := func() int {
		l, e := net.Listen("tcp", "127.0.0.1:0")
		if e != nil {
			t.Fatal(e)
		}
		p := l.Addr().(*net.TCPAddr).Port
		l.Close()
		return p
	}
	runtime := &DockerRuntime{Client: docker.Client(), Instance: instance, Config: ProxyConfig{BindIP: "127.0.0.1", HTTPPort: freePort(), HTTPSPort: freePort()}}
	dir := t.TempDir()
	db, e := store.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	w := NewWorker(db, runtime, events.New(), dir)
	w.Source = fixtureSource{commit: strings.Repeat("a", 40)}
	app := store.Application{ID: auth.Token()[:24], Name: "Static fixture", Config: testConfig()}
	app.Config.Domain = app.ID + ".localhost"
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 30*time.Second)
		defer stop()
		if e := runtime.RemoveApplication(cleanup, app.ID); e != nil {
			t.Error(e)
		}
		if e := runtime.removeOwned(cleanup, runtime.proxy(), ""); e != nil {
			t.Error(e)
		}
		if n, e := runtime.Client.NetworkInspect(cleanup, runtime.network(), client.NetworkInspectOptions{}); e == nil && owned(n.Network.Labels, instance, "") {
			if _, e = runtime.Client.NetworkRemove(cleanup, runtime.network(), client.NetworkRemoveOptions{}); e != nil {
				t.Error(e)
			}
		}
		volume := runtime.prefix() + "-proxy-data"
		if v, e := runtime.Client.VolumeInspect(cleanup, volume, client.VolumeInspectOptions{}); e == nil && owned(v.Volume.Labels, instance, "") {
			if _, e = runtime.Client.VolumeRemove(cleanup, volume, client.VolumeRemoveOptions{}); e != nil {
				t.Error(e)
			}
		}
		images, e := runtime.Client.ImageList(cleanup, client.ImageListOptions{Filters: make(client.Filters).Add("label", "io.shipper.instance="+instance)})
		if e != nil {
			t.Error(e)
		} else {
			for _, img := range images.Items {
				inspect, e := runtime.Client.ImageInspect(cleanup, img.ID)
				if e == nil && owned(inspect.Config.Labels, instance, app.ID) {
					if _, e = runtime.Client.ImageRemove(cleanup, img.ID, client.ImageRemoveOptions{PruneChildren: false}); e != nil {
						t.Error(e)
					}
				}
			}
		}
	})
	deploySite := func(c store.StaticConfig) store.Deployment {
		t.Helper()
		current := app
		if existing, e := db.Application(ctx, app.ID); e == nil {
			current = existing
		}
		current.Config = c
		d, e := db.Enqueue(ctx, current, store.Deployment{ID: auth.Token()[:24], Operation: "deploy"}, "", current.CreatedAt == 0)
		if e != nil {
			t.Fatal(e)
		}
		w.process(ctx, d)
		got, e := db.Deployment(ctx, d.ID)
		if e != nil {
			t.Fatal(e)
		}
		if got.State != "succeeded" {
			lines, _ := db.Logs(ctx, d.ID, 0)
			t.Fatalf("deployment failed: %+v\n%+v", got, lines)
		}
		return got
	}
	request := func(p string) (int, string, string) {
		t.Helper()
		req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://127.0.0.1:%d%s", runtime.Config.HTTPPort, p), nil)
		req.Host = app.Config.Domain
		response, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer response.Body.Close()
		b, _ := io.ReadAll(response.Body)
		return response.StatusCode, string(b), response.Header.Get("X-Shipper-Deployment")
	}
	first := deploySite(app.Config)
	if ok, e := runtime.ProbeLocal(ctx, first); e != nil || !ok {
		t.Fatal("local domain verification failed", ok, e)
	}
	// A disposable, intentionally unmanaged sentinel proves deletion cannot sweep
	// other containers on the same host. It is never started or exposed.
	sentinel, e := runtime.Client.ContainerCreate(ctx, client.ContainerCreateOptions{Name: "shipper-test-sentinel-" + instance[:12], Config: &container.Config{Image: ApacheImage, Labels: map[string]string{"io.shipper.test-run": instance}}})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		info, e := runtime.Client.ContainerInspect(cleanup, sentinel.ID, client.ContainerInspectOptions{})
		if e == nil && info.Container.Config.Labels["io.shipper.test-run"] == instance {
			runtime.Client.ContainerRemove(cleanup, sentinel.ID, client.ContainerRemoveOptions{})
		}
	})
	code, body, release := request("/")
	if code != 200 || !strings.Contains(body, "Shipper static fixture") || release != first.ID {
		t.Fatal(code, body, release)
	}
	if code, _, _ := request("/about"); code != 200 {
		t.Fatal("file based routing failed", code)
	}
	for _, p := range []string{"/.git/config", "/.env", "/.htaccess"} {
		if code, _, _ := request(p); code == 200 {
			t.Fatal("sensitive file was served", p)
		}
	}
	c := app.Config
	c.Routing = "spa"
	second := deploySite(c)
	if code, _, r := request("/nested/client/route"); code != 200 || r != second.ID {
		t.Fatal("SPA fallback failed", code, r)
	}
	c.Routing = "strict"
	c.HTAccess = true
	third := deploySite(c)
	if code, _, _ := request("/missing"); code != 404 {
		t.Fatal("strict routing failed", code)
	}
	if code, body, _ := request("/hello"); code != 200 || !strings.Contains(body, "About Shipper") {
		t.Fatal(".htaccess rewrite failed", code, body)
	}
	// A packaging failure must not disturb the serving release.
	current, _ := db.Application(ctx, app.ID)
	broken := current
	broken.Config.PublishDirectory = "does-not-exist"
	failed, e := db.Enqueue(ctx, broken, store.Deployment{ID: auth.Token()[:24], Operation: "deploy"}, "", false)
	if e != nil {
		t.Fatal(e)
	}
	w.process(ctx, failed)
	failed, _ = db.Deployment(ctx, failed.ID)
	if failed.State != "failed" {
		t.Fatal("broken publish directory did not fail")
	}
	if code, _, r := request("/"); code != 200 || r != third.ID {
		t.Fatal("failed deployment changed serving site", code, r)
	}
	// An Apache configuration error must fail the candidate while the old release serves.
	w.Source = fixtureSource{broken: true, commit: strings.Repeat("b", 40)}
	current, _ = db.Application(ctx, app.ID)
	current.Config.PublishDirectory = "."
	current.Config.HTAccess = true
	unhealthy, e := db.Enqueue(ctx, current, store.Deployment{ID: auth.Token()[:24], Operation: "deploy"}, "", false)
	if e != nil {
		t.Fatal(e)
	}
	w.process(ctx, unhealthy)
	unhealthy, _ = db.Deployment(ctx, unhealthy.ID)
	if unhealthy.State != "failed" || !strings.Contains(unhealthy.Error, "health check") {
		t.Fatal("unhealthy candidate was not rejected", unhealthy)
	}
	if !strings.Contains(unhealthy.Error, "InvalidApacheDirective") {
		t.Fatal("Apache diagnostics were not persisted", unhealthy.Error)
	}
	if code, _, r := request("/"); code != 200 || r != third.ID {
		t.Fatal("unhealthy candidate replaced the serving release", code, r)
	}
	// Exercise the real, hardened Git transport as part of the full pipeline.
	w.Source = GitSource{}
	c.Repository = "https://github.com/mdn/beginner-html-site.git"
	c.Branch = ""
	c.HTAccess = false
	fromGit := deploySite(c)
	if code, _, r := request("/"); code != 200 || r != fromGit.ID {
		t.Fatal("public Git deployment failed", code, r)
	}
	if len(fromGit.Commit) != 40 {
		t.Fatal("Git revision was not recorded", fromGit.Commit)
	}
	current, _ = db.Application(ctx, app.ID)
	// Verify TLS with a generated trust anchor, without disabling certificate checks.
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: app.Config.Domain}, DNSNames: []string{app.Config.Domain}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IsCA: true, BasicConstraintsValid: true}
	der, e := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if e != nil {
		t.Fatal(e)
	}
	private, _ := x509.MarshalPKCS8PrivateKey(key)
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if e = runtime.copy(ctx, runtime.proxy(), "/data", map[string][]byte{"test.crt": cert, "test.key": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private})}); e != nil {
		t.Fatal(e)
	}
	secure := fromGit
	secure.Config.HTTPS = true
	config := Routes([]store.Application{current}, map[string]store.Deployment{app.ID: secure})
	for _, v := range config["http"].(map[string]any)["routers"].(map[string]any) {
		router := v.(map[string]any)
		if router["tls"] != nil {
			router["tls"] = map[string]any{}
		}
	}
	config["tls"] = map[string]any{"certificates": []any{map[string]string{"certFile": "/data/test.crt", "keyFile": "/data/test.key"}}}
	data, _ := json.Marshal(config)
	if e = runtime.copy(ctx, runtime.proxy(), "/data/dynamic", map[string][]byte{"routes.next": data}); e != nil {
		t.Fatal(e)
	}
	if _, e = runtime.exec(ctx, runtime.proxy(), []string{"mv", "/data/dynamic/routes.next", "/data/dynamic/routes.yaml"}); e != nil {
		t.Fatal(e)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(cert)
	transport := &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, ServerName: app.Config.Domain, MinVersion: tls.VersionTLS12}}
	defer transport.CloseIdleConnections()
	secureClient := &http.Client{Transport: transport, Timeout: 3 * time.Second}
	verified := false
	for limit := time.Now().Add(5 * time.Second); time.Now().Before(limit); {
		req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://127.0.0.1:%d/", runtime.Config.HTTPSPort), nil)
		req.Host = app.Config.Domain
		res, e := secureClient.Do(req)
		if e == nil {
			res.Body.Close()
			if res.StatusCode == 200 && res.Header.Get("X-Shipper-Deployment") == fromGit.ID {
				verified = true
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !verified {
		t.Fatal("trusted HTTPS route did not serve the active release")
	}
	if e := runtime.Routes(ctx, []store.Application{current}, map[string]store.Deployment{app.ID: fromGit}); e != nil {
		t.Fatal(e)
	}
	// Proxy routes survive a proxy restart without needing Shipper to supply them.
	if _, e = runtime.Client.ContainerRestart(ctx, runtime.proxy(), client.ContainerRestartOptions{}); e != nil {
		t.Fatal(e)
	}
	ready := false
	for limit := time.Now().Add(5 * time.Second); time.Now().Before(limit); {
		req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://127.0.0.1:%d/", runtime.Config.HTTPPort), nil)
		req.Host = app.Config.Domain
		res, e := http.DefaultClient.Do(req)
		if e == nil {
			res.Body.Close()
			if res.StatusCode == 200 && res.Header.Get("X-Shipper-Deployment") == fromGit.ID {
				ready = true
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		t.Fatal("proxy routes did not survive restart")
	}
	current, _ = db.Application(ctx, app.ID)
	deletion, e := db.Enqueue(ctx, current, store.Deployment{ID: auth.Token()[:24], Operation: "delete"}, "", false)
	if e != nil {
		t.Fatal(e)
	}
	w.process(ctx, deletion)
	done, _ := db.Deployment(ctx, deletion.ID)
	if done.State != "succeeded" {
		t.Fatal("safe deletion failed", done.Error)
	}
	if _, e = db.Application(ctx, app.ID); e == nil {
		t.Fatal("deleted application still listed")
	}
	if _, e = runtime.Client.ContainerInspect(ctx, sentinel.ID, client.ContainerInspectOptions{}); e != nil {
		t.Fatal("application deletion touched an unrelated container", e)
	}
	t.Log("Passed public Git -> Apache -> Traefik HTTP, trusted TLS, file/SPA/strict routing, .htaccess, secret exclusions, versioned promotion, failed-release preservation, proxy restart, and scoped deletion")
}
