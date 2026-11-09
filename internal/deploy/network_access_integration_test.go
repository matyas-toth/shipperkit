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
	consumer := func(connect bool) store.Deployment {
		a := store.Deployment{ID: auth.Token()[:24], ApplicationID: auth.Token()[:24], ApplicationName: "Network client", ImageID: image, ResolvedMethod: "image", Config: store.Config{Method: "image", Image: image, StartupPolicy: "process", StartCommand: "sleep 3600"}}
		if connect {
			a.Config.Networking.Applications = []string{b.ApplicationID}
		}
		a.ContainerID, err = r.Start(ctx, a)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	a, isolated := consumer(true), consumer(false)
	command := []string{"python", "-c", fmt.Sprintf("import socket; s=socket.create_connection(('app-%s',9003),3); print(s.recv(64).decode())", b.ApplicationID)}
	out, err := r.exec(ctx, a.ContainerID, command)
	if err != nil || !strings.Contains(string(out), "shipper-tcp") {
		t.Fatalf("private connection: %s %v", out, err)
	}
	targetInfo, inspectError := r.Client.ContainerInspect(ctx, b.ContainerID, client.ContainerInspectOptions{})
	if inspectError != nil {
		t.Fatal(inspectError)
	}
	targetIP := targetInfo.Container.NetworkSettings.Networks[r.applicationNetwork(b.ApplicationID)].IPAddress.String()
	isolationCommand := []string{"python", "-c", fmt.Sprintf("import socket; s=socket.create_connection((%q,9003),2); print(s.recv(64).decode())", targetIP)}
	if out, err = r.exec(ctx, isolated.ContainerID, isolationCommand); err == nil {
		t.Fatalf("unconnected application reached private target IP: %s", out)
	}
	dnsContext, dnsCancel := context.WithTimeout(ctx, 3*time.Second)
	if out, err = r.exec(dnsContext, isolated.ContainerID, command); err == nil {
		dnsCancel()
		t.Fatalf("unconnected application resolved private alias: %s", out)
	}
	dnsCancel()
	t.Log("Connected application resolves the stable alias; unconnected application cannot resolve or reach it")
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
	if _, err = r.Client.NetworkInspect(ctx, r.applicationNetwork(conflict.ApplicationID), client.NetworkInspectOptions{}); !errdefs.IsNotFound(err) {
		t.Fatal("conflict mutated application network", err)
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
}
