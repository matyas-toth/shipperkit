// Package operations collects bounded facts and delivers transition notifications
// inside the existing control plane. It never evaluates model output or runs a host shell.
package operations

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
	"shipper/internal/inventory"
	"shipper/internal/store"
)

type Service struct {
	ConfigIdentity    string
	RepairProxy       func(context.Context) error
	DB                *store.Store
	Docker            *client.Client
	Monitor           *inventory.Monitor
	Instance, DataDir string
	DashboardURL      string
	Publish           func()
	certificateOffset int
	certificates      map[string]certificateSchedule
	offset            int
	previous          map[string]container.CPUStats
}

func (s *Service) Run(ctx context.Context) {
	tick := time.NewTicker(15 * time.Second)
	defer tick.Stop()
	for ctx.Err() == nil {
		work, cancel := context.WithTimeout(ctx, 12*time.Second)
		s.Collect(work)
		s.Deliver(work)
		s.AutomaticRetention(work)
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
func (s *Service) record(ctx context.Context, o store.Observation) {
	if e := s.DB.Observe(ctx, o); e != nil && ctx.Err() == nil {
		slog.Warn("Observation could not be saved", "error", e)
	}
}
func (s *Service) Collect(ctx context.Context) {
	h := s.Monitor.Health()
	state := "healthy"
	message := "Docker is connected"
	if !h.Connected {
		state = "failed"
		message = "Docker is unavailable"
	}
	if h.CheckedAt == 0 {
		state = "unknown"
		message = "Docker has not been checked"
	}
	s.record(ctx, store.Observation{ID: "daemon", Kind: "daemon", State: state, Summary: message, ObservedAt: h.CheckedAt})
	if s.Docker != nil && h.Connected && len(s.Instance) >= 12 {
		info, e := s.Docker.ContainerInspect(ctx, "shipper-"+s.Instance[:12]+"-proxy", client.ContainerInspectOptions{})
		state = "unknown"
		message = "Proxy has not been observed"
		if e == nil && info.Container.Config != nil && info.Container.Config.Labels["io.shipper.instance"] == s.Instance && info.Container.State != nil {
			state = "healthy"
			message = "Proxy is running"
			if !info.Container.State.Running {
				state = "failed"
				message = "Proxy is stopped"
			}
		}
		s.record(ctx, store.Observation{ID: "proxy", Kind: "proxy", State: state, Summary: message})
	}
	if disk, e := Disk(s.DataDir); e == nil {
		state = "healthy"
		message = "Data filesystem has available space"
		if disk.Free < 512<<20 || disk.Total > 0 && float64(disk.Free)/float64(disk.Total) < 0.1 {
			state = "warning"
			message = "Data filesystem is running low on space"
		}
		raw, _ := json.Marshal(disk)
		s.record(ctx, store.Observation{ID: "disk", Kind: "storage", State: state, Summary: message, Details: raw})
	}
	apps, e := s.DB.Applications(ctx)
	if e != nil {
		return
	}
	all := []store.Container{}
	for _, a := range apps {
		if ctx.Err() != nil {
			return
		}
		if d := a.LatestDeployment; d != nil {
			state = d.State
			if d.State == "succeeded" {
				state = "healthy"
			} else if d.State == "failed" || d.State == "interrupted" {
				state = "failed"
			} else if d.State == "cancelled" {
				state = "cancelled"
			}
			s.record(ctx, store.Observation{ID: "operation/" + a.ID, ApplicationID: a.ID, Kind: func() string {
				if d.Operation == "task" {
					return "task"
				}
				return "deployment"
			}(), State: state, Summary: "Latest " + d.Operation + " is " + d.State, Reference: d.ID})
		}
		for _, d := range a.Domains {
			if !d.HTTPS || s.DashboardURL != "" {
				continue
			}
			state = "unknown"
			message = "Certificate has not been observed"
			if d.CertificateExpiresAt > 0 {
				state = "healthy"
				message = "Certificate expiry is observed"
				if d.CertificateExpiresAt < time.Now().Add(14*24*time.Hour).Unix() {
					state = "warning"
					message = "Certificate expires within 14 days"
				}
			}
			s.record(ctx, store.Observation{ID: "certificate/" + a.ID + "/" + d.ID, ApplicationID: a.ID, Kind: "certificate", State: state, Summary: message, Reference: d.ID, ObservedAt: d.CheckedAt})
		}
		plans, _ := s.DB.BackupPlans(ctx, a.ID)
		runs, _ := s.DB.BackupRuns(ctx, a.ID)
		for _, p := range plans {
			if !p.Enabled {
				continue
			}
			state = "warning"
			message = "Backup has not completed"
			ref := ""
			for _, r := range runs {
				if r.PlanID != p.ID {
					continue
				}
				ref = r.ID
				if r.State == "succeeded" {
					state = "healthy"
					message = "Backup completed"
					if p.Schedule != "" && p.NextAt > 0 && time.Now().Unix() > p.NextAt+3600 {
						state = "warning"
						message = "Scheduled backup is overdue"
					}
				} else if r.State == "failed" || r.State == "interrupted" {
					state = "failed"
					message = "Backup failed"
				} else {
					state = "running"
					message = "Backup is " + r.State
				}
				break
			}
			s.record(ctx, store.Observation{ID: "backup/" + p.ID, ApplicationID: a.ID, Kind: "backup", State: state, Summary: message, Reference: ref})
		}
		for _, c := range a.Containers {
			if c.Managed {
				all = append(all, c)
			}
		}
	}
	s.checkCertificate(ctx, apps)
	// A rotating budget avoids inspecting hundreds of containers every interval.
	// Unvisited samples retain their own timestamps and become visibly stale.
	if s.Docker != nil && h.Connected && len(all) > 0 {
		if s.previous == nil {
			s.previous = map[string]container.CPUStats{}
		}
		alive := map[string]bool{}
		for _, c := range all {
			alive[c.ID] = true
		}
		for id := range s.previous {
			if !alive[id] {
				delete(s.previous, id)
			}
		}
		for i := 0; i < min(32, len(all)) && ctx.Err() == nil; i++ {
			c := all[(s.offset+i)%len(all)]
			s.sample(ctx, c)
		}
		s.offset = (s.offset + 32) % len(all)
	}
	_ = s.DB.RetainObservations(ctx)
}
func (s *Service) sample(ctx context.Context, c store.Container) {
	probe, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	inspected, e := s.Docker.ContainerInspect(probe, c.ID, client.ContainerInspectOptions{})
	if e != nil {
		return
	}
	v := inspected.Container
	if v.Config == nil || v.State == nil || v.Config.Labels["io.shipper.instance"] != s.Instance || v.Config.Labels["io.shipper.application.id"] != c.ApplicationID || v.Config.Labels["io.shipper.managed"] != "true" {
		return
	}
	state := "healthy"
	message := "Service is " + c.State
	app, e := s.DB.Application(ctx, c.ApplicationID)
	if e != nil {
		return
	}
	oneShot := false
	if app.ActiveDeploymentID != "" {
		if release, e := s.DB.Deployment(ctx, app.ActiveDeploymentID); e == nil {
			for _, svc := range release.Services {
				if svc.Name == c.Service {
					oneShot = svc.OneShot
				}
			}
		}
	}
	if app.DesiredState != "stopped" && (!v.State.Running && !oneShot || c.Health == "unhealthy" || v.State.Restarting || v.State.OOMKilled) {
		state = "failed"
		message = "Service needs attention"
	}
	details := map[string]any{"containerId": c.ID, "service": c.Service, "restarts": v.RestartCount, "oomKilled": v.State.OOMKilled, "exitCode": v.State.ExitCode, "state": c.State, "health": c.Health, "cpuPercent": nil, "memoryBytes": nil, "memoryLimit": nil}
	networks := []string{}
	if v.NetworkSettings != nil {
		for name := range v.NetworkSettings.Networks {
			networks = append(networks, name)
		}
	}
	details["networks"] = networks
	if v.State.Running {
		stats, e := s.Docker.ContainerStats(probe, c.ID, client.ContainerStatsOptions{})
		if e == nil {
			var sample container.StatsResponse
			e = json.NewDecoder(io.LimitReader(stats.Body, 256<<10)).Decode(&sample)
			stats.Body.Close()
			if e == nil {
				details["memoryBytes"] = sample.MemoryStats.Usage
				details["memoryLimit"] = sample.MemoryStats.Limit
				old, ok := s.previous[c.ID]
				if ok && sample.CPUStats.SystemUsage > old.SystemUsage && sample.CPUStats.CPUUsage.TotalUsage >= old.CPUUsage.TotalUsage {
					cpus := sample.CPUStats.OnlineCPUs
					if cpus == 0 {
						cpus = 1
					}
					details["cpuPercent"] = float64(sample.CPUStats.CPUUsage.TotalUsage-old.CPUUsage.TotalUsage) / float64(sample.CPUStats.SystemUsage-old.SystemUsage) * float64(cpus) * 100
				}
				s.previous[c.ID] = sample.CPUStats
			}
		}
	}
	raw, _ := json.Marshal(details)
	s.record(ctx, store.Observation{ID: "container/" + c.ID, ApplicationID: c.ApplicationID, Kind: "service", State: state, Summary: message, Reference: c.ID, Details: raw})
}

type DiskInfo struct {
	Total uint64 `json:"total"`
	Free  uint64 `json:"free"`
	Scope string `json:"scope"`
}

func ObservationID(kind, app, reference string) string {
	return fmt.Sprintf("%s/%s/%s", kind, app, reference)
}
