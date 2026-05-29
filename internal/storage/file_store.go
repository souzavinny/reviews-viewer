package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/souzavinny/reviews-api/internal/domain"
)

// FileStore keeps reviews per app in memory and mirrors each app to
// data/<appID>.json. The stored id set is the dedupe key, so re-saving an
// already-seen review is a no-op and restart loses nothing.
//
// writeMu serializes writers so concurrent Saves can't lose an update; mu
// guards the in-memory map and is held only to read or commit it, never across
// disk I/O.
type FileStore struct {
	dir     string
	writeMu sync.Mutex
	mu      sync.RWMutex
	reviews map[string][]domain.Review
}

// NewFileStore creates the data directory if needed and loads every
// data/<appID>.json back into memory. The registry file is skipped, and an
// unreadable or corrupt review file is logged and skipped so one bad file
// can't keep the service from starting.
func NewFileStore(dir string) (*FileStore, error) {
	s := &FileStore{dir: dir, reviews: make(map[string][]domain.Review)}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || name == registryFileName || !strings.HasSuffix(name, ".json") {
			continue
		}
		data, err := readJSONFile(filepath.Join(dir, name))
		if err != nil {
			log.Printf("storage: skipping unreadable review file %s: %v", name, err)
			continue
		}
		var reviews []domain.Review
		if err := json.Unmarshal(data, &reviews); err != nil {
			log.Printf("storage: skipping corrupt review file %s: %v", name, err)
			continue
		}
		s.reviews[strings.TrimSuffix(name, ".json")] = reviews
	}
	return s, nil
}

// Save appends only reviews whose id isn't already stored for the app and
// rewrites the app's file when something new was added. The new state is
// committed to memory only after the file write succeeds, so memory and disk
// never diverge.
func (s *FileStore) Save(ctx context.Context, appID string, reviews []domain.Review) error {
	path, err := s.pathFor(appID)
	if err != nil {
		return err
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	s.mu.RLock()
	stored := s.reviews[appID]
	seen := make(map[string]struct{}, len(stored))
	for _, r := range stored {
		seen[r.ID] = struct{}{}
	}
	s.mu.RUnlock()

	next := make([]domain.Review, len(stored), len(stored)+len(reviews))
	copy(next, stored)
	added := false
	for _, r := range reviews {
		if _, ok := seen[r.ID]; ok {
			continue
		}
		seen[r.ID] = struct{}{}
		next = append(next, r)
		added = true
	}
	if !added {
		return nil
	}

	if err := writeJSON(path, next); err != nil {
		return err
	}

	s.mu.Lock()
	s.reviews[appID] = next
	s.mu.Unlock()
	return nil
}

// ListByApp returns a copy of the app's stored reviews; the service applies
// windowing and newest-first ordering.
func (s *FileStore) ListByApp(ctx context.Context, appID string) ([]domain.Review, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stored := s.reviews[appID]
	out := make([]domain.Review, len(stored))
	copy(out, stored)
	return out, nil
}

// pathFor maps an app id to its file, rejecting ids that aren't a safe base
// name. Only the write path needs this: load reads os.ReadDir basenames, which
// are always flat, so traversal can't enter through boot.
func (s *FileStore) pathFor(appID string) (string, error) {
	if appID == "" || appID == "." || appID == ".." || appID != filepath.Base(appID) {
		return "", fmt.Errorf("unsafe app id %q for file storage", appID)
	}
	if appID+".json" == registryFileName {
		return "", fmt.Errorf("app id %q collides with the registry file", appID)
	}
	return filepath.Join(s.dir, appID+".json"), nil
}
