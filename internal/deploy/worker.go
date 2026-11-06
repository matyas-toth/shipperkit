package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"shipper/internal/auth"
	"shipper/internal/events"
	"shipper/internal/gitprovider"
	"shipper/internal/store"
)

type Worker struct {
	DB          *store.Store
	Runtime     Runtime
	Source      Source
	Hub         *events.Hub
	DataDir     string
	wake        chan struct{}
	Refresh     func()
	Operations  func(context.Context)
	Maintenance func(context.Context) bool
}

func NewWorker(db *store.Store, r Runtime, hub *events.Hub, dir string) *Worker {
	return &Worker{DB: db, Runtime: r, Source: ConnectedSource{DB: db, GitHub: gitprovider.New()}, Hub: hub, DataDir: dir, wake: make(chan struct{}, 1)}
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
	var operationsDone chan struct{}
	if w.Operations != nil {
		operationsDone = make(chan struct{})
		go func() { defer close(operationsDone); w.Operations(work) }()
	}
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
	defer func() {
		stop()
		<-heartbeatDone
		if operationsDone != nil {
			<-operationsDone
		}
		w.DB.ReleaseLease(context.Background(), owner)
	}()
	if e := w.cleanWorkspaces(); e != nil {
		slog.Error("Deployment workspace cleanup failed", "error", e)
		return
	}
	_ = w.DB.InterruptBackups(work)
	if e := w.DB.ResetSourceClaims(work); e != nil {
		return
	}
	if e := w.cleanBackupWorkspaces(); e != nil {
		slog.Error("Backup workspace cleanup failed", "error", e)
		return
	}
	if runtime, ok := w.Runtime.(*DockerRuntime); ok {
		cleanup, cancel := context.WithTimeout(work, 10*time.Second)
		e := runtime.cleanBackupHelpers(cleanup)
		cancel()
		if e != nil {
			slog.Warn("Backup helper cleanup failed", "error", e)
		}
	}
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	initialized := false
	contact := ""
	for work.Err() == nil {
		if runtime, ok := w.Runtime.(*DockerRuntime); ok && runtime.Config.AutoHTTPS {
			if c, err := runtime.EffectiveProxy(work); err == nil && c.ACMEEmail != contact {
				contact = c.ACMEEmail
				initialized = false
			}
		}
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
			if dashboard, ok := w.Runtime.(*DockerRuntime); ok && dashboard.Config.DashboardUpstream != "" {
				hasActive = true
			}
			for _, a := range apps {
				if a.ActiveDeploymentID != "" {
					hasActive = true
				}
			}
			if hasActive || len(pending) > 0 {
				initCtx, cancel := context.WithTimeout(work, 3*time.Minute)
				e = w.Runtime.Ensure(initCtx)
				if e == nil {
					if naming, ok := w.Runtime.(interface {
						ReconcileName(context.Context, store.Deployment) error
					}); ok {
						for _, app := range apps {
							if app.ActiveDeploymentID != "" {
								if release, err := w.DB.Deployment(initCtx, app.ActiveDeploymentID); err == nil {
									if err = naming.ReconcileName(initCtx, release); err != nil {
										slog.Warn("Container naming update failed", "error", err)
									}
								}
							}
						}
					}
				}
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
		w.ScheduleBackups(work)
		w.scheduled(work)
		w.triggered(work)
		if len(pending) > 0 {
			w.process(work, pending[0])
			continue
		}
		if w.ProcessBackup(work) {
			continue
		}
		if w.Maintenance != nil && w.Maintenance(work) {
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
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || (!strings.HasPrefix(entry.Name(), "deployment-") && !strings.HasPrefix(entry.Name(), "inspection-")) {
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
	line = RedactDeployment(d, line)
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
		if a.ID == exclude || a.ActiveDeploymentID == "" || a.DesiredState == "stopped" || (a.Domains != nil && len(a.Domains) == 0) {
			continue
		}
		d, e := w.DB.Deployment(ctx, a.ActiveDeploymentID)
		if e != nil {
			return e
		}
		if a.LatestDeployment != nil && a.LatestDeployment.ID != d.ID && Method(a.LatestDeployment.Config) == "compose" && (a.LatestDeployment.State == "starting" || a.LatestDeployment.State == "checking" || a.LatestDeployment.State == "promoting") {
			continue
		}
		active[a.ID] = d
	}
	if override != nil {
		active[override.ApplicationID] = *override
	}
	return w.Runtime.Routes(ctx, apps, active)
}
func (w *Worker) process(parent context.Context, d store.Deployment) {
	if current, e := w.DB.Deployment(parent, d.ID); e == nil {
		d = current
		if d.State == "cancelled" {
			return
		}
	}
	timeout := 30 * time.Minute
	if d.Operation == "task" && d.TaskCommand != nil {
		timeout = time.Duration(d.TaskCommand.Timeout)*time.Second + time.Minute
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	deploymentID := d.ID
	cancellationDone := make(chan struct{})
	defer close(cancellationDone)
	go func() {
		tick := time.NewTicker(200 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-cancellationDone:
				return
			case <-ctx.Done():
				return
			case <-tick.C:
				if w.DB.CancellationRequested(parent, deploymentID) {
					cancel()
					return
				}
			}
		}
	}()
	if d.StartedAt == 0 {
		d.StartedAt = time.Now().Unix()
	}
	err := func() error {
		if d.Operation == "configure" {
			w.log(d, "Automatic deployment settings updated")
			return nil
		}
		if e := w.Runtime.Ensure(ctx); e != nil {
			return fmt.Errorf("Proxy setup failed: %w", e)
		}
		if strings.HasPrefix(d.Operation, "service_") {
			if e := w.stage(ctx, &d, "operating"); e != nil {
				return e
			}
			release, e := w.DB.Deployment(ctx, d.PreviousID)
			if e != nil {
				return e
			}
			operator, ok := w.Runtime.(interface {
				LifecycleService(context.Context, store.Deployment, string, string, ...func(string)) error
			})
			if !ok {
				return errors.New("Service actions are unavailable")
			}
			action := strings.TrimPrefix(d.Operation, "service_")
			if check, ok := w.Runtime.(interface {
				CheckServiceAction(context.Context, store.Deployment, string, string) error
			}); ok {
				if e = check.CheckServiceAction(ctx, release, d.Service, action); e != nil {
					return e
				}
			}
			if action != "start" {
				hookRelease := release
				hookRelease.ID = d.ID
				runs := []hookRun{}
				for _, run := range hookRuns(release, "before_stop") {
					if run.command.Service == d.Service {
						runs = append(runs, run)
					}
				}
				if e = w.runHooks(ctx, hookRelease, runs, true); e != nil {
					return e
				}
			}
			if e = operator.LifecycleService(ctx, release, d.Service, action, func(line string) { w.log(d, line) }); e != nil {
				return e
			}
			w.log(d, "Service "+d.Service+" "+action+" completed")
			return nil
		}
		if d.Operation == "task" {
			if d.TaskCommand == nil {
				return errors.New("Task command is unavailable")
			}
			if e := w.stage(ctx, &d, "operating"); e != nil {
				return e
			}
			release, e := w.DB.Deployment(ctx, d.PreviousID)
			if e != nil {
				return e
			}
			release.ID = d.ID
			return w.runTask(ctx, release, "task", *d.TaskCommand)
		}
		if d.Operation == "domains" {
			if e := w.stage(ctx, &d, "routing"); e != nil {
				return e
			}
			w.log(d, "Updating domains without rebuilding the application")
			if e := w.DB.SetDomains(ctx, d.ApplicationID, d.Config.Domains); e != nil {
				return e
			}
			if e := w.routes(ctx, "", nil); e != nil {
				rollback, stop := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
				defer stop()
				w.DB.SetDomains(rollback, d.ApplicationID, d.PreviousDomains)
				w.routes(rollback, "", nil)
				w.DB.ReleaseDomainClaims(rollback, d.ApplicationID)
				return e
			}
			if e := w.DB.ReleaseDomainClaims(ctx, d.ApplicationID); e != nil {
				return e
			}
			w.log(d, "Application domains updated")
			return nil
		}
		if d.Operation == "start" || d.Operation == "stop" || d.Operation == "restart" {
			if e := w.stage(ctx, &d, "operating"); e != nil {
				return e
			}
			release, e := w.DB.Deployment(ctx, d.PreviousID)
			if e != nil {
				return e
			}
			operator, ok := w.Runtime.(interface {
				Lifecycle(context.Context, store.Deployment, string) error
			})
			if !ok {
				return errors.New("Lifecycle operations are unavailable")
			}
			if d.Operation != "start" {
				hookRelease := release
				hookRelease.ID = d.ID
				if e = w.hooks(ctx, hookRelease, "before_stop", true); e != nil {
					return e
				}
				if e = w.DB.SetDesiredState(ctx, d.ApplicationID, "stopped"); e != nil {
					return e
				}
				if e = w.routes(ctx, "", nil); e != nil {
					return e
				}
			}
			if e = operator.Lifecycle(ctx, release, d.Operation); e != nil {
				return e
			}
			if d.Operation != "stop" {
				if e = w.Runtime.Healthy(ctx, release); e != nil {
					return e
				}
				if e = w.DB.SetDesiredState(ctx, d.ApplicationID, "running"); e != nil {
					return e
				}
				if e = w.routes(ctx, "", nil); e != nil {
					return e
				}
			}
			w.log(d, "Application "+map[string]string{"start": "started", "stop": "stopped", "restart": "restarted"}[d.Operation])
			return nil
		}
		if d.Operation == "delete" {
			if e := w.stage(ctx, &d, "deleting"); e != nil {
				return e
			}
			if d.PreviousID != "" {
				if release, e := w.DB.Deployment(ctx, d.PreviousID); e == nil {
					release.ID = d.ID
					if e = w.hooks(ctx, release, "before_stop", true); e != nil {
						return e
					}
				}
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
			w.DB.ReleaseDomainClaims(ctx, d.ApplicationID)
			w.log(d, "Application containers removed; volumes and image history retained")
			return nil
		}
		if d.Operation == "rollback" && d.ImageID == "" {
			target, e := w.DB.Deployment(ctx, d.RollbackID)
			if e != nil || target.ApplicationID != d.ApplicationID || target.State != "succeeded" {
				return errors.New("The recorded release is unavailable")
			}
			checker, ok := w.Runtime.(interface {
				CheckRelease(context.Context, store.Deployment) error
			})
			if !ok {
				return errors.New("Release recovery is unavailable")
			}
			if e = checker.CheckRelease(ctx, target); e != nil {
				return e
			}
			d.Commit = target.Commit
			d.ImageID = target.ImageID
			d.Digest = target.Digest
			d.ResolvedMethod = target.ResolvedMethod
			d.RuntimePort = target.RuntimePort
			d.Services = target.Services
			d.ComposePlan = target.ComposePlan
			if Method(d.Config) == "compose" {
				if e = prepareRollbackCompose(&d); e != nil {
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
			}
			w.log(d, "Restoring recorded code and runtime settings; current domains and persistent data are retained")
		}
		if d.Operation == "apply" && d.ImageID == "" && d.PreviousID != "" {
			previous, e := w.DB.Deployment(ctx, d.PreviousID)
			if e != nil {
				return e
			}
			if PlanChanges(previous.Config, d.Config).Operation != "rebuild" {
				d.Commit = previous.Commit
				d.ResolvedMethod = previous.ResolvedMethod
				d.RuntimePort = 0
				if Method(d.Config) == "compose" {
					r, ok := w.Runtime.(interface {
						ReuseCompose(context.Context, *store.Deployment, store.Deployment) error
					})
					if !ok {
						return errors.New("Compose recreation is unavailable")
					}
					if e = r.ReuseCompose(ctx, &d, previous); e != nil {
						return e
					}
				} else {
					d.ImageID = previous.ImageID
					d.Digest = previous.Digest
				}
				w.log(d, "Reusing the current release image without cloning or rebuilding")
			}
		}
		if Method(d.Config) == "image" && d.ImageID == "" {
			if e := w.stage(ctx, &d, "pulling"); e != nil {
				return e
			}
			resolver, ok := w.Runtime.(interface {
				PullImage(context.Context, *store.Deployment, *store.Connection, func(string)) error
			})
			if !ok {
				return errors.New("Image deployments are unavailable")
			}
			var registry *store.Connection
			if d.Config.RegistryID != "" {
				c, e := w.DB.Connection(ctx, d.Config.RegistryID)
				if e != nil {
					return errors.New("Registry connection is unavailable")
				}
				registry = &c
			}
			if e := resolver.PullImage(ctx, &d, registry, func(line string) { w.log(d, line) }); e != nil {
				return e
			}
		}
		if d.ImageID == "" {
			facts, err := w.DB.Observations(ctx, "")
			if err != nil {
				return err
			}
			for _, fact := range facts {
				if fact.ID == "disk" && !fact.Stale {
					var disk struct {
						Free uint64 `json:"free"`
					}
					if json.Unmarshal(fact.Details, &disk) == nil && disk.Free < 128<<20 {
						return errors.New("Data filesystem has less than 128 MiB available; review storage before building")
					}
				}
			}
		}
		if Method(d.Config) == "compose" {
			return w.deployCompose(ctx, &d)
		}
		if d.ImageID == "" {
			if e := w.stage(ctx, &d, "cloning"); e != nil {
				return e
			}
			if d.Config.LocalSource != "" {
				w.log(d, "Loading local source")
			} else {
				w.log(d, "Cloning "+d.Config.Repository)
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
			commit, e := w.Source.Clone(ctx, cloneConfig(d), dir, func(line string) { w.log(d, line) })
			if e != nil {
				return e
			}
			if d.Commit != "" && d.Commit != commit {
				return errors.New("The branch changed during recovery; redeploy to use the new revision")
			}
			d.Commit = commit
			appRoot, e := SourceRoot(filepath.Join(dir, "repository"), d.Config)
			if e != nil {
				return e
			}
			if Method(d.Config) == "auto" {
				d.ResolvedMethod = DetectMethod(appRoot)
			} else {
				d.ResolvedMethod = Method(d.Config)
				if d.ResolvedMethod == "inline" {
					d.ResolvedMethod = "dockerfile"
				}
			}
			stage := "building"
			if d.ResolvedMethod == "static" && d.Config.BuildCommand == "" {
				stage = "packaging"
			}
			if e = w.stage(ctx, &d, stage); e != nil {
				return e
			}
			if builder, ok := w.Runtime.(Builder); ok {
				d.ImageID, e = builder.Build(ctx, d, appRoot, func(line string) { w.log(d, line) })
			} else {
				archive, err := StaticArchive(appRoot, d.Config, d.ID)
				if err != nil {
					return err
				}
				d.ImageID, e = w.Runtime.Package(ctx, d, archive, func(line string) { w.log(d, line) })
			}
			if e != nil {
				return e
			}
		}
		if resolver, ok := w.Runtime.(interface {
			ResolveRuntime(context.Context, *store.Deployment) error
		}); ok {
			if e := resolver.ResolveRuntime(ctx, &d); e != nil {
				return e
			}
		}
		if e := w.prepareStartup(ctx, &d); e != nil {
			return e
		}
		if e := w.stage(ctx, &d, "before_deploy"); e != nil {
			return e
		}
		if e := w.deploymentHooks(ctx, d, "before_deploy", false); e != nil {
			return e
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
		application, e := w.DB.Application(ctx, d.ApplicationID)
		if e != nil {
			return e
		}
		if e = w.DB.SetDesiredState(ctx, d.ApplicationID, "running"); e != nil {
			return e
		}
		if e = w.routes(ctx, "", nil); e != nil {
			rollbackCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
			defer stop()
			w.DB.SetDesiredState(rollbackCtx, d.ApplicationID, application.DesiredState)
			if rollback := w.DB.SetActive(rollbackCtx, d.ApplicationID, d.PreviousID); rollback != nil {
				return fmt.Errorf("%w; routing recovery required: %v", e, rollback)
			}
			if rollback := w.routes(rollbackCtx, "", nil); rollback != nil {
				return fmt.Errorf("%w; previous route recovery pending: %v", e, rollback)
			}
			return e
		}
		if d.Config.Domain != "" {
			w.log(d, "Application is serving at "+d.Config.Domain)
		} else {
			w.log(d, "Application is running without public domains")
		}
		if d.PreviousID != "" && d.PreviousID != d.ID {
			if d.Config.Runtime.DrainSeconds > 0 {
				w.log(d, "Draining the previous release")
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Duration(d.Config.Runtime.DrainSeconds) * time.Second):
				}
			}
			if old, e := w.DB.Deployment(ctx, d.PreviousID); e == nil {
				old.ID = d.ID
				if e = w.hooks(ctx, old, "before_stop", true); e != nil {
					return e
				}
				old.ID = d.PreviousID
				if e = w.Runtime.RemoveDeployment(ctx, old); e != nil {
					w.log(d, "Previous container cleanup will be retried during application deletion: "+e.Error())
				}
			}
		}
		return w.deploymentHooks(ctx, d, "after_deploy", true)
	}()
	if parent.Err() != nil {
		return
	} // Durable stage remains resumable after shutdown.
	d.FinishedAt = time.Now().Unix()
	if err != nil {
		d.State = "failed"
		if w.DB.CancellationRequested(context.Background(), d.ID) {
			d.State = "cancelled"
			err = store.ErrCancelled
		}
		d.Error = RedactDeployment(d, err.Error())
		w.log(d, err.Error())
		if (d.Operation == "deploy" || d.Operation == "apply" || d.Operation == "rollback") && Method(d.Config) != "compose" {
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
