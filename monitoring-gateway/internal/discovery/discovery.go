package discovery

import (
	"context"
	"log/slog"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// Snapshot holds currently resolved upstream endpoints.
type Snapshot struct {
	APIURL      *url.URL
	AnalyzerURL *url.URL
	ResolvedAt  time.Time
}

// Resolver discovers current upstream endpoints.
type Resolver interface {
	Resolve(ctx context.Context) (Snapshot, error)
}

// Manager periodically refreshes and stores discovery snapshots.
type Manager struct {
	resolver        Resolver
	refreshInterval time.Duration

	snapshot atomic.Pointer[Snapshot]
	ready    atomic.Bool

	lastErrMu sync.RWMutex
	lastErr   error
}

func NewManager(resolver Resolver, refreshInterval time.Duration) *Manager {
	return &Manager{
		resolver:        resolver,
		refreshInterval: refreshInterval,
	}
}

func (m *Manager) Refresh(ctx context.Context) error {
	snapshot, err := m.resolver.Resolve(ctx)
	if err != nil {
		m.ready.Store(false)
		m.setLastErr(err)
		return err
	}
	snapshot.ResolvedAt = time.Now().UTC()
	m.snapshot.Store(&snapshot)
	m.ready.Store(true)
	m.setLastErr(nil)
	return nil
}

func (m *Manager) Start(ctx context.Context, logger *slog.Logger) {
	ticker := time.NewTicker(m.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refreshCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := m.Refresh(refreshCtx)
			cancel()
			if err != nil {
				logger.Error("discovery refresh failed", "error", err)
				continue
			}
			logger.Debug("discovery snapshot refreshed")
		}
	}
}

func (m *Manager) Snapshot() (Snapshot, bool) {
	current := m.snapshot.Load()
	if current == nil {
		return Snapshot{}, false
	}
	return *current, m.ready.Load()
}

func (m *Manager) Ready() bool {
	return m.ready.Load()
}

func (m *Manager) LastError() error {
	m.lastErrMu.RLock()
	defer m.lastErrMu.RUnlock()
	return m.lastErr
}

func (m *Manager) setLastErr(err error) {
	m.lastErrMu.Lock()
	defer m.lastErrMu.Unlock()
	m.lastErr = err
}
