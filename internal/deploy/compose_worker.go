package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"shipper/internal/store"
)

type ComposeRuntime interface {
	PrepareCompose(context.Context, *store.Deployment, string, string, func(string)) error
	BuildCompose(context.Context, *store.Deployment, func(string)) error
	UpCompose(context.Context, store.Deployment, func(string)) error
}

func (w *Worker) deployCompose(ctx context.Context, d *store.Deployment) (result error) {
	runtime, ok := w.Runtime.(ComposeRuntime)
	if !ok {
		return errors.New("Docker Compose is unavailable")
	}
	log := func(line string) { w.log(*d, line) }
	// A build failure never enters the apply path and never touches the old stack.
	if d.ImageID != "compose" {
		if e := w.stage(ctx, d, "cloning"); e != nil {
			return e
		}
		root := filepath.Join(w.DataDir, "work")
		if e := os.MkdirAll(root, 0700); e != nil {
			return e
		}
		dir, e := os.MkdirTemp(root, "deployment-")
		if e != nil {
			return e
		}
		defer os.RemoveAll(dir)
		repository := filepath.Join(dir, "repository")
		if d.Config.ComposeSource == "paste" {
			if e = os.MkdirAll(repository, 0700); e != nil {
				return e
			}
		} else {
			log("Cloning " + d.Config.Repository)
			commit, e := w.Source.Clone(ctx, d.Config, dir, log)
			if e != nil {
				return e
			}
			if d.Commit != "" && d.Commit != commit {
				return errors.New("The branch changed during recovery; redeploy to use the new revision")
			}
			d.Commit = commit
		}
		appRoot, e := SourceRoot(repository, d.Config)
		if e != nil {
			return e
		}
		if e = runtime.PrepareCompose(ctx, d, appRoot, repository, log); e != nil {
			return e
		}
		app, e := w.DB.Application(ctx, d.ApplicationID)
		if e != nil {
			return e
		}
		var plan composePlan
		if e = json.Unmarshal([]byte(d.ComposePlan), &plan); e != nil {
			return e
		}
		plan.PreviousDesiredState = app.DesiredState
		raw, e := json.Marshal(plan)
		if e != nil {
			return e
		}
		d.ComposePlan = string(raw)
		for _, domain := range app.Domains {
			found := false
			for _, svc := range d.Services {
				if svc.Name == domain.Service && svc.Port > 0 {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("Domain %s needs a Compose service with an HTTP port", domain.Hostname)
			}
		}
		if e = w.stage(ctx, d, "building"); e != nil {
			return e
		}
		log("Preparing Compose images before updating services")
		if e = runtime.BuildCompose(ctx, d, log); e != nil {
			return e
		}
		d.ImageID = "compose"
	}
	if e := w.stage(ctx, d, "starting"); e != nil {
		return e
	}
	// Compose updates are stateful. Restore the previous pinned model on an apply
	// failure, but never claim that restoring containers reverses a DB migration.
	defer func() {
		if result == nil || ctx.Err() == context.Canceled {
			return
		}
		rollback, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Minute)
		defer cancel()
		if d.PreviousID == "" {
			w.DB.SetActive(rollback, d.ApplicationID, "")
			w.routes(rollback, d.ApplicationID, nil)
			if e := w.Runtime.RemoveApplication(rollback, d.ApplicationID); e != nil {
				result = fmt.Errorf("%w; candidate cleanup failed: %v", result, e)
			}
			return
		}
		previous, e := w.DB.Deployment(rollback, d.PreviousID)
		if e != nil {
			result = fmt.Errorf("%w; previous release could not be loaded", result)
			return
		}
		if ReleaseMethod(previous) != "compose" {
			return
		}
		w.stage(rollback, d, "rolling_back")
		log("Restoring the previous Compose services; persistent volumes are retained")
		if e = runtime.UpCompose(rollback, previous, log); e == nil {
			e = w.Runtime.Healthy(rollback, previous)
		}
		if e == nil {
			e = w.DB.SetActive(rollback, d.ApplicationID, previous.ID)
		}
		var plan composePlan
		json.Unmarshal([]byte(d.ComposePlan), &plan)
		desired := "running"
		if plan.PreviousDesiredState == "stopped" {
			desired = "stopped"
		}
		if e == nil && desired == "stopped" {
			operator, ok := w.Runtime.(interface {
				Lifecycle(context.Context, store.Deployment, string) error
			})
			if !ok {
				e = errors.New("Restoring the stopped application is unavailable")
			} else {
				e = operator.Lifecycle(rollback, previous, "stop")
			}
		}
		if e == nil {
			e = w.DB.SetDesiredState(rollback, d.ApplicationID, desired)
		}
		if e == nil {
			if desired == "stopped" {
				e = w.routes(rollback, d.ApplicationID, nil)
			} else {
				e = w.routes(rollback, "", &previous)
			}
		}
		if e != nil {
			w.DB.SetDesiredState(rollback, d.ApplicationID, "stopped")
			w.routes(rollback, d.ApplicationID, nil)
			result = fmt.Errorf("%w; restoring the previous services failed: %v", result, e)
		}
	}()
	if e := w.routes(ctx, d.ApplicationID, nil); e != nil {
		return e
	}
	log("Updating Compose services")
	if e := runtime.UpCompose(ctx, *d, log); e != nil {
		return e
	}
	if e := w.stage(ctx, d, "checking"); e != nil {
		return e
	}
	if e := w.Runtime.Healthy(ctx, *d); e != nil {
		return e
	}
	if e := w.stage(ctx, d, "promoting"); e != nil {
		return e
	}
	if e := w.DB.SetActive(ctx, d.ApplicationID, d.ID); e != nil {
		return e
	}
	if e := w.DB.SetDesiredState(ctx, d.ApplicationID, "running"); e != nil {
		return e
	}
	if e := w.routes(ctx, "", d); e != nil {
		return e
	}
	log("Compose services are running; persistent volumes were retained")
	return nil
}
