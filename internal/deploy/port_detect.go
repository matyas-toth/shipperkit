package deploy

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"shipper/internal/store"
)

// PortDetected reports that the candidate never answered on the port Shipper
// guessed but listens on another one. The worker adopts the port and checks again.
type PortDetected struct{ From, To int }

func (p *PortDetected) Error() string {
	return fmt.Sprintf("Nothing answered on port %d; the application listens on port %d", p.From, p.To)
}

// PortGuessed reports a release whose port nobody chose: no container port and
// no PORT variable, so it came from the image's EXPOSE or the 3000 fallback.
func PortGuessed(d store.Deployment) bool {
	method := ReleaseMethod(d)
	if d.Config.Port > 0 || method == "static" || method == "compose" || d.Config.Database != nil || d.TaskCommand != nil {
		return false
	}
	for _, v := range d.Config.Variables {
		if v.Name == "PORT" && (v.Scope == "runtime" || v.Scope == "both" || v.Scope == "") {
			return false
		}
	}
	return true
}

// portScanner reads the TCP sockets a candidate listens on. Many images ship
// without a shell, so a small helper joins the candidate's network namespace and
// reads /proc/net there.
type portScanner struct {
	r      *DockerRuntime
	d      store.Deployment
	helper string
}

func (s *portScanner) ports(ctx context.Context) ([]int, error) {
	if s.helper == "" {
		if e := s.r.pull(ctx, ApacheImage, func(string) {}); e != nil {
			return nil, e
		}
		labels := s.r.labels(s.d.ApplicationID, "")
		labels["io.shipper.infrastructure"] = "port-probe"
		name := s.r.prefix() + "-ports-" + s.d.ID
		s.r.removeOwned(ctx, name, s.d.ApplicationID)
		c, e := s.r.Client.ContainerCreate(ctx, client.ContainerCreateOptions{Name: name, Config: &container.Config{Image: ApacheImage, Entrypoint: []string{"/bin/sh", "-c"}, Cmd: []string{"sleep 600"}, Labels: labels}, HostConfig: &container.HostConfig{NetworkMode: container.NetworkMode("container:" + s.d.ContainerID), ReadonlyRootfs: true, CapDrop: []string{"ALL"}, SecurityOpt: []string{"no-new-privileges:true"}, Resources: container.Resources{Memory: 32 << 20, PidsLimit: ptr(int64(8))}, LogConfig: container.LogConfig{Type: "none"}}})
		if e != nil {
			return nil, e
		}
		s.helper = c.ID
		if _, e = s.r.Client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{}); e != nil {
			return nil, e
		}
	}
	out, e := s.r.execPipe(ctx, s.helper, []string{"/bin/sh", "-c", "cat /proc/net/tcp /proc/net/tcp6 2>/dev/null; true"}, nil, nil, false)
	if e != nil {
		return nil, e
	}
	return parseListening(string(out)), nil
}

func (s *portScanner) close() {
	if s.helper != "" {
		s.r.removeOwned(context.Background(), s.helper, s.d.ApplicationID)
	}
}

// parseListening returns the ports with a LISTEN socket that other containers
// can reach. Sockets bound to loopback are skipped because the proxy cannot.
func parseListening(text string) []int {
	out := []int{}
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[3] != "0A" {
			continue
		}
		address, hex, ok := strings.Cut(fields[1], ":")
		if !ok {
			continue
		}
		loopback := len(address) == 8 && strings.HasSuffix(address, "7F") ||
			address == "00000000000000000000000001000000" ||
			strings.HasPrefix(address, "0000000000000000FFFF0000") && strings.HasSuffix(address, "7F")
		port, e := strconv.ParseUint(hex, 16, 16)
		if loopback || e != nil || port == 0 || slices.Contains(out, int(port)) {
			continue
		}
		out = append(out, int(port))
	}
	slices.Sort(out)
	return out
}

// choosePort picks the port to adopt, or 0 when the guessed port is among the
// listeners and the application only needs more time.
func choosePort(listening []int, current int) int {
	if len(listening) == 0 || slices.Contains(listening, current) {
		return 0
	}
	for _, common := range []int{80, 8080, 8000, 5000, 3000, 4000, 8888, 5173, 4321, 1337, 9000} {
		if slices.Contains(listening, common) {
			return common
		}
	}
	return listening[0]
}
