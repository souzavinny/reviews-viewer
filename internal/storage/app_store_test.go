package storage_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/souzavinny/reviews-api/internal/domain"
	"github.com/souzavinny/reviews-api/internal/storage"
)

func TestAppStoreSeedOnFirstRun(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	s, err := storage.NewAppStore(dir, []domain.App{{ID: "1", Name: "One"}, {ID: "2"}})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := s.List(ctx)
	if want := []string{"1", "2"}; !slices.Equal(appIDs(got), want) {
		t.Fatalf("ids = %v, want %v", appIDs(got), want)
	}
	if _, err := os.Stat(filepath.Join(dir, "apps.json")); err != nil {
		t.Fatalf("expected data/apps.json to exist: %v", err)
	}
}

func TestAppStoreNoReseedOnSecondRun(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	s1, err := storage.NewAppStore(dir, []domain.App{{ID: "1"}, {ID: "2"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s1.Remove(ctx, "1"); err != nil {
		t.Fatal(err)
	}

	s2, err := storage.NewAppStore(dir, []domain.App{{ID: "9"}}) // different seed must be ignored
	if err != nil {
		t.Fatal(err)
	}
	got, _ := s2.List(ctx)
	if want := []string{"2"}; !slices.Equal(appIDs(got), want) {
		t.Fatalf("after restart ids = %v, want %v (seed must not re-apply)", appIDs(got), want)
	}
}

func TestAppStoreAddRemovePersist(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	s1, err := storage.NewAppStore(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s1.Add(ctx, domain.App{ID: "3", Name: "Three"}); err != nil {
		t.Fatal(err)
	}

	s2, err := storage.NewAppStore(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := s2.List(ctx)
	if want := []string{"3"}; !slices.Equal(appIDs(got), want) {
		t.Fatalf("after add+restart ids = %v, want %v", appIDs(got), want)
	}
	if got[0].Name != "Three" {
		t.Fatalf("name = %q, want Three", got[0].Name)
	}

	if err := s2.Remove(ctx, "3"); err != nil {
		t.Fatal(err)
	}
	s3, err := storage.NewAppStore(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := s3.List(ctx); len(got) != 0 {
		t.Fatalf("after remove+restart got %d apps, want 0", len(got))
	}
}

func TestAppStoreAddIdempotent(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	s, err := storage.NewAppStore(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	app := domain.App{ID: "5", Name: "Five"}
	if err := s.Add(ctx, app); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(ctx, app); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.List(ctx); len(got) != 1 {
		t.Fatalf("got %d apps, want 1", len(got))
	}
}

func TestAppStoreListSorted(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	s, err := storage.NewAppStore(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"3", "1", "2"} {
		if err := s.Add(ctx, domain.App{ID: id}); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := s.List(ctx)
	if want := []string{"1", "2", "3"}; !slices.Equal(appIDs(got), want) {
		t.Fatalf("ids = %v, want sorted %v", appIDs(got), want)
	}
}

func TestAppStoreFailsOnCorruptRegistry(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "apps.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.NewAppStore(dir, nil); err == nil {
		t.Fatal("want error for a corrupt registry, got nil")
	}
}

func appIDs(apps []domain.App) []string {
	out := make([]string, len(apps))
	for i, a := range apps {
		out[i] = a.ID
	}
	return out
}
