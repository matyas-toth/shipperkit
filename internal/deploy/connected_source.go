package deploy

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"shipper/internal/gitprovider"
	"shipper/internal/store"
)

type ConnectedSource struct {
	DB     *store.Store
	GitHub *gitprovider.GitHub
}

func (s ConnectedSource) Clone(ctx context.Context, c store.Config, dir string, log func(string)) (string, error) {
	if c.LocalSource != "" {
		log("Loading immutable local source")
		return s.DB.MaterializeSource(ctx, c.LocalSource, dir)
	}
	if c.Method == "inline" {
		root := filepath.Join(dir, "repository")
		if e := os.MkdirAll(root, 0700); e != nil {
			return "", e
		}
		return "", os.WriteFile(filepath.Join(root, "Dockerfile"), []byte(c.InlineDockerfile), 0600)
	}
	if c.SourceID == "" {
		return (GitSource{}).Clone(ctx, c, dir, log)
	}
	connection, e := s.DB.Connection(ctx, c.SourceID)
	if e != nil {
		return "", errors.New("The GitHub connection is unavailable")
	}
	repo, e := s.GitHub.Repository(ctx, connection, c.Repository)
	if e != nil {
		return "", e
	}
	if repo.ID != c.RepositoryID {
		return "", errors.New("Repository identity changed; choose the repository again")
	}
	token, e := s.GitHub.Token(ctx, connection, repo.ID)
	if e != nil {
		return "", e
	}
	return (GitSource{Token: token}).Clone(ctx, c, dir, log)
}
func cloneConfig(d store.Deployment) store.Config {
	c := d.Config
	if d.Commit != "" {
		c.Branch = d.Commit
	}
	return c
}
