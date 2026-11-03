package deploy

import (
	"context"
	"errors"
	"shipper/internal/auth"
	"shipper/internal/store"
)

func (w *Worker) triggered(ctx context.Context) {
	jobs, e := w.DB.DeployIntents(ctx)
	if e != nil {
		return
	}
	for _, job := range jobs {
		a, e := w.DB.Application(ctx, job.ApplicationID)
		if e != nil || !a.Config.AutoDeploy.Enabled {
			w.DB.ClearDeployIntent(ctx, job.ApplicationID, job.Revision)
			continue
		}
		if source, ok := w.Source.(ConnectedSource); ok && a.Config.SourceID != "" {
			connection, err := w.DB.Connection(ctx, a.Config.SourceID)
			if err == nil {
				head, err := source.GitHub.Head(ctx, connection, a.Config.Repository, a.Config.Branch)
				if err == nil && head != job.Revision {
					w.DB.ClearDeployIntent(ctx, a.ID, job.Revision)
					continue
				}
			}
		}
		if a.LatestDeployment != nil && a.LatestDeployment.State == "queued" && a.Config.AutoDeploy.Supersede {
			w.DB.CancelDeployment(ctx, a.ID, a.LatestDeployment.ID)
		}
		_, e = w.DB.Enqueue(ctx, a, store.Deployment{ID: auth.Token()[:24], Operation: "deploy", Commit: job.Revision, Actor: job.Actor}, "push:"+a.ID+":"+job.Revision, false)
		if e == nil || errors.Is(e, store.ErrIdempotencyConflict) {
			w.DB.ClearDeployIntent(ctx, a.ID, job.Revision)
			w.changed()
		}
	}
}
