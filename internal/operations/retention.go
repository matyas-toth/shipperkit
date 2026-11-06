package operations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"sort"
	"time"

	"github.com/moby/moby/client"
	"shipper/internal/auth"
	"shipper/internal/store"
)

type RetentionImage struct {
	ID    string `json:"id"`
	Bytes int64  `json:"bytes"`
}
type RetentionPlan struct {
	LogLines    int64                  `json:"logLines"`
	LogCutoff   int64                  `json:"logCutoff"`
	LogSequence int64                  `json:"logSequence"`
	Revision    string                 `json:"revision"`
	Images      []RetentionImage       `json:"images"`
	Sources     []store.SourceSnapshot `json:"sources"`
	Protected   string                 `json:"protected"`
}

func (s *Service) AutomaticRetention(ctx context.Context) {
	enabled, e := s.DB.PrivateSetting(ctx, "automatic_retention", "false")
	if e != nil || enabled != "true" {
		return
	}
	runs, e := s.DB.MaintenanceRuns(ctx)
	if e != nil {
		return
	}
	if len(runs) > 0 && runs[0].CreatedAt > time.Now().Add(-24*time.Hour).Unix() {
		return
	}
	p, e := s.Retention(ctx)
	if e != nil || len(p.Images)+len(p.Sources) == 0 && p.LogLines == 0 {
		return
	}
	_ = s.DB.EnqueueMaintenance(ctx, store.MaintenanceRun{Actor: "policy:daily-retention", ID: auth.Token()[:24], Revision: p.Revision})
}
func (s *Service) Retention(ctx context.Context) (p RetentionPlan, e error) {
	p.LogCutoff = time.Now().UTC().Truncate(24 * time.Hour).Add(-30 * 24 * time.Hour).Unix()
	p.LogLines, p.LogSequence, e = s.DB.LogRetention(ctx, p.LogCutoff)
	if e != nil {
		return
	}
	p.Images = []RetentionImage{}
	p.Sources, e = s.DB.AbandonedSources(ctx)
	if e != nil {
		return
	}
	refs, compose, e := s.DB.ImageReferences(ctx)
	if e != nil {
		return p, e
	}
	p.Protected = "All recorded releases, containers, volumes and certificate state"
	// Compose can retain nested image references in encrypted plans. Until those
	// references can be proven exhaustively, do not remove any daemon images.
	if compose {
		p.Protected += "; image cleanup withheld while Compose release history exists"
	} else if s.Docker != nil {
		images, err := s.Docker.ImageList(ctx, client.ImageListOptions{Filters: make(client.Filters).Add("label", "io.shipper.instance="+s.Instance)})
		if err != nil {
			return p, err
		}
		containers, err := s.Docker.ContainerList(ctx, client.ContainerListOptions{All: true})
		if err != nil {
			return p, err
		}
		for _, v := range containers.Items {
			refs[v.ImageID] = true
		}
		for _, image := range images.Items {
			referenced := false
			for _, tag := range append(append([]string{}, image.RepoTags...), image.RepoDigests...) {
				referenced = referenced || refs[tag]
			}
			if referenced {
				continue
			}
			if len(p.Images) >= 100 {
				break
			}
			if image.Labels["io.shipper.instance"] != s.Instance || image.Labels["io.shipper.managed"] != "true" || image.Labels["io.shipper.application.id"] == "" || refs[image.ID] || image.Created > time.Now().Add(-24*time.Hour).Unix() {
				continue
			}
			if _, e = s.DB.Application(ctx, image.Labels["io.shipper.application.id"]); e != nil {
				continue
			}
			p.Images = append(p.Images, RetentionImage{ID: image.ID, Bytes: image.Size})
		}
	}
	sort.Slice(p.Images, func(i, j int) bool { return p.Images[i].ID < p.Images[j].ID })
	sort.Slice(p.Sources, func(i, j int) bool { return p.Sources[i].ID < p.Sources[j].ID })
	raw, _ := json.Marshal(p)
	digest := sha256.Sum256(raw)
	p.Revision = hex.EncodeToString(digest[:])
	return p, nil
}
func (s *Service) RepairRevision(ctx context.Context) (string, error) {
	apps, e := s.DB.Applications(ctx)
	if e != nil {
		return "", e
	}
	values := []string{s.Instance, s.ConfigIdentity}
	if domain, e := s.DB.Setting(ctx, "dashboard_domain", ""); e == nil && domain != "" {
		values = append(values, "dashboard:"+domain)
	}
	for _, a := range apps {
		values = append(values, a.ID+":"+s.DB.ConfigRevision(a))
		for _, d := range a.Domains {
			raw, _ := json.Marshal(d)
			values = append(values, string(raw))
		}
	}
	sort.Strings(values)
	raw, _ := json.Marshal(values)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
func (s *Service) ProcessMaintenance(ctx context.Context) bool {
	runs, e := s.DB.MaintenanceRuns(ctx)
	if e != nil {
		return false
	}
	var run *store.MaintenanceRun
	for i := len(runs) - 1; i >= 0; i-- {
		if runs[i].State == "queued" || runs[i].State == "running" {
			run = &runs[i]
			break
		}
	}
	if run == nil {
		return false
	}
	if run.Kind == "proxy" {
		run.State = "running"
		_ = s.DB.SaveMaintenance(ctx, *run)
		revision, err := s.RepairRevision(ctx)
		if err == nil && (revision != run.Revision || s.RepairProxy == nil) {
			err = errors.New("Proxy repair review changed")
		}
		if err == nil {
			work, cancel := context.WithTimeout(ctx, 2*time.Minute)
			err = s.RepairProxy(work)
			cancel()
		}
		run.State = "succeeded"
		run.FinishedAt = time.Now().Unix()
		if err != nil {
			run.State = "failed"
			run.Error = "Proxy repair could not finish; recheck system health"
		}
		_ = s.DB.SaveMaintenance(context.WithoutCancel(ctx), *run)
		s.record(ctx, store.Observation{ID: "proxy-repair", Kind: "proxy", State: func() string {
			if err != nil {
				return "failed"
			}
			return "healthy"
		}(), Summary: "Reviewed proxy repair " + run.State, Reference: run.ID})
		return true
	}
	work, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	run.State = "running"
	if e = s.DB.SaveMaintenance(work, *run); e != nil {
		return false
	}
	p, e := s.Retention(work)
	if e == nil && p.Revision != run.Revision {
		e = errors.New("Cleanup candidates changed; review again")
	}
	if e == nil {
		for _, v := range p.Images {
			if work.Err() != nil {
				e = work.Err()
				break
			}
			_, e = s.Docker.ImageRemove(work, v.ID, client.ImageRemoveOptions{Force: false, PruneChildren: false})
			if e != nil {
				break
			}
		}
	}
	if e == nil {
		for _, v := range p.Sources {
			removed, err := s.DB.RemoveAbandonedSource(work, v.ID)
			if err != nil {
				e = err
				break
			}
			if removed {
				path, err := s.DB.SourcePath(v.ID)
				if err != nil {
					e = err
					break
				}
				if err = os.Remove(path); err != nil && !os.IsNotExist(err) {
					_ = s.DB.ReleaseSourceClaim(work, v.ID)
					e = err
					break
				}
				if err = s.DB.DeleteSourceSnapshot(work, v.ID); err != nil {
					e = err
					break
				}
			} else {
				e = errors.New("Source references changed; review cleanup again")
				break
			}
		}
	}
	if e == nil {
		e = s.DB.RetainLogs(work, p.LogCutoff)
	}
	run.State = "succeeded"
	run.FinishedAt = time.Now().Unix()
	if e != nil {
		run.State = "failed"
		run.Error = "Cleanup could not finish; review current candidates before retrying"
	}
	_ = s.DB.SaveMaintenance(context.WithoutCancel(ctx), *run)
	s.record(ctx, store.Observation{ID: "cleanup", Kind: "storage", State: func() string {
		if e != nil {
			return "failed"
		}
		return "healthy"
	}(), Summary: "Reviewed cleanup " + run.State, Reference: run.ID})
	return true
}
