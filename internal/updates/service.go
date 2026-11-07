// Package updates discovers release feeds and requests host-side control-plane updates.
package updates

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"shipper/internal/store"
)

type Channel string

const (
	Main    Channel = "main"
	Nightly Channel = "nightly"

	DefaultMainFeed    = "https://raw.githubusercontent.com/matyas-toth/shipperkit/latest"
	DefaultNightlyFeed = "https://raw.githubusercontent.com/matyas-toth/shipperkit/nightly"

	settingsKey = "update_settings"
)

var (
	versionPattern    = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$`)
	imagePattern      = regexp.MustCompile(`^ghcr\.io/matyas-toth/shipperkit@sha256:[0-9a-f]{64}$`)
	releaseURLPattern = regexp.MustCompile(`^https://github\.com/matyas-toth/shipperkit/(releases/tag|commit)/[A-Za-z0-9._/-]+$`)

	ErrUnavailable = errors.New("Updates are unavailable in this installation")
	ErrNoUpdate    = errors.New("No update is available")
	ErrInProgress  = errors.New("An update is already in progress")
)

type Available struct {
	Version    string  `json:"version"`
	Image      string  `json:"image"`
	ReleaseURL string  `json:"releaseUrl"`
	Channel    Channel `json:"channel"`
	CheckedAt  int64   `json:"checkedAt"`
}

type Status struct {
	State       string `json:"state"`
	Message     string `json:"message"`
	FromVersion string `json:"fromVersion"`
	ToVersion   string `json:"toVersion"`
	StartedAt   int64  `json:"startedAt"`
	FinishedAt  int64  `json:"finishedAt"`
	RequestID   string `json:"requestId"`
	Log         string `json:"log"`
}

func (s Status) Active() bool {
	switch s.State {
	case "queued", "preparing", "pulling", "checkpoint", "updating", "starting", "verifying", "restarting":
		return true
	default:
		return false
	}
}

func (s Status) Terminal() bool {
	return s.State == "succeeded" || s.State == "failed"
}

type State struct {
	Enabled        bool       `json:"enabled"`
	CurrentVersion string     `json:"currentVersion"`
	Channel        Channel    `json:"channel"`
	Auto           bool       `json:"auto"`
	Available      *Available `json:"available"`
	CheckedAt      int64      `json:"checkedAt"`
	Checking       bool       `json:"checking"`
	Error          string     `json:"error,omitempty"`
	Status         Status     `json:"status"`
}

type persisted struct {
	Channel         Channel    `json:"channel"`
	Auto            bool       `json:"auto"`
	Available       *Available `json:"available,omitempty"`
	CheckedAt       int64      `json:"checkedAt,omitempty"`
	Error           string     `json:"error,omitempty"`
	LastAutoVersion string     `json:"lastAutoVersion,omitempty"`
}

type Config struct {
	Store          *store.Store
	DataDir        string
	CurrentVersion string
	MainFeed       string
	NightlyFeed    string
	Interval       time.Duration
	Enabled        bool
	Ready          func() bool
	Client         *http.Client
	Now            func() time.Time
}

type Service struct {
	mu             sync.Mutex
	db             *store.Store
	dataDir        string
	currentVersion string
	feeds          map[Channel]string
	interval       time.Duration
	enabled        bool
	ready          func() bool
	client         *http.Client
	now            func() time.Time
	state          persisted
	checking       bool
}

func New(cfg Config) *Service {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: 10 * time.Second}
	}
	if cfg.MainFeed == "" {
		cfg.MainFeed = DefaultMainFeed
	}
	if cfg.NightlyFeed == "" {
		cfg.NightlyFeed = DefaultNightlyFeed
	}
	if cfg.Interval == 0 {
		cfg.Interval = 5 * time.Minute
	}
	s := &Service{
		db:             cfg.Store,
		dataDir:        cfg.DataDir,
		currentVersion: strings.TrimSpace(cfg.CurrentVersion),
		feeds:          map[Channel]string{Main: cfg.MainFeed, Nightly: cfg.NightlyFeed},
		interval:       cfg.Interval,
		enabled:        cfg.Enabled,
		ready:          cfg.Ready,
		client:         cfg.Client,
		now:            cfg.Now,
		state:          persisted{Channel: Main},
	}
	s.load()
	return s
}

func (s *Service) Enabled() bool {
	if s == nil || !s.enabled || s.db == nil || s.dataDir == "" {
		return false
	}
	if s.currentVersion == "" {
		return false
	}
	if s.ready != nil && !s.ready() {
		return false
	}
	return true
}

func (s *Service) load() {
	if s.db == nil {
		return
	}
	raw, err := s.db.Setting(context.Background(), settingsKey, "")
	if err != nil || raw == "" {
		return
	}
	var value persisted
	if err = json.Unmarshal([]byte(raw), &value); err != nil {
		slog.Warn("Ignoring invalid update settings", "error", err)
		return
	}
	if !value.Channel.Valid() {
		value.Channel = Main
	}
	s.state = value
}

func (c Channel) Valid() bool {
	return c == Main || c == Nightly
}

func (s *Service) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotLocked()
}

func (s *Service) snapshotLocked() State {
	state := State{
		Enabled:        s.Enabled(),
		CurrentVersion: s.currentVersion,
		Channel:        s.state.Channel,
		Auto:           s.state.Auto,
		CheckedAt:      s.state.CheckedAt,
		Checking:       s.checking,
		Error:          s.state.Error,
		Status:         s.readStatus(),
	}
	if s.state.Available != nil && Newer(s.state.Available.Version, s.currentVersion) {
		available := *s.state.Available
		state.Available = &available
	}
	return state
}

func (s *Service) SetSettings(ctx context.Context, channel Channel, auto bool) (State, error) {
	if !channel.Valid() {
		return State{}, errors.New("Choose a valid update channel")
	}
	s.mu.Lock()
	changed := s.state.Channel != channel
	s.state.Channel = channel
	s.state.Auto = auto
	if changed {
		s.state.Available = nil
		s.state.CheckedAt = 0
		s.state.Error = ""
		s.state.LastAutoVersion = ""
	}
	state := s.snapshotLocked()
	s.mu.Unlock()
	if err := s.persist(ctx); err != nil {
		return state, err
	}
	return state, nil
}

func (s *Service) Check(ctx context.Context) (State, error) {
	s.mu.Lock()
	if s.checking {
		state := s.snapshotLocked()
		s.mu.Unlock()
		return state, nil
	}
	if !s.Enabled() {
		state := s.snapshotLocked()
		s.mu.Unlock()
		return state, ErrUnavailable
	}
	s.checking = true
	channel := s.state.Channel
	base := strings.TrimRight(s.feeds[channel], "/")
	current := s.currentVersion
	s.mu.Unlock()

	available, err := s.fetch(ctx, channel, base)

	s.mu.Lock()
	s.checking = false
	if s.state.Channel != channel {
		state := s.snapshotLocked()
		s.mu.Unlock()
		return state, nil
	}
	s.state.CheckedAt = s.now().Unix()
	if err != nil {
		s.state.Error = err.Error()
	} else {
		s.state.Error = ""
		if available != nil && Newer(available.Version, current) {
			available.CheckedAt = s.state.CheckedAt
			s.state.Available = available
		} else {
			s.state.Available = nil
		}
	}
	state := s.snapshotLocked()
	s.mu.Unlock()
	if persistErr := s.persist(ctx); persistErr != nil {
		slog.Warn("Could not persist update settings", "error", persistErr)
	}
	return state, err
}

func (s *Service) fetch(ctx context.Context, channel Channel, base string) (*Available, error) {
	version, err := s.text(ctx, base+"/version.txt")
	if err != nil {
		return nil, fmt.Errorf("release version is unavailable: %w", err)
	}
	if !versionPattern.MatchString(version) {
		return nil, errors.New("release version is invalid")
	}
	image, err := s.text(ctx, base+"/image.txt")
	if err != nil {
		return nil, fmt.Errorf("release image is unavailable: %w", err)
	}
	if !imagePattern.MatchString(image) {
		return nil, errors.New("release image is invalid")
	}
	releaseURL, err := s.text(ctx, base+"/release-url.txt")
	if err != nil {
		return nil, fmt.Errorf("release notes are unavailable: %w", err)
	}
	if !releaseURLPattern.MatchString(releaseURL) {
		return nil, errors.New("release notes link is invalid")
	}
	return &Available{Version: version, Image: image, ReleaseURL: releaseURL, Channel: channel}, nil
}

func (s *Service) text(ctx context.Context, url string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	request.Header.Set("User-Agent", "shipper-updater/1")
	request.Header.Set("Cache-Control", "no-cache")
	response, err := s.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 4096))
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(raw))
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return "", errors.New("release feed is malformed")
	}
	return value, nil
}

func (s *Service) MaybeAutoApply(ctx context.Context) error {
	s.mu.Lock()
	if !s.Enabled() || !s.state.Auto || s.state.Available == nil || !Newer(s.state.Available.Version, s.currentVersion) {
		s.mu.Unlock()
		return nil
	}
	if s.state.LastAutoVersion == s.state.Available.Version {
		s.mu.Unlock()
		return nil
	}
	status := s.readStatus()
	if status.Active() {
		s.mu.Unlock()
		return nil
	}
	available := *s.state.Available
	s.mu.Unlock()
	if err := s.db.UpdateReady(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	if s.state.LastAutoVersion == available.Version {
		s.mu.Unlock()
		return nil
	}
	s.state.LastAutoVersion = available.Version
	s.mu.Unlock()
	if err := s.persist(ctx); err != nil {
		return err
	}
	if err := s.applyAvailable(ctx, available); err != nil {
		s.mu.Lock()
		if s.state.LastAutoVersion == available.Version {
			s.state.LastAutoVersion = ""
		}
		s.mu.Unlock()
		_ = s.persist(ctx)
		return err
	}
	return nil
}

func (s *Service) Apply(ctx context.Context) (Status, error) {
	s.mu.Lock()
	if !s.Enabled() {
		s.mu.Unlock()
		return Status{}, ErrUnavailable
	}
	if s.state.Available == nil || !Newer(s.state.Available.Version, s.currentVersion) {
		s.mu.Unlock()
		return Status{}, ErrNoUpdate
	}
	if s.readStatus().Active() {
		s.mu.Unlock()
		return Status{}, ErrInProgress
	}
	available := *s.state.Available
	s.mu.Unlock()
	if err := s.db.UpdateReady(ctx); err != nil {
		return Status{}, err
	}
	if err := s.applyAvailable(ctx, available); err != nil {
		return Status{}, err
	}
	return s.readStatus(), nil
}

func (s *Service) applyAvailable(ctx context.Context, available Available) error {
	if !imagePattern.MatchString(available.Image) || !versionPattern.MatchString(available.Version) {
		return errors.New("The update metadata is invalid")
	}
	requestID, err := randomID()
	if err != nil {
		return err
	}
	request := struct {
		RequestID   string  `json:"requestId"`
		Version     string  `json:"version"`
		Image       string  `json:"image"`
		ReleaseURL  string  `json:"releaseUrl"`
		Channel     Channel `json:"channel"`
		RequestedAt int64   `json:"requestedAt"`
		FromVersion string  `json:"fromVersion"`
	}{
		RequestID: requestID, Version: available.Version, Image: available.Image,
		ReleaseURL: available.ReleaseURL, Channel: available.Channel,
		RequestedAt: s.now().Unix(), FromVersion: s.currentVersion,
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return err
	}
	next, err := os.CreateTemp(s.dataDir, ".update-request-*")
	if err != nil {
		return fmt.Errorf("could not request the update: %w", err)
	}
	path := next.Name()
	defer os.Remove(path)
	if err = next.Chmod(0600); err == nil {
		_, err = next.Write(raw)
	}
	if closeErr := next.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("could not request the update: %w", err)
	}
	if err = os.Rename(path, filepath.Join(s.dataDir, "update-request.json")); err != nil {
		return fmt.Errorf("could not request the update: %w", err)
	}
	slog.Info("Requested a Shipper update", "version", available.Version, "channel", available.Channel)
	return nil
}

func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.readStatus()
}

func (s *Service) readStatus() Status {
	var status Status
	statusPath := filepath.Join(s.dataDir, "update-status.json")
	var statusMod time.Time
	if info, err := os.Stat(statusPath); err == nil {
		statusMod = info.ModTime()
	}
	if file, err := os.Open(statusPath); err == nil {
		decoder := json.NewDecoder(io.LimitReader(file, 16<<10))
		err = decoder.Decode(&status)
		file.Close()
		if err != nil {
			status = Status{}
		}
	}
	var request struct {
		RequestID   string `json:"requestId"`
		Version     string `json:"version"`
		FromVersion string `json:"fromVersion"`
	}
	if file, err := os.Open(filepath.Join(s.dataDir, "update-request.json")); err == nil {
		decoder := json.NewDecoder(io.LimitReader(file, 4096))
		err = decoder.Decode(&request)
		file.Close()
		if err != nil {
			request = struct {
				RequestID   string `json:"requestId"`
				Version     string `json:"version"`
				FromVersion string `json:"fromVersion"`
			}{}
		}
	}
	if request.RequestID != "" && request.Version != "" && request.RequestID != status.RequestID {
		return Status{State: "queued", Message: "Waiting for the update helper", FromVersion: request.FromVersion, ToVersion: request.Version, RequestID: request.RequestID}
	}
	if status.Active() && !statusMod.IsZero() && time.Since(statusMod) > 30*time.Minute {
		status = Status{
			State: "failed", Message: "The update did not finish. Check the host update log.",
			FromVersion: status.FromVersion, ToVersion: status.ToVersion,
			StartedAt: status.StartedAt, FinishedAt: time.Now().Unix(), RequestID: status.RequestID, Log: status.Log,
		}
	}
	if status.State == "" {
		status.State = "idle"
	}
	if len(status.Log) > 8<<10 {
		status.Log = status.Log[len(status.Log)-(8<<10):]
	}
	return status
}

func (s *Service) persist(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	s.mu.Lock()
	raw, err := json.Marshal(s.state)
	s.mu.Unlock()
	if err != nil {
		return err
	}
	return s.db.SaveSetting(ctx, settingsKey, string(raw))
}

func (s *Service) Run(ctx context.Context) {
	if !s.enabled || s.interval <= 0 {
		return
	}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if !s.Enabled() {
				timer.Reset(30 * time.Second)
				continue
			}
			if _, err := s.Check(ctx); err != nil {
				slog.Debug("Shipper update check failed", "error", err)
			}
			if err := s.MaybeAutoApply(ctx); err != nil {
				slog.Warn("Could not start automatic Shipper update", "error", err)
			}
			timer.Reset(s.interval)
		}
	}
}

func randomID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}
