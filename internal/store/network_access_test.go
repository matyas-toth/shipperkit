package store

import (
	"context"
	"errors"
	"testing"
)

func TestNetworkClaimsAreTransactional(t *testing.T) {
	ctx := context.Background()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := Application{ID: "a", Name: "A", Config: Config{Method: "image", Image: "alpine", Networking: NetworkAccess{Ports: []PublishedPort{{Protocol: "tcp", HostPort: 15432, ContainerPort: 5432}}}}}
	if _, err = s.Enqueue(ctx, a, Deployment{ID: "deploy-a", Operation: "deploy"}, "", true); err != nil {
		t.Fatal(err)
	}
	b := a
	b.ID = "b"
	b.Name = "B"
	if _, err = s.Enqueue(ctx, b, Deployment{ID: "deploy-b", Operation: "deploy"}, "", true); !errors.Is(err, ErrPortInUse) {
		t.Fatalf("conflict: %v", err)
	}
	if _, err = s.Application(ctx, b.ID); err == nil {
		t.Fatal("conflict persisted an application")
	}
	b.Config.Networking.Ports[0].Protocol = "udp"
	if _, err = s.Enqueue(ctx, b, Deployment{ID: "deploy-b-udp", Operation: "deploy"}, "", true); err != nil {
		t.Fatal(err)
	}
}
