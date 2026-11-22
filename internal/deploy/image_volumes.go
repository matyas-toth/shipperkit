package deploy

import (
	"context"
	"errors"
	"path"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/client"
	"shipper/internal/store"
)

var anonymousVolume = regexp.MustCompile(`^[a-f0-9]{64}$`)
var volumeSlug = regexp.MustCompile(`[^a-z0-9]+`)

// ImageVolumes returns a persistent volume for every path the image declares
// with VOLUME that the application does not mount yet. Without one, Docker gives
// each release a fresh anonymous volume and the data stays behind on redeploy.
// Data the serving release already wrote to its anonymous volume is copied into
// the new volume, so the first deploy with this behaviour keeps it too.
func (r *DockerRuntime) ImageVolumes(ctx context.Context, d store.Deployment, log func(string)) ([]store.Volume, error) {
	image, e := r.Client.ImageInspect(ctx, d.ImageID)
	if e != nil {
		return nil, e
	}
	if image.Config == nil || len(image.Config.Volumes) == 0 {
		return nil, nil
	}
	declared := make([]string, 0, len(image.Config.Volumes))
	for p := range image.Config.Volumes {
		declared = append(declared, path.Clean(p))
	}
	sort.Strings(declared)
	volumes := imageVolumes(declared, d.Config.Volumes)
	for _, v := range volumes {
		created, e := r.ensureDataVolume(ctx, d.ApplicationID, r.prefix()+"-"+d.ApplicationID+"-"+v.Name)
		if e != nil {
			return nil, e
		}
		if created && d.PreviousID != "" {
			copied, e := r.adoptAnonymousVolume(ctx, d, v)
			if e != nil {
				return nil, e
			}
			if copied {
				log("Copied the data in " + v.Path + " from the current release")
			}
		}
	}
	return volumes, nil
}

// imageVolumes names a volume for each declared path that is not already on a
// configured volume, keeping within the application's volume limit.
func imageVolumes(declared []string, configured []store.Volume) []store.Volume {
	taken := map[string]bool{}
	for _, v := range configured {
		taken[v.Name] = true
	}
	out := []store.Volume{}
	for _, p := range declared {
		if !strings.HasPrefix(p, "/") || len(p) > 256 || strings.ContainsAny(p, "\\\r\n\x00:") || systemVolumePath(p) {
			continue
		}
		covered := slices.ContainsFunc(configured, func(v store.Volume) bool {
			return v.Path == p || strings.HasPrefix(p, strings.TrimSuffix(v.Path, "/")+"/")
		})
		if covered || len(configured)+len(out) >= 12 {
			continue
		}
		name := strings.Trim(volumeSlug.ReplaceAllString(strings.ToLower(p), "-"), "-")
		if len(name) > 34 {
			name = strings.Trim(name[len(name)-34:], "-")
		}
		if name == "" {
			name = "data"
		}
		base := name
		for i := 2; taken[name]; i++ {
			name = base + "-" + strconv.Itoa(i)
		}
		taken[name] = true
		out = append(out, store.Volume{Name: name, Path: p})
	}
	return out
}

func (r *DockerRuntime) ensureDataVolume(ctx context.Context, app, name string) (bool, error) {
	existing, e := r.Client.VolumeInspect(ctx, name, client.VolumeInspectOptions{})
	if e == nil {
		if !owned(existing.Volume.Labels, r.Instance, app) {
			return false, errors.New("Persistent volume ownership check failed")
		}
		return false, nil
	}
	if !errdefs.IsNotFound(e) {
		return false, e
	}
	labels := r.labels(app, "")
	labels["io.shipper.resource"] = "data"
	_, e = r.Client.VolumeCreate(ctx, client.VolumeCreateOptions{Name: name, Labels: labels})
	return e == nil, e
}

// adoptAnonymousVolume copies the serving release's anonymous volume at the
// same path into the new named volume.
func (r *DockerRuntime) adoptAnonymousVolume(ctx context.Context, d store.Deployment, v store.Volume) (bool, error) {
	items, e := r.Client.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: make(client.Filters).Add("label", "io.shipper.instance="+r.Instance).Add("label", "io.shipper.application.id="+d.ApplicationID).Add("label", "io.shipper.deployment.id="+d.PreviousID)})
	if e != nil {
		return false, e
	}
	source := ""
	for _, c := range items.Items {
		if !owned(c.Labels, r.Instance, d.ApplicationID) || c.Labels["io.shipper.infrastructure"] != "" {
			continue
		}
		for _, m := range c.Mounts {
			if m.Type == mount.TypeVolume && path.Clean(m.Destination) == v.Path && anonymousVolume.MatchString(m.Name) {
				source = m.Name
			}
		}
	}
	if source == "" {
		return false, nil
	}
	if e = r.pull(ctx, ApacheImage, func(string) {}); e != nil {
		return false, e
	}
	labels := r.labels(d.ApplicationID, "")
	labels["io.shipper.infrastructure"] = "backup-helper"
	target := r.prefix() + "-" + d.ApplicationID + "-" + v.Name
	c, e := r.Client.ContainerCreate(ctx, client.ContainerCreateOptions{Name: r.prefix() + "-adopt-" + d.ID + "-" + v.Name, Config: &container.Config{Image: ApacheImage, Entrypoint: []string{"/bin/sh", "-c"}, Cmd: []string{"sleep 1800"}, Labels: labels, NetworkDisabled: true}, HostConfig: &container.HostConfig{ReadonlyRootfs: true, CapDrop: []string{"ALL"}, CapAdd: []string{"CHOWN", "DAC_OVERRIDE", "FOWNER", "FSETID"}, SecurityOpt: []string{"no-new-privileges:true"}, Mounts: []mount.Mount{{Type: mount.TypeVolume, Source: source, Target: "/from", ReadOnly: true}, {Type: mount.TypeVolume, Source: target, Target: "/to"}}, Resources: container.Resources{Memory: 128 << 20, PidsLimit: ptr(int64(16))}, LogConfig: container.LogConfig{Type: "none"}}})
	if e != nil {
		return false, e
	}
	defer r.removeOwned(context.WithoutCancel(ctx), c.ID, d.ApplicationID)
	if _, e = r.Client.ContainerStart(ctx, c.ID, client.ContainerStartOptions{}); e != nil {
		return false, e
	}
	if _, e = r.execPipe(ctx, c.ID, []string{"cp", "-a", "/from/.", "/to/"}, nil, nil, false); e != nil {
		return false, errors.New("Copying the data in " + v.Path + " from the current release failed: " + e.Error())
	}
	return true, nil
}
