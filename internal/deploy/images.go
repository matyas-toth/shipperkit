package deploy

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/distribution/reference"
	"github.com/moby/moby/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"shipper/internal/store"
	"strings"
)

func ImageReference(raw string) (reference.Named, error) {
	if len(raw) > 512 || strings.ContainsAny(raw, " \r\n\x00") {
		return nil, errors.New("Enter a container image reference")
	}
	ref, e := reference.ParseNormalizedNamed(raw)
	if e != nil {
		return nil, errors.New("Enter an image such as ghcr.io/owner/app:tag")
	}
	return reference.TagNameOnly(ref), nil
}
func RegistryAuth(c store.Connection) (string, error) {
	raw, e := json.Marshal(map[string]string{"username": c.Secret.Username, "password": c.Secret.Password, "serveraddress": c.Host})
	return base64.URLEncoding.EncodeToString(raw), e
}
func (r *DockerRuntime) PullImage(ctx context.Context, d *store.Deployment, c *store.Connection, log func(string)) error {
	ref, e := ImageReference(d.Config.Image)
	if e != nil {
		return e
	}
	host := reference.Domain(ref)
	auth := ""
	if c != nil {
		if c.Kind != "registry" || c.Host != host {
			return errors.New("Registry credentials do not match the image host")
		}
		auth, e = RegistryAuth(*c)
		if e != nil {
			return e
		}
	} else if host != "docker.io" && host != "ghcr.io" && host != "quay.io" {
		return errors.New("Add a registry connection before using a custom registry")
	}
	opts := client.ImagePullOptions{RegistryAuth: auth}
	if d.Config.Platform != "" {
		_, arch, _ := strings.Cut(d.Config.Platform, "/")
		opts.Platforms = []ocispec.Platform{{OS: "linux", Architecture: arch}}
	}
	log("Pulling " + ref.String())
	stream, e := r.Client.ImagePull(ctx, ref.String(), opts)
	if e != nil {
		return errors.New("Image pull failed; check the image and registry access")
	}
	e = stream.Wait(ctx)
	stream.Close()
	if e != nil {
		return errors.New("Image pull failed; check the image and registry access")
	}
	image, e := r.Client.ImageInspect(ctx, ref.String())
	if e != nil {
		return e
	}
	d.ImageID = image.ID
	d.ResolvedMethod = "image"
	if _, ok := ref.(reference.Digested); ok {
		d.Digest = ref.String()
	}
	for _, raw := range image.RepoDigests {
		candidate, e := reference.ParseNormalizedNamed(raw)
		if e == nil && candidate.Name() == ref.Name() {
			d.Digest = candidate.String()
			break
		}
	}
	return nil
}
