// Package inventory reconciles persistent metadata with complete Docker snapshots.
package inventory

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"

	"shipper/internal/engine"
	"shipper/internal/events"
	"shipper/internal/store"
)

type Health struct {
	Connected  bool        `json:"connected"`
	CheckedAt  int64       `json:"checkedAt"`
	ObservedAt int64       `json:"observedAt"`
	Error      string      `json:"error,omitempty"`
	Engine     engine.Info `json:"engine"`
}
type Monitor struct {
	source  engine.Source
	store   *store.Store
	hub     *events.Hub
	mu      sync.RWMutex
	health  Health
	refresh chan struct{}
}

func New(source engine.Source, db *store.Store, hub *events.Hub) *Monitor {
	return &Monitor{source: source, store: db, hub: hub, refresh: make(chan struct{}, 1), health: Health{Error: "Connecting to Docker"}}
}
func (m *Monitor) Health() Health { m.mu.RLock(); defer m.mu.RUnlock(); return m.health }
func (m *Monitor) Trigger() {
	select {
	case m.refresh <- struct{}{}:
	default:
	}
}

func (m *Monitor) Reconcile(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	snapshot, err := m.source.Snapshot(ctx)
	if err == nil {
		err = m.store.ReplaceInventory(ctx, snapshot.Containers)
	}
	m.mu.Lock()
	m.health.CheckedAt = time.Now().Unix()
	if err != nil {
		m.health.Connected = false
		m.health.Error = "Docker is unavailable"
	} else {
		m.health.Connected = true
		m.health.Error = ""
		m.health.ObservedAt = time.Now().Unix()
		m.health.Engine = snapshot.Info
	}
	m.mu.Unlock()
	m.hub.Publish()
	return err
}

func (m *Monitor) streamLost() {
	m.mu.Lock()
	m.health.Connected = false
	m.health.Error = "Reconnecting to Docker"
	m.mu.Unlock()
	m.hub.Publish()
}

func (m *Monitor) Run(ctx context.Context) {
	delay := time.Second
	for ctx.Err() == nil {
		streamCtx, cancel := context.WithCancel(ctx)
		changes, errs := m.source.Subscribe(streamCtx)
		err := m.Reconcile(ctx)
		if err == nil {
			delay = time.Second
		}
		periodic := time.NewTicker(30 * time.Second)
		var debounce *time.Timer
		var pending <-chan time.Time
		alive := true
		for alive {
			select {
			case <-ctx.Done():
				alive = false
			case _, ok := <-changes:
				if !ok {
					alive = false
					break
				}
				// A fixed window coalesces storms without starving reconciliation.
				if pending == nil {
					debounce = time.NewTimer(150 * time.Millisecond)
					pending = debounce.C
				}
			case <-pending:
				pending = nil
				if err := m.Reconcile(ctx); err != nil {
					slog.Debug("Docker reconciliation failed", "error", err)
				}
			case <-m.refresh:
				if err := m.Reconcile(ctx); err != nil {
					slog.Debug("Docker reconciliation failed", "error", err)
				}
			case <-periodic.C:
				if err := m.Reconcile(ctx); err != nil {
					slog.Debug("Docker reconciliation failed", "error", err)
				}
			case err, ok := <-errs:
				if ok && err != nil && ctx.Err() == nil {
					slog.Debug("Docker event subscription ended", "error", err)
				}
				alive = false
			}
		}
		if debounce != nil {
			debounce.Stop()
		}
		periodic.Stop()
		cancel()
		if ctx.Err() != nil {
			return
		}
		m.streamLost()
		timer := time.NewTimer(delay + time.Duration(rand.Int64N(int64(delay/4))))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-m.refresh:
			timer.Stop()
		case <-timer.C:
		}
		delay = min(delay*2, 15*time.Second)
	}
}
