package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/souzavinny/reviews-api/internal/domain"
)

// AppStore is the persisted set of monitored apps, keyed by app id and mirrored
// to data/apps.json. The seed is applied only on first run (no file yet); after
// that the file is the source of truth so user add/remove survives restart.
//
// Locking mirrors FileStore: writeMu serializes mutations, mu guards the map
// and is held only to read or commit it, never across the disk write.
type AppStore struct {
	dir     string
	writeMu sync.Mutex
	mu      sync.RWMutex
	apps    map[string]domain.App
}

// NewAppStore loads data/apps.json, or seeds from the given apps and writes the
// file when none exists yet. A corrupt or oversized registry is fatal — it is
// the single source of the monitored-app set, so silently starting empty would
// be worse than failing loudly.
func NewAppStore(dir string, seed []domain.App) (*AppStore, error) {
	s := &AppStore{dir: dir, apps: make(map[string]domain.App)}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	data, err := readJSONFile(filepath.Join(dir, registryFileName))
	switch {
	case err == nil:
		var apps []domain.App
		if err := json.Unmarshal(data, &apps); err != nil {
			return nil, fmt.Errorf("load registry: %w", err)
		}
		for _, a := range apps {
			s.apps[a.ID] = a
		}
		return s, nil
	case errors.Is(err, os.ErrNotExist):
		for _, a := range seed {
			s.apps[a.ID] = a
		}
		if err := s.persist(s.apps); err != nil {
			return nil, err
		}
		return s, nil
	default:
		return nil, err
	}
}

// List returns the monitored apps sorted by id for stable output.
func (s *AppStore) List(ctx context.Context) ([]domain.App, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return sortedApps(s.apps), nil
}

// Add upserts an app by id and persists; re-adding an identical app is a no-op.
func (s *AppStore) Add(ctx context.Context, app domain.App) error {
	if app.ID == "" {
		return fmt.Errorf("app id is required")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	s.mu.RLock()
	if existing, ok := s.apps[app.ID]; ok && existing == app {
		s.mu.RUnlock()
		return nil
	}
	next := make(map[string]domain.App, len(s.apps)+1)
	for k, v := range s.apps {
		next[k] = v
	}
	s.mu.RUnlock()
	next[app.ID] = app

	if err := s.persist(next); err != nil {
		return err
	}
	s.mu.Lock()
	s.apps = next
	s.mu.Unlock()
	return nil
}

// Remove deletes an app by id and persists; removing an absent app is a no-op.
func (s *AppStore) Remove(ctx context.Context, appID string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	s.mu.RLock()
	if _, ok := s.apps[appID]; !ok {
		s.mu.RUnlock()
		return nil
	}
	next := make(map[string]domain.App, len(s.apps))
	for k, v := range s.apps {
		if k != appID {
			next[k] = v
		}
	}
	s.mu.RUnlock()

	if err := s.persist(next); err != nil {
		return err
	}
	s.mu.Lock()
	s.apps = next
	s.mu.Unlock()
	return nil
}

func (s *AppStore) persist(apps map[string]domain.App) error {
	return writeJSON(filepath.Join(s.dir, registryFileName), sortedApps(apps))
}

func sortedApps(apps map[string]domain.App) []domain.App {
	out := make([]domain.App, 0, len(apps))
	for _, a := range apps {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
