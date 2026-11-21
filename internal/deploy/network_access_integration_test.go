package deploy

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"shipper/internal/auth"
	"shipper/internal/engine"
	"shipper/internal/store"
)

func TestDockerNetworkingAcceptance(t *testing.T) {
	if os.Getenv("SHIPPER_TEST_NETWORKING") != "1" {
		t.Skip("set SHIPPER_TEST_NETWORKING=1 for isolated Docker networking acceptance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	docker, err := engine.New()
	if err != nil {
		t.Fatal(err)
	}
	defer docker.Close()
	instance := auth.Token()
	r := &DockerRuntime{Client: docker.Client(), Instance: instance, DataDir: t.TempDir(), Config: ProxyConfig{BindIP: "127.0.0.1"}}
	free := func() int {
		l, e := net.Listen("tcp", "127.0.0.1:0")
		if e != nil {
			t.Fatal(e)
		}
		defer l.Close()
		return l.Addr().(*net.TCPAddr).Port
	}
	r.Config.HTTPPort = free()
	r.Config.HTTPSPort = free()
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		items, _ := r.Client.ContainerList(cleanup, client.ContainerListOptions{All: true, Filters: make(client.Filters).Add("label", "io.shipper.instance="+instance)})
		for _, c := range items.Items {
			if owned(c.Labels, instance, "") {
				r.removeOwned(cleanup, c.ID, "")
			}
		}
		networks, _ := r.Client.NetworkList(cleanup, client.NetworkListOptions{Filters: make(client.Filters).Add("label", "io.shipper.instance="+instance)})
		for _, n := range networks.Items {
			if owned(n.Labels, instance, "") {
				r.Client.NetworkRemove(cleanup, n.ID, client.NetworkRemoveOptions{})
			}
		}
		volumes, _ := r.Client.VolumeList(cleanup, client.VolumeListOptions{Filters: make(client.Filters).Add("label", "io.shipper.instance="+instance)})
		for _, v := range volumes.Items {
			if owned(v.Labels, instance, "") {
				r.Client.VolumeRemove(cleanup, v.Name, client.VolumeRemoveOptions{})
			}
		}
	})
	if err = r.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	image := "python:3.13-alpine"
	if err = r.pull(ctx, image, func(line string) { t.Log(line) }); err != nil {
		t.Fatal(err)
	}
	tcpPort, udpPort := free(), free()
	script := `import socket,threading,time
s=socket.socket();s.setsockopt(socket.SOL_SOCKET,socket.SO_REUSEADDR,1);s.bind(('0.0.0.0',9001));s.listen()
u=socket.socket(socket.AF_INET,socket.SOCK_DGRAM);u.bind(('0.0.0.0',9002))
v=socket.socket();v.bind(('0.0.0.0',9003));v.listen()
def tcp(sock):
 while True:
  c,a=sock.accept();c.sendall(b'shipper-tcp');c.close()
def udp():
 while True:
  b,a=u.recvfrom(4096);u.sendto(b'shipper-udp',a)
threading.Thread(target=tcp,args=(s,),daemon=True).start();threading.Thread(target=tcp,args=(v,),daemon=True).start();threading.Thread(target=udp,daemon=True).start()
time.sleep(3600)`
	b := store.Deployment{ID: auth.Token()[:24], ApplicationID: auth.Token()[:24], ApplicationName: "Network target", ImageID: image, ResolvedMethod: "image", Config: store.Config{Method: "image", Image: image, StartupPolicy: "process", StartCommand: `python -u -c "$SCRIPT"`, Variables: []store.Variable{{Name: "SCRIPT", Value: script, Scope: "runtime"}}, Networking: store.NetworkAccess{Ports: []store.PublishedPort{{Protocol: "tcp", BindIP: "127.0.0.1", HostPort: tcpPort, ContainerPort: 9001}, {Protocol: "udp", BindIP: "127.0.0.1", HostPort: udpPort, ContainerPort: 9002}}}}}
	b.ContainerID, err = r.Start(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.Healthy(ctx, b); err != nil {
		t.Fatal(err)
	}
	if err = r.reconcileApplicationAliases(ctx, map[string]store.Deployment{b.ApplicationID: b}); err != nil {
		t.Fatal(err)
	}
	c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", tcpPort), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 64)
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := c.Read(data)
	c.Close()
	if err != nil || string(data[:n]) != "shipper-tcp" {
		t.Fatalf("TCP %q %v", data[:n], err)
	}
	u, err := net.DialTimeout("udp", fmt.Sprintf("127.0.0.1:%d", udpPort), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	u.SetDeadline(time.Now().Add(5 * time.Second))
	u.Write([]byte("ping"))
	n, err = u.Read(data)
	u.Close()
	if err != nil || string(data[:n]) != "shipper-udp" {
		t.Fatalf("UDP %q %v", data[:n], err)
	}
	t.Log("External TCP and UDP clients reached reviewed published ports")
	// Every application joins one private network, so any application reaches
	// another by its stable name with nothing to connect first.
	a := store.Deployment{ID: auth.Token()[:24], ApplicationID: auth.Token()[:24], ApplicationName: "Network client", ImageID: image, ResolvedMethod: "image", Config: store.Config{Method: "image", Image: image, StartupPolicy: "process", StartCommand: "sleep 3600"}}
	a.ContainerID, err = r.Start(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	command := []string{"python", "-c", fmt.Sprintf("import socket; s=socket.create_connection(('app-%s',9003),3); print(s.recv(64).decode())", b.ApplicationID)}
	out, err := r.exec(ctx, a.ContainerID, command)
	if err != nil || !strings.Contains(string(out), "shipper-tcp") {
		t.Fatalf("private connection: %s %v", out, err)
	}
	if err = r.reconcileApplicationAliases(ctx, map[string]store.Deployment{a.ApplicationID: a, b.ApplicationID: b}); err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"app-" + a.ApplicationID, "db-" + a.ApplicationID} {
		if out, err = r.exec(ctx, b.ContainerID, []string{"python", "-c", fmt.Sprintf("import socket; print(socket.gethostbyname(%q))", host)}); err != nil {
			t.Fatalf("application did not resolve %s: %s %v", host, out, err)
		}
	}
	t.Log("Applications reach each other by stable name on the shared private network")
	// Earlier versions gave each application its own network. Routing moves
	// releases onto the private network, then retires the old one.
	legacy := r.applicationNetwork(b.ApplicationID)
	labels := r.labels(b.ApplicationID, "")
	labels["io.shipper.resource"] = "network"
	if _, err = r.Client.NetworkCreate(ctx, legacy, client.NetworkCreateOptions{Driver: "bridge", Labels: labels}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{a.ContainerID, b.ContainerID, r.proxy()} {
		if _, err = r.Client.NetworkConnect(ctx, legacy, client.NetworkConnectOptions{Container: id}); err != nil {
			t.Fatal(err)
		}
	}
	if err = r.retireApplicationNetworks(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = r.Client.NetworkInspect(ctx, legacy, client.NetworkInspectOptions{}); !errdefs.IsNotFound(err) {
		t.Fatal("per-application network survived", err)
	}
	if out, err = r.exec(ctx, a.ContainerID, command); err != nil || !strings.Contains(string(out), "shipper-tcp") {
		t.Fatalf("private connection after retiring the old network: %s %v", out, err)
	}
	t.Log("Per-application networks of earlier versions are retired without losing a connection")
	// Port claims include unmanaged and stopped Docker containers.
	unmanagedPort := free()
	foreignInstance := auth.Token()
	foreign, err := r.Client.ContainerCreate(ctx, client.ContainerCreateOptions{Config: &container.Config{Image: image, Cmd: []string{"sleep", "3600"}, Labels: map[string]string{"io.shipper.instance": foreignInstance}}, HostConfig: &container.HostConfig{PortBindings: publishedBindings([]store.PublishedPort{{Protocol: "tcp", BindIP: "127.0.0.1", HostPort: unmanagedPort, ContainerPort: 9001}})}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		info, e := r.Client.ContainerInspect(context.Background(), foreign.ID, client.ContainerInspectOptions{})
		if e == nil && info.Container.Config.Labels["io.shipper.instance"] == foreignInstance {
			r.Client.ContainerRemove(context.Background(), foreign.ID, client.ContainerRemoveOptions{Force: true})
		}
	})
	conflict := b
	conflict.ID = auth.Token()[:24]
	conflict.ApplicationID = auth.Token()[:24]
	conflict.ContainerID = ""
	conflict.Config.Networking = store.NetworkAccess{Ports: []store.PublishedPort{{Protocol: "tcp", BindIP: "127.0.0.1", HostPort: unmanagedPort, ContainerPort: 9001}}}
	if _, err = r.Start(ctx, conflict); err == nil {
		t.Fatal("claimed stopped unmanaged container port")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	conflict.Config.Networking.Ports[0].HostPort = listener.Addr().(*net.TCPAddr).Port
	if _, err = r.Start(ctx, conflict); err == nil {
		t.Fatal("claimed OS listener")
	}
	udpListener, udpError := net.ListenPacket("udp", "127.0.0.1:0")
	if udpError != nil {
		t.Fatal(udpError)
	}
	defer udpListener.Close()
	conflict.Config.Networking.Ports[0].Protocol = "udp"
	conflict.Config.Networking.Ports[0].HostPort = udpListener.LocalAddr().(*net.UDPAddr).Port
	if _, err = r.Start(ctx, conflict); err == nil {
		t.Fatal("claimed UDP OS listener")
	}
	conflict.Config.Networking.Ports[0].Protocol = "tcp"
	r.Config.ReservedPorts = []int{free()}
	conflict.Config.Networking.Ports[0].HostPort = r.Config.ReservedPorts[0]
	if _, err = r.Start(ctx, conflict); err == nil {
		t.Fatal("claimed custom dashboard port")
	}
	t.Log("Stopped unmanaged container, OS listener, and reserved dashboard port rejected before runtime mutation")
	// A failed fixed-port cutover restores the previous running release.
	replacement := b
	replacement.ID = auth.Token()[:24]
	replacement.PreviousID = b.ID
	replacement.ContainerID = ""
	replacement.ImageID = "shipper-network-test-image-that-does-not-exist:missing"
	if err = r.PrepareNetworkCutover(ctx, replacement); err != nil {
		t.Fatal(err)
	}
	if _, err = r.Start(ctx, replacement); err == nil {
		t.Fatal("invalid image unexpectedly started")
	}
	if err = r.RestoreNetworkCutover(ctx, replacement); err != nil {
		t.Fatal(err)
	}
	old, err := r.Client.ContainerInspect(ctx, b.ContainerID, client.ContainerInspectOptions{})
	if err != nil || !old.Container.State.Running {
		t.Fatal("previous release was not restored", err)
	}
	t.Log("Failed fixed-port replacement restored the previous release")
	databaseDomainAcceptance(ctx, t, r, free)
	if err = r.RemoveApplication(ctx, b.ApplicationID); err != nil {
		t.Fatal(err)
	}
	if out, err = r.exec(ctx, a.ContainerID, command); err == nil {
		t.Fatalf("deleted application still answered: %s", out)
	}
}

// databaseDomainAcceptance serves a managed PostgreSQL and Redis on public
// domains through the proxy. The proxy terminates TLS, PostgreSQL's in-protocol
// upgrade included, and routes by SNI, while the databases themselves only
// see plain connections on the private network.
func databaseDomainAcceptance(ctx context.Context, t *testing.T, r *DockerRuntime, free func() int) {
	t.Helper()
	ports := map[string]int{}
	for engine, port := range store.DatabaseDomainPorts {
		ports[engine] = port
	}
	t.Cleanup(func() {
		for engine, port := range ports {
			store.DatabaseDomainPorts[engine] = port
		}
	})
	apps := []store.Application{}
	active := map[string]store.Deployment{}
	hosts := map[string]string{"postgresql": "orders.shipper.test", "redis": "cache.shipper.test"}
	for _, engine := range []string{"postgresql", "redis"} {
		// The engines' standard ports may be busy on a developer machine.
		store.DatabaseDomainPorts[engine] = free()
		version := map[string]string{"postgresql": "18", "redis": "8"}[engine]
		c, err := DatabaseResourceConfig(engine, version)
		if err != nil {
			t.Fatal(err)
		}
		c.Database.Domain = hosts[engine]
		if err = ValidateBuild(&c); err != nil {
			t.Fatal(err)
		}
		if err = r.pull(ctx, c.Image, func(line string) { t.Log(line) }); err != nil {
			t.Fatal(err)
		}
		d := store.Deployment{ID: auth.Token()[:24], ApplicationID: auth.Token()[:24], ApplicationName: engine, Config: c, ImageID: c.Image, ResolvedMethod: "image", RuntimePort: c.Port}
		if d.ContainerID, err = r.Start(ctx, d); err != nil {
			t.Fatal(err)
		}
		if err = r.Healthy(ctx, d); err != nil {
			t.Fatal(engine, err)
		}
		apps = append(apps, store.Application{ID: d.ApplicationID, Name: engine})
		active[d.ApplicationID] = d
	}
	if err := r.Routes(ctx, apps, active); err != nil {
		t.Fatal(err)
	}
	proxy, err := r.Client.ContainerInspect(ctx, r.proxy(), client.ContainerInspectOptions{})
	if err != nil || proxy.Container.Config.Labels[proxyDatabasesLabel] != "postgresql,redis" {
		t.Fatal("proxy did not open the database entrypoints", err)
	}
	address := ""
	if endpoint := proxy.Container.NetworkSettings.Networks[r.privateNetwork()]; endpoint != nil {
		address = endpoint.IPAddress.String()
	}
	if address == "" {
		t.Fatal("proxy is not on the private network")
	}
	for _, d := range active {
		engine := d.Config.Database.Engine
		consumer := store.Deployment{ID: auth.Token()[:24], ApplicationID: auth.Token()[:24], ImageID: d.ImageID, ResolvedMethod: "image", Config: store.Config{Method: "image", Image: d.ImageID, StartupPolicy: "process", StartCommand: "sleep 900"}}
		if consumer.ContainerID, err = r.Start(ctx, consumer); err != nil {
			t.Fatal(err)
		}
		// Stand in for public DNS: the domain resolves to the proxy.
		if _, err = r.execInput(ctx, consumer.ContainerID, []string{"sh", "-c", "cat >> /etc/hosts"}, nil, strings.NewReader(address+" "+hosts[engine]+"\n")); err != nil {
			t.Fatal(err)
		}
		uri, err := DatabaseConnectionURI(d, DatabaseAccess{Public: true})
		if err != nil {
			t.Fatal(err)
		}
		var command []string
		want := "1"
		if engine == "postgresql" {
			command = []string{"psql", uri, "-qAtc", "SELECT 1"}
		} else {
			// The test proxy has no ACME account, so it serves its default
			// certificate. Unlike Redis drivers, redis-cli sends SNI only on request.
			command = []string{"redis-cli", "-u", uri, "--sni", hosts[engine], "--insecure", "--no-auth-warning", "--raw", "PING"}
			want = "PONG"
		}
		var out []byte
		for deadline := time.Now().Add(30 * time.Second); ; time.Sleep(time.Second) {
			if out, err = r.exec(ctx, consumer.ContainerID, command); err == nil && strings.TrimSpace(string(out)) == want {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s through its public domain: %s %v", engine, out, err)
			}
		}
		if engine == "postgresql" {
			// Without TLS there is no SNI, so the proxy cannot route the connection.
			plain := strings.Replace(uri, "sslmode=require", "sslmode=disable", 1)
			if out, err = r.exec(ctx, consumer.ContainerID, []string{"psql", plain, "-qAtc", "SELECT 1"}); err == nil {
				t.Fatalf("plain connection through the public domain: %s", out)
			}
		}
	}
	t.Log("PostgreSQL and Redis answer on their public domains through the proxy")
	for id, d := range active {
		d.Config.Database.Domain = ""
		active[id] = d
	}
	if err = r.Routes(ctx, apps, active); err != nil {
		t.Fatal(err)
	}
	if proxy, err = r.Client.ContainerInspect(ctx, r.proxy(), client.ContainerInspectOptions{}); err != nil || proxy.Container.Config.Labels[proxyDatabasesLabel] != "" {
		t.Fatal("proxy kept database entrypoints without domains", err)
	}
	t.Log("Removing the last domain closes the database entrypoints")
	// A port the proxy cannot take fails the routing change, restores the
	// previous proxy and is not retried right away by later routing.
	busy, err := net.Listen("tcp", r.Config.BindIP+":0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	store.DatabaseDomainPorts["postgresql"] = busy.Addr().(*net.TCPAddr).Port
	for id, d := range active {
		if d.Config.Database.Engine == "postgresql" {
			d.Config.Database.Domain = hosts["postgresql"]
			active[id] = d
		}
	}
	if err = r.Routes(ctx, apps, active); err == nil {
		t.Fatal("proxy claimed a port another process holds")
	}
	proxy, err = r.Client.ContainerInspect(ctx, r.proxy(), client.ContainerInspectOptions{})
	if err != nil || !proxy.Container.State.Running || proxy.Container.Config.Labels[proxyDatabasesLabel] != "" || !strings.HasPrefix(proxy.Container.Config.Labels[proxyDatabasesFailedLabel], "postgresql@") {
		t.Fatal("previous proxy was not restored", err)
	}
	if err = r.Routes(ctx, apps, active); err != nil {
		t.Fatal("later routing retried the failed entrypoints", err)
	}
	if again, err := r.Client.ContainerInspect(ctx, r.proxy(), client.ContainerInspectOptions{}); err != nil || again.Container.ID != proxy.Container.ID {
		t.Fatal("proxy restarted again for the same failed entrypoints", err)
	}
	t.Log("A busy database port fails the change once and keeps the proxy serving")
}
