package storage_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/souzavinny/reviews-api/internal/domain"
	"github.com/souzavinny/reviews-api/internal/service"
	"github.com/souzavinny/reviews-api/internal/storage"
)

var (
	_ service.ReviewStore = (*storage.FileStore)(nil)
	_ service.AppRegistry = (*storage.AppStore)(nil)
)

func TestFileStoreSaveAndList(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	s := newStore(t, dir)

	if err := s.Save(ctx, "1", []domain.Review{review("a"), review("b")}); err != nil {
		t.Fatal(err)
	}
	got, err := s.ListByApp(ctx, "1")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a", "b"}; !slices.Equal(idsOf(got), want) {
		t.Fatalf("ids = %v, want %v", idsOf(got), want)
	}
	if _, err := os.Stat(filepath.Join(dir, "1.json")); err != nil {
		t.Fatalf("expected data/1.json to exist: %v", err)
	}
}

func TestFileStoreResume(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	s1 := newStore(t, dir)
	if err := s1.Save(ctx, "1", []domain.Review{review("a"), review("b"), review("c")}); err != nil {
		t.Fatal(err)
	}

	s2 := newStore(t, dir) // rebuilt from the same directory
	got, err := s2.ListByApp(ctx, "1")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a", "b", "c"}; !slices.Equal(idsOf(got), want) {
		t.Fatalf("after resume ids = %v, want %v (no loss)", idsOf(got), want)
	}

	var a domain.Review
	for _, r := range got {
		if r.ID == "a" {
			a = r
		}
	}
	want := review("a")
	if a.Author != want.Author || a.Content != want.Content || a.Score != want.Score || !a.SubmittedAt.Equal(want.SubmittedAt) {
		t.Fatalf("review fields not faithfully restored: %+v", a)
	}
}

func TestFileStoreDedup(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	s := newStore(t, dir)

	if err := s.Save(ctx, "1", []domain.Review{review("a"), review("b")}); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(ctx, "1", []domain.Review{review("b"), review("c")}); err != nil { // b overlaps
		t.Fatal(err)
	}
	if err := s.Save(ctx, "1", []domain.Review{review("d"), review("d")}); err != nil { // dup within batch
		t.Fatal(err)
	}

	want := []string{"a", "b", "c", "d"}
	got, _ := s.ListByApp(ctx, "1")
	if !slices.Equal(idsOf(got), want) {
		t.Fatalf("dedup ids = %v, want %v", idsOf(got), want)
	}

	got2, _ := newStore(t, dir).ListByApp(ctx, "1")
	if !slices.Equal(idsOf(got2), want) {
		t.Fatalf("after resume dedup ids = %v, want %v", idsOf(got2), want)
	}
}

func TestFileStoreListReturnsCopy(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	s := newStore(t, dir)
	if err := s.Save(ctx, "1", []domain.Review{review("a")}); err != nil {
		t.Fatal(err)
	}

	got, _ := s.ListByApp(ctx, "1")
	got[0].ID = "mutated"

	again, _ := s.ListByApp(ctx, "1")
	if again[0].ID != "a" {
		t.Fatalf("mutating the returned slice corrupted the store: %q", again[0].ID)
	}
}

func TestFileStoreRejectsUnsafeAppID(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	s := newStore(t, dir)

	for _, bad := range []string{"", ".", "..", "../evil", "a/b", "apps"} {
		if err := s.Save(ctx, bad, []domain.Review{review("x")}); err == nil {
			t.Errorf("Save(appID=%q) = nil, want error", bad)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "..", "evil.json")); !os.IsNotExist(err) {
		t.Fatalf("a review file escaped the data directory")
	}
}

func TestFileStoreSkipsRegistryFileOnLoad(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	if err := os.WriteFile(filepath.Join(dir, "apps.json"), []byte(`[{"id":"1","name":"One"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := newStore(t, dir).Save(ctx, "1", []domain.Review{review("a")}); err != nil {
		t.Fatal(err)
	}

	s := newStore(t, dir)
	if asReviews, _ := s.ListByApp(ctx, "apps"); len(asReviews) != 0 {
		t.Fatalf("registry file was loaded as reviews: %d entries", len(asReviews))
	}
	if got, _ := s.ListByApp(ctx, "1"); !slices.Equal(idsOf(got), []string{"a"}) {
		t.Fatalf("reviews for app 1 not loaded: %v", idsOf(got))
	}
}

func TestFileStoreSkipsCorruptReviewFile(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	if err := os.WriteFile(filepath.Join(dir, "1.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := newStore(t, dir).Save(ctx, "2", []domain.Review{review("x")}); err != nil {
		t.Fatal(err)
	}

	s := newStore(t, dir) // must boot despite the corrupt file
	if got, _ := s.ListByApp(ctx, "1"); len(got) != 0 {
		t.Fatalf("corrupt app 1 should be skipped, got %d reviews", len(got))
	}
	if got, _ := s.ListByApp(ctx, "2"); !slices.Equal(idsOf(got), []string{"x"}) {
		t.Fatalf("valid app 2 not loaded: %v", idsOf(got))
	}
}

func TestFileStoreConcurrentSaveAndList(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	s := newStore(t, dir)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			appID := strconv.Itoa(i%3 + 1)
			if err := s.Save(ctx, appID, []domain.Review{review("r" + strconv.Itoa(i))}); err != nil {
				t.Errorf("Save: %v", err)
			}
			if _, err := s.ListByApp(ctx, appID); err != nil {
				t.Errorf("ListByApp: %v", err)
			}
		}(i)
	}
	wg.Wait()

	total := 0
	for _, appID := range []string{"1", "2", "3"} {
		got, _ := s.ListByApp(ctx, appID)
		total += len(got)
	}
	if total != 20 {
		t.Fatalf("concurrent saves lost data: total = %d, want 20", total)
	}
}

func newStore(t *testing.T, dir string) *storage.FileStore {
	t.Helper()
	s, err := storage.NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	return s
}

func review(id string) domain.Review {
	return domain.Review{
		ID:          id,
		AppID:       "1",
		Author:      "author-" + id,
		Content:     "content-" + id,
		Score:       3,
		SubmittedAt: time.Date(2026, 5, 28, 6, 0, 0, 0, time.UTC),
	}
}

func idsOf(reviews []domain.Review) []string {
	out := make([]string, len(reviews))
	for i, r := range reviews {
		out[i] = r.ID
	}
	sort.Strings(out)
	return out
}
