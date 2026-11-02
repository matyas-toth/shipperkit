package deploy

import (
	"context"
	"errors"
	"strings"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/client"
	"shipper/internal/store"
)

// ContainerName keeps human-readable names separate from immutable ownership
// labels and network aliases. The release suffix is random and stable on retry.
func ContainerName(application, service, release string) string {
	slug := func(value string, limit int) string {
		parts := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool { return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) })
		value = strings.Join(parts, "-")
		return strings.Trim(value[:min(len(value), limit)], "-")
	}
	app := slug(application, 40)
	if app == "" {
		app = "app"
	}
	name := "shp-" + app
	if service = slug(service, 24); service != "" {
		name += "-" + service
	}
	suffix := slug(release, 12)
	if suffix == "" {
		suffix = "release"
	}
	return name + "-" + suffix
}
func runtimeName(d store.Deployment) string {
	name := d.ApplicationName
	if name == "" {
		name = d.ApplicationID
	}
	return ContainerName(name, "", d.ID)
}

// ReconcileName upgrades only the exact old generated name. Docker rename does
// not restart the workload or change its ID, volumes, or routing aliases.
func (r *DockerRuntime) ReconcileName(ctx context.Context, d store.Deployment) error {
	old := r.prefix() + "-site-" + d.ID
	info, e := r.Client.ContainerInspect(ctx, old, client.ContainerInspectOptions{})
	if errdefs.IsNotFound(e) {
		return nil
	}
	if e != nil {
		return e
	}
	if !owned(info.Container.Config.Labels, r.Instance, d.ApplicationID) || info.Container.Config.Labels["io.shipper.deployment.id"] != d.ID {
		return errors.New("Container naming ownership check failed")
	}
	_, e = r.Client.ContainerRename(ctx, info.Container.ID, client.ContainerRenameOptions{NewName: runtimeName(d)})
	return e
}
