package deploy

import (
	"shipper/internal/store"
	"testing"
)

func TestNetworkAccessValidation(t *testing.T) {
	app := "111111111111111111111111"
	valid := store.NetworkAccess{Applications: []string{"222222222222222222222222"}, Ports: []store.PublishedPort{{Protocol: "TCP", HostPort: 15432, ContainerPort: 5432}}}
	if err := ValidateNetworkAccess(app, &valid); err != nil {
		t.Fatal(err)
	}
	if valid.Ports[0].Protocol != "tcp" || valid.Ports[0].BindIP != "0.0.0.0" {
		t.Fatal(valid)
	}
	for _, bad := range []store.NetworkAccess{
		{Applications: []string{app}}, {Applications: []string{"host"}},
		{Applications: []string{"222222222222222222222222", "222222222222222222222222"}},
		{Ports: []store.PublishedPort{{Protocol: "sctp", HostPort: 10000, ContainerPort: 1}}},
		{Ports: []store.PublishedPort{{Protocol: "tcp", BindIP: "host", HostPort: 10000, ContainerPort: 1}}},
		{Ports: []store.PublishedPort{{Protocol: "tcp", HostPort: 80, ContainerPort: 1}}},
		{Ports: []store.PublishedPort{{Protocol: "tcp", HostPort: 10000, ContainerPort: 0}}},
		{Ports: []store.PublishedPort{{Protocol: "tcp", HostPort: 10000, ContainerPort: 1}, {Protocol: "tcp", HostPort: 10000, ContainerPort: 2}}},
	} {
		if err := ValidateNetworkAccess(app, &bad); err == nil {
			t.Fatalf("accepted unsafe networking: %+v", bad)
		}
	}
}
func TestNetworkChangeReusesImage(t *testing.T) {
	old := testConfig()
	next := old
	next.Networking = store.NetworkAccess{Applications: []string{"222222222222222222222222"}, Ports: []store.PublishedPort{{Protocol: "tcp", BindIP: "127.0.0.1", HostPort: 12345, ContainerPort: 8080}}}
	plan := PlanChanges(old, next)
	if plan.Operation != "recreate" {
		t.Fatalf("%+v", plan)
	}
}
