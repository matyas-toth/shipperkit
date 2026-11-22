package deploy

import (
	"reflect"
	"testing"

	"shipper/internal/store"
)

func TestHealthyStatusOnDefaultPath(t *testing.T) {
	for _, c := range []struct {
		code int
		path string
		want bool
	}{
		{200, "/", true}, {301, "/", true}, {404, "/", true}, {401, "/", true}, {500, "/", false}, {503, "", false},
		{200, "/health", true}, {302, "/health", true}, {404, "/health", false}, {401, "/health", false},
	} {
		if got := healthyStatus(c.code, c.path); got != c.want {
			t.Errorf("healthyStatus(%d, %q) = %v, want %v", c.code, c.path, got, c.want)
		}
	}
}

func TestParseListeningSkipsLoopbackAndClosedSockets(t *testing.T) {
	text := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:1F40 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 1 1 0000000000000000 100 0 0 10 0
   1: 0100007F:2405 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 2 1 0000000000000000 100 0 0 10 0
   2: 0B00A8C0:1F40 0100A8C0:C350 01 00000000:00000000 00:00000000 00000000  1000        0 3 1 0000000000000000 100 0 0 10 0
  sl  local_address                         remote_address                        st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000000000000000000000000000:1388 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 4 1 0000000000000000 100 0 0 10 0
   1: 00000000000000000000000001000000:0BB8 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 5 1 0000000000000000 100 0 0 10 0
   2: 0000000000000000FFFF00000100007F:22B8 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 6 1 0000000000000000 100 0 0 10 0
`
	if got, want := parseListening(text), []int{5000, 8000}; !reflect.DeepEqual(got, want) {
		t.Fatalf("parseListening = %v, want %v", got, want)
	}
}

func TestChoosePortPrefersCommonHTTPPorts(t *testing.T) {
	for _, c := range []struct {
		listening []int
		current   int
		want      int
	}{
		{nil, 3000, 0},
		{[]int{3000}, 3000, 0},
		{[]int{8000}, 3000, 8000},
		{[]int{8080, 9229}, 3000, 8080},
		{[]int{6123, 7001}, 3000, 6123},
	} {
		if got := choosePort(c.listening, c.current); got != c.want {
			t.Errorf("choosePort(%v, %d) = %d, want %d", c.listening, c.current, got, c.want)
		}
	}
}

func TestPortGuessed(t *testing.T) {
	d := store.Deployment{ResolvedMethod: "image", Config: store.Config{Method: "image"}}
	if !PortGuessed(d) {
		t.Fatal("a release without a port or PORT variable guesses its port")
	}
	d.Config.Port = 8000
	if PortGuessed(d) {
		t.Fatal("a configured port is not a guess")
	}
	d.Config.Port = 0
	d.Config.Variables = []store.Variable{{Name: "PORT", Value: "8000", Scope: "runtime"}}
	if PortGuessed(d) {
		t.Fatal("a PORT variable is not a guess")
	}
	if PortGuessed(store.Deployment{ResolvedMethod: "static", Config: store.Config{Method: "static"}}) {
		t.Fatal("static sites always use 8080")
	}
}

func TestImageVolumesNamesDeclaredPaths(t *testing.T) {
	configured := []store.Volume{{Name: "data", Path: "/srv/data"}, {Name: "uploads", Path: "/app/uploads"}}
	got := imageVolumes([]string{"/app/data", "/app/uploads", "/app/uploads/cache", "/data", "/etc/app", "/var/lib/postgresql/data/very/deeply/nested/directory"}, configured)
	want := []store.Volume{{Name: "app-data", Path: "/app/data"}, {Name: "data-2", Path: "/data"}, {Name: "data-very-deeply-nested-directory", Path: "/var/lib/postgresql/data/very/deeply/nested/directory"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("imageVolumes = %#v, want %#v", got, want)
	}
	for _, v := range got {
		if !volumeName.MatchString(v.Name) {
			t.Fatalf("volume name %q is not valid", v.Name)
		}
	}
}

func TestImageVolumesRespectTheVolumeLimit(t *testing.T) {
	configured := make([]store.Volume, 11)
	for i := range configured {
		configured[i] = store.Volume{Name: "v" + string(rune('a'+i)), Path: "/v/" + string(rune('a'+i))}
	}
	if got := imageVolumes([]string{"/one", "/two"}, configured); len(got) != 1 {
		t.Fatalf("expected one volume within the limit of 12, got %v", got)
	}
}

func TestPublicDomainChangeIsRoutingOnly(t *testing.T) {
	old := store.Config{Method: "image", Image: "postgres:18", Port: 5432, Database: &store.DatabaseResource{Engine: "postgresql", Version: "18"}}
	next := old
	next.Database = &store.DatabaseResource{Engine: "postgresql", Version: "18", Domain: "db.example.com"}
	plan := PlanChanges(old, next)
	if plan.Operation != "route" || plan.Downtime != "No database restart" {
		t.Fatalf("adding a public domain should only change routing: %#v", plan)
	}
	if plan := PlanChanges(next, old); plan.Operation != "route" {
		t.Fatalf("removing a public domain should only change routing: %#v", plan)
	}
	ported := next
	ported.Networking.Ports = []store.PublishedPort{{Protocol: "tcp", HostPort: 15432, ContainerPort: 5432, BindIP: "0.0.0.0"}}
	if plan := PlanChanges(old, ported); plan.Operation == "route" {
		t.Fatal("published ports still recreate the database")
	}
	cached := next
	cached.Database = &store.DatabaseResource{Engine: "postgresql", Version: "18", Domain: "db.example.com", Cache: true}
	if plan := PlanChanges(old, cached); plan.Operation == "route" {
		t.Fatal("database settings other than the domain still recreate the database")
	}
}
