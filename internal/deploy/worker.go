package deploy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"shipper/internal/auth"
	"shipper/internal/events"
	"shipper/internal/store"
)

type Worker struct {
	DB      *store.Store
	Runtime Runtime
	Source  Source
	Hub     *events.Hub
	DataDir string
	wake    chan struct{}
	Refresh func()
}

func NewWorker(db *store.Store, r Runtime, hub *events.Hub, dir string) *Worker {
	return &Worker{DB: db, Runtime: r, Source: GitSource{}, Hub: hub, DataDir: dir, wake: make(chan struct{}, 1)}
}
func (w *Worker) Trigger() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}
func (w *Worker) changed() {
	w.Hub.Publish()
	if w.Refresh != nil {
		w.Refresh()
	}
}
func (w *Worker) Run(ctx context.Context) {
	owner := auth.Token()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for ctx.Err() == nil {
		ok, e := w.DB.Lease(ctx, owner)
		if e != nil {
			slog.Error("Deployment lease failed", "error", e)
		}
		if ok {
			w.runOwned(ctx, owner)
			if ctx.Err() != nil {
				return
			}
			slog.Warn("Deployment worker is reacquiring its lease")
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
func (w *Worker) runOwned(ctx context.Context, owner string) {
	work, stop := context.WithCancel(ctx)
	defer stop()
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-work.Done():
				return
			case <-t.C:
				ok, e := w.DB.Lease(work, owner)
				if e != nil || !ok {
					stop()
					return
				}
			}
		}
	}()
	defer func() { stop(); <-heartbeatDone; w.DB.ReleaseLease(context.Background(), owner) }()
	if e := w.cleanWorkspaces(); e != nil {
		slog.Error("Deployment workspace cleanup failed", "error", e)
		return
	}
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	initialized := false
	for work.Err() == nil {
		pending, e := w.DB.Pending(work)
		if e != nil {
			slog.Error("Deployment queue read failed", "error", e)
			return
		}
		if !initialized {
			apps, e := w.DB.Applications(work)
			if e != nil {
				return
			}
			hasActive := false
			for _, a := range apps {
				if a.ActiveDeploymentID != "" {
					hasActive = true
				}
			}
			if hasActive || len(pending) > 0 {
				initCtx, cancel := context.WithTimeout(work, 3*time.Minute)
				e = w.Runtime.Ensure(initCtx)
				if e == nil {
					e = w.routes(initCtx, "", nil)
				}
				cancel()
				if e != nil {
					slog.Warn("Proxy reconciliation failed", "error", e)
				} else {
					initialized = true
				}
			} else {
				initialized = true
			}
		}
		if len(pending) > 0 {
			w.process(work, pending[0])
			continue
		}
		select {
		case <-work.Done():
			return
		case <-w.wake:
		case <-tick.C:
		}
	}
}
func (w *Worker) cleanWorkspaces() error {
	root, e := filepath.Abs(filepath.Join(w.DataDir, "work"))
	if e != nil {
		return e
	}
	entries, e := os.ReadDir(root)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.HasPrefix(entry.Name(), "deployment-") {
			continue
		}
		target := filepath.Join(root, entry.Name())
		if !strings.HasPrefix(target, root+string(filepath.Separator)) {
			return errors.New("Invalid deployment workspace path")
		}
		if e = os.RemoveAll(target); e != nil {
			return e
		}
	}
	return nil
}
func (w *Worker) log(d store.Deployment, line string) {
	// Tool output is text only, bounded in SQLite and never interpreted as markup.
	line = strings.Map(func(r rune) rune {
		if r < 32 && r != '\t' {
			return -1
		}
		return r
	}, line)
	if e := w.DB.AppendLog(context.Background(), d.ID, d.State, line); e != nil {
		slog.Error("Deployment log failed", "error", e)
	}
	w.Hub.Publish()
}
func (w *Worker) stage(ctx context.Context, d *store.Deployment, state string) error {
	d.State = state
	if e := w.DB.SaveDeployment(ctx, *d); e != nil {
		return e
	}
	w.changed()
	return nil
}
func (w *Worker) routes(ctx context.Context, exclude string, override *store.Deployment) error {
	apps, e := w.DB.Applications(ctx)
	if e != nil {
		return e
	}
	active := map[string]store.Deployment{}
	for _, a := range apps {
		if a.ID == exclude || a.ActiveDeploymentID == "" {
			continue
		}
		d, e := w.DB.Deployment(ctx, a.ActiveDeploymentID)
		if e != nil {
			return e
		}
		active[a.ID] = d
	}
	if override != nil {
		active[override.ApplicationID] = *override
	}
	return w.Runtime.Routes(ctx, apps, active)
}
func (w *Worker) process(parent context.Context, d store.Deployment) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Minute)
	defer cancel()
	if d.StartedAt == 0 {
		d.StartedAt = time.Now().Unix()
	}
	err := func() error {
		if e := w.Runtime.Ensure(ctx); e != nil {
			return fmt.Errorf("Proxy setup failed: %w", e)
		}
		if d.Operation == "delete" {
			if e := w.stage(ctx, &d, "deleting"); e != nil {
				return e
			}
			w.log(d, "Removing application routes")
			if e := w.routes(ctx, d.ApplicationID, nil); e != nil {
				return e
			}
			if e := w.Runtime.RemoveApplication(ctx, d.ApplicationID); e != nil {
				return e
			}
			if e := w.DB.MarkDeleted(ctx, d.ApplicationID); e != nil {
				return e
			}
			w.log(d, "Application containers removed; volumes and image history retained")
			return nil
		}
		if d.ImageID == "" {
			if e := w.stage(ctx, &d, "cloning"); e != nil {
				return e
			}
			w.log(d, "Cloning "+d.Config.Repository)
			root := filepath.Join(w.DataDir, "work")
			if e := os.MkdirAll(root, 0700); e != nil {
				return e
			}
			dir, e := os.MkdirTemp(root, "deployment-")
			if e != nil {
				return e
			}
			defer os.RemoveAll(dir)
			commit, e := w.Source.Clone(ctx, d.Config, dir, func(line string) { w.log(d, line) })
			if e != nil {
				return e
			}
			if d.Commit != "" && d.Commit != commit {
				return errors.New("The branch changed during recovery; redeploy to use the new revision")
			}
			d.Commit = commit
			if e = w.stage(ctx, &d, "packaging"); e != nil {
				return e
			}
			archive, e := StaticArchive(filepath.Join(dir, "repository"), d.Config, d.ID)
			if e != nil {
				return e
			}
			d.ImageID, e = w.Runtime.Package(ctx, d, archive, func(line string) { w.log(d, line) })
			if e != nil {
				return e
			}
		}
		if e := w.stage(ctx, &d, "starting"); e != nil {
			return e
		}
		var e error
		d.ContainerID, e = w.Runtime.Start(ctx, d)
		if e != nil {
			return e
		}
		if e = w.stage(ctx, &d, "checking"); e != nil {
			return e
		}
		w.log(d, "Checking the candidate HTTP response")
		if e = w.Runtime.Healthy(ctx, d); e != nil {
			return e
		}
		if e = w.stage(ctx, &d, "promoting"); e != nil {
			return e
		}
		w.log(d, "Switching application traffic")
		// Durable intent precedes the external routing change. Restart reconciliation
		// replays this release; the old release stays available until acknowledgment.
		if e = w.DB.SetActive(ctx, d.ApplicationID, d.ID); e != nil {
			return e
		}
		if e = w.routes(ctx, "", nil); e != nil {
			rollbackCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
			defer stop()
			if rollback := w.DB.SetActive(rollbackCtx, d.ApplicationID, d.PreviousID); rollback != nil {
				return fmt.Errorf("%w; routing recovery required: %v", e, rollback)
			}
			if rollback := w.routes(rollbackCtx, "", nil); rollback != nil {
				return fmt.Errorf("%w; previous route recovery pending: %v", e, rollback)
			}
			return e
		}
		w.log(d, "Application is serving at "+d.Config.Domain)
		if d.PreviousID != "" && d.PreviousID != d.ID {
			if old, e := w.DB.Deployment(ctx, d.PreviousID); e == nil {
				if e = w.Runtime.RemoveDeployment(ctx, old); e != nil {
					w.log(d, "Previous container cleanup will be retried during application deletion: "+e.Error())
				}
			}
		}
		return nil
	}()
	if parent.Err() != nil {
		return
	} // Durable stage remains resumable after shutdown.
	d.FinishedAt = time.Now().Unix()
	if err != nil {
		d.State = "failed"
		d.Error = err.Error()
		w.log(d, err.Error())
		if d.Operation != "delete" {
			cleanup, stop := context.WithTimeout(context.Background(), 20*time.Second)
			if a, e := w.DB.Application(cleanup, d.ApplicationID); e == nil && a.ActiveDeploymentID != d.ID {
				if e = w.Runtime.RemoveDeployment(cleanup, d); e != nil {
					w.log(d, "Candidate cleanup failed: "+e.Error())
				}
			}
			stop()
		}
	} else {
		d.State = "succeeded"
		d.Error = ""
	}
	if e := w.DB.SaveDeployment(context.Background(), d); e != nil {
		slog.Error("Deployment completion write failed", "error", e)
	}
	w.changed()
}
