// Package engine adapts Docker's official Go SDK to read-only inventory operations.
package engine

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/moby/moby/client"
	"shipper/internal/store"
)

type Info struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	CPUs         int    `json:"cpus"`
	Memory       int64  `json:"memory"`
}
type Snapshot struct {
	Containers []store.Container
	Info       Info
}
type Source interface {
	Snapshot(context.Context) (Snapshot, error)
	Subscribe(context.Context) (<-chan struct{}, <-chan error)
}
type Docker struct{ client *client.Client }

func New() (*Docker, error) {
	host := os.Getenv("DOCKER_HOST")
	if host != "" && !strings.HasPrefix(host, "unix://") && !strings.HasPrefix(host, "npipe://") {
		if !strings.HasPrefix(host, "tcp://") || os.Getenv("DOCKER_TLS_VERIFY") != "1" {
			return nil, errors.New("remote Docker requires tcp:// with DOCKER_TLS_VERIFY=1 and valid certificates")
		}
	}
	cli, err := client.New(client.FromEnv, client.WithUserAgent("shipper/0.1.0"))
	if err != nil {
		return nil, err
	}
	return &Docker{client: cli}, nil
}

// NewWithClient allows tests to exercise the real SDK against an isolated HTTP fixture.
func NewWithClient(cli *client.Client) *Docker { return &Docker{client: cli} }
func (d *Docker) Close() error                 { return d.client.Close() }

func (d *Docker) Snapshot(ctx context.Context) (Snapshot, error) {
	list, err := d.client.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return Snapshot{}, err
	}
	info, err := d.client.Info(ctx, client.InfoOptions{})
	if err != nil {
		return Snapshot{}, err
	}
	snapshot := Snapshot{Containers: []store.Container{}, Info: Info{Name: info.Info.Name, Version: info.Info.ServerVersion, OS: info.Info.OperatingSystem, Architecture: info.Info.Architecture, CPUs: info.Info.NCPU, Memory: info.Info.MemTotal}}
	for _, item := range list.Items {
		if item.Labels["io.shipper.control-plane"] == "true" {
			continue
		}
		name := item.ID[:min(12, len(item.ID))]
		if len(item.Names) > 0 {
			name = strings.TrimPrefix(item.Names[0], "/")
		}
		health := ""
		if item.Health != nil {
			health = string(item.Health.Status)
		}
		if health == "" {
			for _, candidate := range []string{"unhealthy", "healthy", "starting"} {
				if strings.Contains(item.Status, "("+candidate+")") || strings.Contains(item.Status, "(health: "+candidate+")") {
					health = candidate
					break
				}
			}
		}
		appID := item.Labels["io.shipper.application.id"]
		managed := item.Labels["io.shipper.managed"] == "true" && appID != "" && len(appID) <= 128
		if !managed {
			appID = ""
		}
		c := store.Container{ID: item.ID, Name: name, Image: item.Image, State: string(item.State), Status: item.Status, Health: health, CreatedAt: item.Created, Managed: managed, ApplicationID: appID, Ports: []store.Port{}, Project: item.Labels["com.docker.compose.project"], Service: item.Labels["com.docker.compose.service"]}
		for _, p := range item.Ports {
			ip := ""
			if p.IP.IsValid() {
				ip = p.IP.String()
			}
			c.Ports = append(c.Ports, store.Port{IP: ip, Private: p.PrivatePort, Public: p.PublicPort, Protocol: p.Type})
		}
		snapshot.Containers = append(snapshot.Containers, c)
	}
	return snapshot, nil
}

func (d *Docker) Subscribe(ctx context.Context) (<-chan struct{}, <-chan error) {
	changes := make(chan struct{}, 1)
	errs := make(chan error, 1)
	stream := d.client.Events(ctx, client.EventsListOptions{Filters: make(client.Filters).Add("type", "container")})
	go func() {
		defer close(changes)
		defer close(errs)
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-stream.Messages:
				if !ok {
					return
				}
				select {
				case changes <- struct{}{}:
				default:
				}
			case err, ok := <-stream.Err:
				if ok && err != nil {
					errs <- err
				}
				return
			}
		}
	}()
	return changes, errs
}
