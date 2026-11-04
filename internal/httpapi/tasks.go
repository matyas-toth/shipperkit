package httpapi

import (
	"github.com/go-chi/chi/v5"
	"net/http"
	"shipper/internal/auth"
	"shipper/internal/deploy"
	"shipper/internal/store"
	"strings"
	"time"
)

func (s *Server) tasks(w http.ResponseWriter, r *http.Request) {
	a, e := s.db.Application(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		s.resourceError(w, e)
		return
	}
	tasks, e := s.db.Tasks(r.Context(), a.ID)
	if e != nil {
		s.internal(w, e)
		return
	}
	runs, e := s.db.TaskRuns(r.Context(), a.ID)
	if e != nil {
		s.internal(w, e)
		return
	}
	s.json(w, 200, map[string]any{"items": tasks, "runs": runs})
}
func (s *Server) saveTask(w http.ResponseWriter, r *http.Request) {
	a, e := s.db.Application(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		s.resourceError(w, e)
		return
	}
	var t store.Task
	if !s.decode(w, r, &t) {
		return
	}
	t.ApplicationID = a.ID
	t.Name = strings.TrimSpace(t.Name)
	if !validName(t.Name) {
		s.problem(w, 422, "invalid_name", "Enter a task name")
		return
	}
	if e = deploy.ValidateCommand(&t.Command); e != nil {
		s.problem(w, 422, "invalid_command", e.Error())
		return
	}
	if a.Method == "compose" && t.Command.Service == "" || a.Method != "compose" && t.Command.Service != "" {
		s.problem(w, 422, "invalid_service", "Choose a service from this application")
		return
	}
	if a.Method == "compose" && a.ActiveDeploymentID != "" {
		release, e := s.db.Deployment(r.Context(), a.ActiveDeploymentID)
		if e != nil {
			s.internal(w, e)
			return
		}
		found := false
		for _, svc := range release.Services {
			if svc.Name == t.Command.Service {
				found = true
			}
		}
		if !found {
			s.problem(w, 422, "invalid_service", "Choose a service from this release")
			return
		}
	}
	if t.Timezone == "" {
		t.Timezone = "UTC"
	}
	if t.Overlap == "" {
		t.Overlap = "skip"
	}
	if t.Missed == "" {
		t.Missed = "skip"
	}
	if (t.Overlap != "skip" && t.Overlap != "queue") || (t.Missed != "skip" && t.Missed != "once") {
		s.problem(w, 422, "invalid_policy", "Choose a supported schedule policy")
		return
	}
	if t.Schedule != "" {
		next, e := deploy.NextSchedule(t.Schedule, t.Timezone, time.Now())
		if e != nil {
			s.problem(w, 422, "invalid_schedule", e.Error())
			return
		}
		t.NextAt = next.Unix()
	}
	if t.ID == "" {
		t.ID = auth.Token()[:24]
	} else {
		if _, e = s.db.Task(r.Context(), a.ID, t.ID); e != nil {
			s.resourceError(w, e)
			return
		}
	}
	if e = s.db.SaveTask(r.Context(), t); e != nil {
		s.internal(w, e)
		return
	}
	s.hub.Publish()
	s.json(w, 200, t)
}
func (s *Server) runTask(w http.ResponseWriter, r *http.Request) {
	if !s.mutationAvailable(w) {
		return
	}
	a, e := s.db.Application(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		s.resourceError(w, e)
		return
	}
	t, e := s.db.Task(r.Context(), a.ID, chi.URLParam(r, "task"))
	if e != nil {
		s.resourceError(w, e)
		return
	}
	if !t.Enabled {
		s.problem(w, 409, "task_disabled", "Enable the task before running it")
		return
	}
	if a.ActiveDeploymentID == "" || a.DesiredState == "stopped" {
		s.problem(w, 409, "not_running", "Start the application before running a task")
		return
	}
	release, e := s.db.Deployment(r.Context(), a.ActiveDeploymentID)
	if e != nil {
		s.internal(w, e)
		return
	}
	a.Config = release.Config
	s.enqueueJob(w, r, a, store.Deployment{Operation: "task", TaskID: t.ID, TaskCommand: &t.Command}, false)
}
func (s *Server) deleteTask(w http.ResponseWriter, r *http.Request) {
	if _, e := s.db.Task(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "task")); e != nil {
		s.resourceError(w, e)
		return
	}
	if e := s.db.DeleteTask(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "task")); e != nil {
		s.internal(w, e)
		return
	}
	s.hub.Publish()
	s.json(w, 200, map[string]bool{"deleted": true})
}
func (s *Server) schedulePreview(w http.ResponseWriter, r *http.Request) {
	if id := chi.URLParam(r, "id"); id != "" {
		if _, e := s.db.Application(r.Context(), id); e != nil {
			s.resourceError(w, e)
			return
		}
	}
	var input struct {
		Schedule string `json:"schedule"`
		Timezone string `json:"timezone"`
	}
	if !s.decode(w, r, &input) {
		return
	}
	now := time.Now()
	out := []int64{}
	for range 3 {
		next, e := deploy.NextSchedule(input.Schedule, input.Timezone, now)
		if e != nil {
			s.problem(w, 422, "invalid_schedule", e.Error())
			return
		}
		out = append(out, next.Unix())
		now = next
	}
	s.json(w, 200, map[string]any{"items": out})
}
