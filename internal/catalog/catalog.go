// Package catalog owns the curated service library: the embedded baseline,
// the optional remote manifest used for catalog updates, and the local cache.
package catalog

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

//go:embed catalog.json
var embedded embed.FS

// DefaultRemote is the published manifest for released instances. It can be
// replaced with SHIPPER_CATALOG_URL or disabled with SHIPPER_CATALOG_URL=off.
const DefaultRemote = "https://raw.githubusercontent.com/matyas-toth/shipperkit/master/internal/catalog/catalog.json"

const maxManifest = 8 << 20

type Manifest struct {
	Version   string    `json:"version"`
	Source    string    `json:"source"`
	Generated string    `json:"generated"`
	Services  []Service `json:"services"`
}

type Service struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Description   string        `json:"description"`
	Category      string        `json:"category"`
	Tags          []string      `json:"tags,omitempty"`
	Logomark      string        `json:"logomark"`
	Website       string        `json:"website,omitempty"`
	Documentation string        `json:"documentation,omitempty"`
	Source        string        `json:"source"`
	Updated       string        `json:"updated,omitempty"`
	Featured      bool          `json:"featured,omitempty"`
	MainService   string        `json:"mainService"`
	Port          int           `json:"port"`
	HealthPath    string        `json:"healthPath"`
	Support       string        `json:"support"`
	SupportNote   string        `json:"supportNote,omitempty"`
	Compose       string        `json:"compose"`
	Services      []ServiceInfo `json:"services"`
	Fields        []Field       `json:"fields"`
	Domains       []Domain      `json:"domains,omitempty"`
}

type ServiceInfo struct {
	Name  string `json:"name"`
	Image string `json:"image"`
	Role  string `json:"role"`
	Port  int    `json:"port,omitempty"`
}

type Domain struct {
	Key     string `json:"key,omitempty"`
	Service string `json:"service"`
	Port    int    `json:"port"`
	Primary bool   `json:"primary,omitempty"`
}

type Field struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Kind        string   `json:"kind"`
	Group       string   `json:"group"`
	Default     string   `json:"default,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Help        string   `json:"help,omitempty"`
	Options     []string `json:"options,omitempty"`
	Service     string   `json:"service,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Secret      bool     `json:"secret,omitempty"`
	Generate    string   `json:"generate,omitempty"`
	Advanced    bool     `json:"advanced,omitempty"`
}

// Summary is the payload for the library grid. Compose definitions stay on the
// detail endpoint so a large catalog does not slow down the first paint.
type Summary struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Category      string   `json:"category"`
	Tags          []string `json:"tags,omitempty"`
	Logomark      string   `json:"logomark"`
	Website       string   `json:"website,omitempty"`
	Documentation string   `json:"documentation,omitempty"`
	Source        string   `json:"source"`
	Featured      bool     `json:"featured,omitempty"`
	MainService   string   `json:"mainService"`
	Port          int      `json:"port"`
	Support       string   `json:"support"`
	SupportNote   string   `json:"supportNote,omitempty"`
	Services      int      `json:"services"`
	Fields        int      `json:"fields"`
	Domain        bool     `json:"domain"`
}

type Category struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type Index struct {
	Version    string     `json:"version"`
	Source     string     `json:"source"`
	Generated  string     `json:"generated"`
	UpdatedAt  int64      `json:"updatedAt"`
	FromCache  bool       `json:"fromCache,omitempty"`
	Categories []Category `json:"categories"`
	Items      []Summary  `json:"items"`
}

type Catalog struct {
	Manifest
	byID map[string]*Service
}

func Parse(data []byte) (*Catalog, error) {
	if len(data) == 0 || len(data) > maxManifest {
		return nil, errors.New("catalog manifest has an invalid size")
	}
	var manifest Manifest
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("catalog manifest is invalid: %w", err)
	}
	if len(manifest.Services) == 0 {
		return nil, errors.New("catalog manifest has no services")
	}
	seen := map[string]bool{}
	for i := range manifest.Services {
		service := &manifest.Services[i]
		service.ID = strings.TrimSpace(service.ID)
		if service.ID == "" || seen[service.ID] || len(service.ID) > 80 || strings.ContainsAny(service.ID, "/\\\x00\r\n") {
			return nil, errors.New("catalog manifest has an invalid service id")
		}
		seen[service.ID] = true
		if service.Name == "" || service.Compose == "" || service.MainService == "" {
			return nil, fmt.Errorf("catalog service %s is incomplete", service.ID)
		}
		if len(service.Compose) > 128<<10 {
			return nil, fmt.Errorf("catalog service %s exceeds the Compose limit", service.ID)
		}
	}
	catalog := &Catalog{Manifest: manifest, byID: make(map[string]*Service, len(manifest.Services))}
	for i := range catalog.Services {
		catalog.byID[catalog.Services[i].ID] = &catalog.Services[i]
	}
	return catalog, nil
}

func Embedded() *Catalog {
	data, err := embedded.ReadFile("catalog.json")
	if err != nil {
		panic("catalog manifest is missing: " + err.Error())
	}
	catalog, err := Parse(data)
	if err != nil {
		panic("catalog manifest is invalid: " + err.Error())
	}
	return catalog
}

func (c *Catalog) Get(id string) (*Service, bool) {
	service, ok := c.byID[id]
	return service, ok
}

func (c *Catalog) Index(updatedAt int64, cached bool) Index {
	index := Index{
		Version: c.Version, Source: c.Source, Generated: c.Generated, UpdatedAt: updatedAt, FromCache: cached,
		Items: make([]Summary, 0, len(c.Services)),
	}
	counts := map[string]int{}
	featured := 0
	for _, service := range c.Services {
		counts[service.Category]++
		if service.Featured {
			featured++
		}
		index.Items = append(index.Items, Summary{
			ID: service.ID, Name: service.Name, Description: service.Description, Category: service.Category,
			Tags: service.Tags, Logomark: service.Logomark, Website: service.Website,
			Documentation: service.Documentation, Source: service.Source, Featured: service.Featured,
			MainService: service.MainService,
			Port:        service.Port, Support: service.Support, SupportNote: service.SupportNote,
			Services: len(service.Services), Fields: len(service.Fields), Domain: len(service.Domains) > 0,
		})
	}
	if featured > 0 {
		index.Categories = append(index.Categories, Category{Name: "Featured", Count: featured})
	}
	for name, count := range counts {
		index.Categories = append(index.Categories, Category{Name: name, Count: count})
	}
	sort.Slice(index.Categories, func(i, j int) bool {
		if index.Categories[i].Name == "Featured" || index.Categories[j].Name == "Featured" {
			return index.Categories[i].Name == "Featured"
		}
		if index.Categories[i].Count != index.Categories[j].Count {
			return index.Categories[i].Count > index.Categories[j].Count
		}
		return index.Categories[i].Name < index.Categories[j].Name
	})
	return index
}

// Manager serves the in-memory catalog and refreshes it from the published
// manifest. Embedded data is always available, so a failed refresh never makes
// the Service Library disappear.
type Manager struct {
	dataDir string
	remote  string
	client  *http.Client
	current atomic.Pointer[Catalog]
	state   atomic.Pointer[catalogState]
}

type catalogState struct {
	updatedAt int64
	cached    bool
	lastError string
	etag      string
	checkedAt int64
}

const RefreshInterval = time.Hour

func NewManager(dataDir string) *Manager {
	manager := &Manager{
		dataDir: dataDir,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
	manager.current.Store(Embedded())
	manager.state.Store(&catalogState{})
	if dataDir != "" {
		if cached, mod, err := readCached(dataDir); err == nil {
			manager.current.Store(cached)
			manager.state.Store(&catalogState{updatedAt: mod.ModTime().Unix(), cached: true})
		}
	}
	return manager
}

func (m *Manager) Current() *Catalog { return m.current.Load() }

func (m *Manager) Index() Index {
	catalog := m.Current()
	state := m.state.Load()
	return catalog.Index(state.updatedAt, state.cached)
}

func (m *Manager) State() (int64, bool, string) {
	state := m.state.Load()
	return state.checkedAt, state.cached, state.lastError
}

func cachePath(dataDir string) string { return filepath.Join(dataDir, "service-catalog.json") }

func readCached(dataDir string) (*Catalog, os.FileInfo, error) {
	path := cachePath(dataDir)
	info, err := os.Stat(path)
	if err != nil || info.Size() > maxManifest {
		return nil, nil, errors.New("no cached catalog")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	catalog, err := Parse(data)
	if err != nil {
		return nil, nil, err
	}
	return catalog, info, nil
}

// Refresh fetches the remote manifest. The embedded manifest always remains
// available if the network, status code, or payload is invalid.
func (m *Manager) Refresh(ctx context.Context, remote string) error {
	if remote == "" {
		remote = m.remote
	}
	if remote == "" {
		remote = DefaultRemote
	}
	state := m.state.Load()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, remote, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "shipper-service-catalog/1")
	if state.etag != "" {
		request.Header.Set("If-None-Match", state.etag)
	}
	response, err := m.client.Do(request)
	if err != nil {
		m.recordError(err)
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotModified {
		m.recordSuccess(state.etag, false)
		return nil
	}
	if response.StatusCode != http.StatusOK {
		err = fmt.Errorf("catalog refresh returned HTTP %d", response.StatusCode)
		m.recordError(err)
		return err
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxManifest+1))
	if err != nil {
		m.recordError(err)
		return err
	}
	catalog, err := Parse(data)
	if err != nil {
		m.recordError(err)
		return err
	}
	m.current.Store(catalog)
	m.remote = remote
	etag := response.Header.Get("ETag")
	if m.dataDir != "" {
		if err := writeCached(m.dataDir, data); err != nil {
			slog.Warn("Service catalog cache could not be saved", "error", err)
		}
	}
	m.recordSuccess(etag, true)
	return nil
}

func (m *Manager) recordSuccess(etag string, changed bool) {
	previous := m.state.Load()
	state := &catalogState{etag: strings.TrimSpace(etag), checkedAt: time.Now().Unix(), updatedAt: previous.updatedAt, cached: previous.cached}
	if changed {
		state.updatedAt = time.Now().Unix()
		state.cached = true
	}
	if state.etag == "" {
		state.etag = previous.etag
	}
	m.state.Store(state)
}

func (m *Manager) recordError(err error) {
	previous := m.state.Load()
	m.state.Store(&catalogState{etag: previous.etag, checkedAt: time.Now().Unix(), updatedAt: previous.updatedAt, cached: previous.cached, lastError: err.Error()})
	slog.Warn("Service catalog refresh failed", "error", err)
}

func writeCached(dataDir string, data []byte) error {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(dataDir, "service-catalog-*.tmp")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if _, err = temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err = temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err = temporary.Close(); err != nil {
		return err
	}
	return os.Rename(name, cachePath(dataDir))
}

// Start refreshes on an interval. The first refresh is delayed so instance
// startup and the first page load are never blocked by catalog networking.
func (m *Manager) Start(ctx context.Context, remote string, interval time.Duration) {
	if remote == "off" || remote == "disabled" {
		return
	}
	if interval <= 0 {
		interval = RefreshInterval
	}
	go func() {
		timer := time.NewTimer(30 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				refreshCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
				_ = m.Refresh(refreshCtx, remote)
				cancel()
				timer.Reset(interval)
			}
		}
	}()
}
