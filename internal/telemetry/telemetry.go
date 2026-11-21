// Package telemetry sends a small anonymous usage report once a day so the
// project can count active installations and see which versions, platforms
// and features are in use. Reports contain counts and enumerated values only:
// never names, hostnames, repositories, emails, IP addresses or secrets.
package telemetry

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	mathrand "math/rand/v2"
	"net/http"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"shipper/internal/store"
)

const (
	DefaultEndpoint = "https://telemetry.shipperkit.dev/api/v1/ingest"
	Schema          = 1

	idKey       = "telemetry_id"
	enabledKey  = "telemetry_enabled"
	lastSentKey = "telemetry_last_sent"

	maxKeys = 24
)

var keyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,39}$`)

// Platform describes the Docker host. Values come from the Docker engine.
type Platform struct {
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	HostOS   string `json:"hostOs"`
	Docker   string `json:"docker"`
	CPUs     int    `json:"cpus"`
	MemoryMB int64  `json:"memoryMb"`
}

type Usage struct {
	Applications        int   `json:"applications"`
	Services            int   `json:"services"`
	Domains             int   `json:"domains"`
	Users               int   `json:"users"`
	Deployments         int   `json:"deployments"`
	FailedDeployments   int   `json:"failedDeployments"`
	Rollbacks           int   `json:"rollbacks"`
	DeployMedianSeconds int64 `json:"deployMedianSeconds"`
	BackupPlans         int   `json:"backupPlans"`
	Tasks               int   `json:"tasks"`
	AccessTokens        int   `json:"accessTokens"`
}

// Report is the complete document sent to the telemetry endpoint. The
// settings page renders it verbatim so operators can see every field.
type Report struct {
	Schema      int            `json:"schema"`
	ID          string         `json:"id"`
	Version     string         `json:"version"`
	Channel     string         `json:"channel"`
	AutoUpdate  bool           `json:"autoUpdate"`
	Install     string         `json:"install"`
	UptimeHours int            `json:"uptimeHours"`
	Platform    Platform       `json:"platform"`
	Usage       Usage          `json:"usage"`
	Methods     map[string]int `json:"methods"`
	Sources     map[string]int `json:"sources"`
	Databases   map[string]int `json:"databases"`
	Catalog     map[string]int `json:"catalog"`
	Connections map[string]int `json:"connections"`
	Features    []string       `json:"features"`
}

type Config struct {
	Store    *store.Store
	Version  string
	Endpoint string
	// Allowed is false when the environment opts out. The setting cannot
	// override it.
	Allowed  bool
	Managed  bool
	Platform func() Platform
	Updates  func() (channel string, auto bool)
	HTTPS    func(context.Context) bool
	Interval time.Duration
	Client   *http.Client
	Now      func() time.Time
}

type Service struct {
	cfg     Config
	started time.Time
	mu      sync.Mutex
	wake    chan struct{}
}

type State struct {
	Allowed  bool    `json:"allowed"`
	Enabled  bool    `json:"enabled"`
	LastSent int64   `json:"lastSent"`
	Report   *Report `json:"report,omitempty"`
}

func New(cfg Config) *Service {
	if cfg.Endpoint == "" {
		cfg.Endpoint = DefaultEndpoint
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 24 * time.Hour
	}
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: 15 * time.Second}
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Service{cfg: cfg, started: cfg.Now(), wake: make(chan struct{}, 1)}
}

func (s *Service) Enabled(ctx context.Context) bool {
	if s == nil || !s.cfg.Allowed || s.cfg.Store == nil {
		return false
	}
	value, err := s.cfg.Store.Setting(ctx, enabledKey, "1")
	return err == nil && value != "0"
}

func (s *Service) State(ctx context.Context) (State, error) {
	state := State{Allowed: s.cfg.Allowed, Enabled: s.Enabled(ctx)}
	if last := s.lastSent(ctx); !last.IsZero() {
		state.LastSent = last.Unix()
	}
	report, err := s.Collect(ctx)
	if err != nil {
		return state, err
	}
	state.Report = &report
	return state, nil
}

func (s *Service) SetEnabled(ctx context.Context, enabled bool) error {
	if !s.cfg.Allowed {
		return errors.New("Usage data is turned off by the environment")
	}
	value := "0"
	if enabled {
		value = "1"
	}
	if err := s.cfg.Store.SaveSetting(ctx, enabledKey, value); err != nil {
		return err
	}
	if enabled {
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
	return nil
}

func (s *Service) id(ctx context.Context) (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	// A dedicated random identifier keeps reports unlinkable to the instance
	// ID used for container labels and backups.
	return s.cfg.Store.Setting(ctx, idKey, hex.EncodeToString(raw))
}

func (s *Service) lastSent(ctx context.Context) time.Time {
	raw, err := s.cfg.Store.Setting(ctx, lastSentKey, "")
	if err != nil || raw == "" {
		return time.Time{}
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return time.Time{}
	}
	return time.Unix(value, 0)
}

// Collect builds the report from aggregate queries. It is safe to call often.
func (s *Service) Collect(ctx context.Context) (Report, error) {
	id, err := s.id(ctx)
	if err != nil {
		return Report{}, err
	}
	now := s.cfg.Now()
	report := Report{
		Schema: Schema, ID: id, Version: s.cfg.Version, Channel: "main", Install: "manual",
		UptimeHours: int(now.Sub(s.started).Hours()),
		Platform:    Platform{OS: runtime.GOOS, Arch: runtime.GOARCH},
		Methods:     map[string]int{}, Sources: map[string]int{}, Databases: map[string]int{}, Catalog: map[string]int{}, Connections: map[string]int{},
		Features: []string{},
	}
	if s.cfg.Managed {
		report.Install = "managed"
	}
	if s.cfg.Updates != nil {
		channel, auto := s.cfg.Updates()
		if channel != "" {
			report.Channel = channel
		}
		report.AutoUpdate = auto
	}
	if s.cfg.Platform != nil {
		platform := s.cfg.Platform()
		platform.OS, platform.Arch = runtime.GOOS, runtime.GOARCH
		platform.HostOS = clip(platform.HostOS, 80)
		platform.Docker = clip(platform.Docker, 40)
		report.Platform = platform
	}
	apps, err := s.cfg.Store.Applications(ctx)
	if err != nil {
		return report, err
	}
	features := map[string]bool{}
	for _, app := range apps {
		report.Usage.Applications++
		method := app.ActiveMethod
		if method == "" {
			method = app.Method
		}
		if method == "" {
			method = app.Config.Method
		}
		increment(report.Methods, method)
		increment(report.Sources, app.SourceKind)
		if app.Config.Database != nil {
			increment(report.Databases, app.Config.Database.Engine)
		}
		if app.Config.CatalogService != "" {
			increment(report.Catalog, app.Config.CatalogService)
		}
		services := len(app.Config.Services)
		if services == 0 {
			services = 1
		}
		report.Usage.Services += services
		report.Usage.Domains += len(app.Domains)
		if app.Config.AutoDeploy.Enabled {
			features["auto_deploy"] = true
		}
		if len(app.Config.Hooks) > 0 {
			features["hooks"] = true
		}
		if len(app.Config.Volumes) > 0 {
			features["volumes"] = true
		}
		for _, domain := range app.Domains {
			if domain.HTTPS {
				features["https"] = true
			}
		}
	}
	counts, err := s.cfg.Store.TelemetryCounts(ctx, now.Add(-24*time.Hour))
	if err != nil {
		return report, err
	}
	report.Usage.Users = counts.Users
	report.Usage.Deployments = counts.Deployments
	report.Usage.FailedDeployments = counts.FailedDeploys
	report.Usage.Rollbacks = counts.Rollbacks
	report.Usage.DeployMedianSeconds = counts.DeployMedianSec
	report.Usage.BackupPlans = counts.BackupPlans
	report.Usage.Tasks = counts.Tasks
	report.Usage.AccessTokens = counts.AccessTokens
	for kind, count := range counts.Connections {
		report.Connections[normalize(kind)] += count
	}
	features["backups"] = counts.BackupPlans > 0
	features["tasks"] = counts.Tasks > 0
	features["access_tokens"] = counts.AccessTokens > 0
	features["two_factor"] = counts.FactorUsers > 0
	features["sso"] = counts.LinkedIdentities > 0
	features["team"] = counts.Users > 1
	features["auto_update"] = report.AutoUpdate
	if s.cfg.HTTPS != nil {
		features["dashboard_https"] = s.cfg.HTTPS(ctx)
	}
	for name, on := range features {
		if on {
			report.Features = append(report.Features, name)
		}
	}
	sort.Strings(report.Features)
	for _, m := range []map[string]int{report.Methods, report.Sources, report.Databases, report.Catalog, report.Connections} {
		trim(m)
	}
	return report, nil
}

// Run reports once per interval. The first report waits a few minutes after
// start so crash loops and short test runs never send anything, and the
// schedule survives restarts through the last-sent setting.
func (s *Service) Run(ctx context.Context) {
	if s == nil || !s.cfg.Allowed {
		return
	}
	failures := 0
	for {
		delay := s.nextDelay(ctx, failures)
		select {
		case <-ctx.Done():
			return
		case <-s.wake:
			continue
		case <-time.After(delay):
		}
		if !s.Enabled(ctx) {
			continue
		}
		if last := s.lastSent(ctx); !last.IsZero() && s.cfg.Now().Sub(last) < s.cfg.Interval-time.Hour {
			continue
		}
		if err := s.send(ctx); err != nil {
			failures++
			slog.Debug("Usage report was not delivered", "error", err)
			continue
		}
		failures = 0
	}
}

func (s *Service) nextDelay(ctx context.Context, failures int) time.Duration {
	settle := 5*time.Minute + time.Duration(mathrand.Int64N(int64(10*time.Minute)))
	if failures > 0 {
		return min(time.Duration(failures)*time.Hour, 6*time.Hour) + time.Duration(mathrand.Int64N(int64(10*time.Minute)))
	}
	if !s.Enabled(ctx) {
		return time.Hour
	}
	last := s.lastSent(ctx)
	if last.IsZero() {
		return settle
	}
	wait := last.Add(s.cfg.Interval).Sub(s.cfg.Now()) + time.Duration(mathrand.Int64N(int64(30*time.Minute)))
	if wait < settle && s.cfg.Now().Sub(s.started) < settle {
		return settle
	}
	return max(wait, time.Minute)
}

func (s *Service) send(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	report, err := s.Collect(ctx)
	if err != nil {
		return err
	}
	body, err := json.Marshal(report)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "shipper/"+s.cfg.Version)
	response, err := s.cfg.Client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("telemetry endpoint returned %d", response.StatusCode)
	}
	return s.cfg.Store.SaveSetting(ctx, lastSentKey, strconv.FormatInt(s.cfg.Now().Unix(), 10))
}

func normalize(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "none"
	}
	if !keyPattern.MatchString(value) {
		return "other"
	}
	return value
}

func increment(values map[string]int, key string) { values[normalize(key)]++ }

// trim folds the long tail into "other" so a report stays small.
func trim(values map[string]int) {
	if len(values) <= maxKeys {
		return
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if values[keys[i]] == values[keys[j]] {
			return keys[i] < keys[j]
		}
		return values[keys[i]] > values[keys[j]]
	})
	other := 0
	for _, key := range keys[maxKeys-1:] {
		other += values[key]
		delete(values, key)
	}
	values["other"] += other
}

func clip(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) > limit {
		return value[:limit]
	}
	return value
}
