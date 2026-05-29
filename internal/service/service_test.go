package service_test

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/souzavinny/reviews-api/internal/domain"
	"github.com/souzavinny/reviews-api/internal/service"
)

var (
	_ service.ReviewStore   = (*fakeStore)(nil)
	_ service.ReviewFetcher = (*fakeFetcher)(nil)
	_ service.AppRegistry   = (*fakeRegistry)(nil)
)

func TestPollAppSavesFetchedReviews(t *testing.T) {
	store := newFakeStore()
	fetcher := &fakeFetcher{fetchResults: [][]domain.Review{{rev("a", 5, time.Hour), rev("b", 4, 2*time.Hour)}}}
	svc := service.New(store, fetcher, newFakeRegistry())

	if err := svc.PollApp(context.Background(), "1"); err != nil {
		t.Fatal(err)
	}
	if got := len(store.reviews["1"]); got != 2 {
		t.Fatalf("stored %d reviews, want 2", got)
	}
}

func TestPollAppPropagatesFetchError(t *testing.T) {
	store := newFakeStore()
	fetcher := &fakeFetcher{fetchErr: errors.New("network down")}
	svc := service.New(store, fetcher, newFakeRegistry())

	if err := svc.PollApp(context.Background(), "1"); err == nil {
		t.Fatal("want error from failed fetch, got nil")
	}
	if len(store.reviews["1"]) != 0 {
		t.Fatal("nothing should be stored when the fetch fails")
	}
}

func TestPollDedupesAcrossPolls(t *testing.T) {
	store := newFakeStore()
	fetcher := &fakeFetcher{fetchResults: [][]domain.Review{
		{rev("a", 5, time.Hour), rev("b", 4, 2*time.Hour)},
		{rev("b", 4, 2*time.Hour), rev("c", 3, 3*time.Hour)}, // b overlaps
	}}
	svc := service.New(store, fetcher, newFakeRegistry())
	ctx := context.Background()

	if err := svc.PollApp(ctx, "1"); err != nil {
		t.Fatal(err)
	}
	if err := svc.PollApp(ctx, "1"); err != nil {
		t.Fatal(err)
	}

	got, _ := svc.GetRecentReviews(ctx, "1", 30*24*time.Hour)
	assertIDs(t, got, []string{"a", "b", "c"}) // newest-first; b not duplicated
}

func TestGetRecentReviewsWindowingAndOrder(t *testing.T) {
	store := newFakeStore()
	store.reviews["1"] = []domain.Review{
		rev("old", 5, 50*time.Hour),        // outside 48h, inside 7d
		rev("mid", 4, 30*time.Hour),        // inside 48h, outside 24h
		rev("new", 3, 1*time.Hour),         // inside 24h
		rev("ancient", 2, 10*24*time.Hour), // outside 7d
	}
	svc := service.New(store, &fakeFetcher{}, newFakeRegistry())
	ctx := context.Background()

	got24, _ := svc.GetRecentReviews(ctx, "1", 24*time.Hour)
	assertIDs(t, got24, []string{"new"})

	got48, _ := svc.GetRecentReviews(ctx, "1", 48*time.Hour)
	assertIDs(t, got48, []string{"new", "mid"})

	got7d, _ := svc.GetRecentReviews(ctx, "1", 7*24*time.Hour)
	assertIDs(t, got7d, []string{"new", "mid", "old"})
}

func TestGetRecentReviewsUnknownAppIsEmpty(t *testing.T) {
	svc := service.New(newFakeStore(), &fakeFetcher{}, newFakeRegistry())
	got, err := svc.GetRecentReviews(context.Background(), "nope", 48*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("unknown app should yield no reviews, got %d", len(got))
	}
}

func TestGetRecentReviewsPropagatesStoreError(t *testing.T) {
	store := newFakeStore()
	store.listErr = errors.New("disk error")
	svc := service.New(store, &fakeFetcher{}, newFakeRegistry())

	if _, err := svc.GetRecentReviews(context.Background(), "1", 48*time.Hour); err == nil {
		t.Fatal("want error from the store, got nil")
	}
}

func TestAddAppValidatesAndRegisters(t *testing.T) {
	fetcher := &fakeFetcher{exists: true}
	reg := newFakeRegistry()
	svc := service.New(newFakeStore(), fetcher, reg)

	app, err := svc.AddApp(context.Background(), "123")
	if err != nil {
		t.Fatal(err)
	}
	if app.ID != "123" {
		t.Fatalf("returned app id %q, want 123", app.ID)
	}
	if fetcher.existsCalls != 1 {
		t.Fatalf("Exists called %d times, want 1", fetcher.existsCalls)
	}
	if _, ok := reg.apps["123"]; !ok {
		t.Fatal("app was not registered")
	}
}

func TestAddAppRejectsUnknownApp(t *testing.T) {
	reg := newFakeRegistry()
	svc := service.New(newFakeStore(), &fakeFetcher{exists: false}, reg)

	_, err := svc.AddApp(context.Background(), "999")
	if !errors.Is(err, service.ErrAppNotFound) {
		t.Fatalf("want ErrAppNotFound, got %v", err)
	}
	if len(reg.apps) != 0 {
		t.Fatal("an unvalidated app must not be registered")
	}
}

func TestAddAppPropagatesValidationError(t *testing.T) {
	reg := newFakeRegistry()
	svc := service.New(newFakeStore(), &fakeFetcher{existsErr: errors.New("timeout")}, reg)

	_, err := svc.AddApp(context.Background(), "123")
	if err == nil {
		t.Fatal("want error from failed validation")
	}
	if errors.Is(err, service.ErrAppNotFound) {
		t.Fatal("a network error must not be reported as ErrAppNotFound")
	}
	if len(reg.apps) != 0 {
		t.Fatal("app must not be registered when validation errors")
	}
}

func TestAddAppPropagatesRegistryAddError(t *testing.T) {
	reg := newFakeRegistry()
	reg.addErr = errors.New("registry write failed")
	store := newFakeStore()
	svc := service.New(store, &fakeFetcher{exists: true}, reg)

	if _, err := svc.AddApp(context.Background(), "123"); err == nil {
		t.Fatal("want error when the registry add fails")
	}
}

func TestListApps(t *testing.T) {
	reg := newFakeRegistry()
	reg.apps["2"] = domain.App{ID: "2"}
	reg.apps["1"] = domain.App{ID: "1"}
	svc := service.New(newFakeStore(), &fakeFetcher{}, reg)

	got, err := svc.ListApps(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertAppIDs(t, got, []string{"1", "2"})
}

func TestRemoveApp(t *testing.T) {
	reg := newFakeRegistry()
	reg.apps["1"] = domain.App{ID: "1"}
	svc := service.New(newFakeStore(), &fakeFetcher{}, reg)

	if err := svc.RemoveApp(context.Background(), "1"); err != nil {
		t.Fatal(err)
	}
	if len(reg.apps) != 0 {
		t.Fatal("app was not removed")
	}
}

func TestGetSummaryAggregatesWithinWindow(t *testing.T) {
	store := newFakeStore()
	store.reviews["1"] = []domain.Review{
		rev("a", 5, time.Hour),
		rev("b", 4, 2*time.Hour),
		rev("c", 5, 3*time.Hour),
		rev("d", 1, 4*time.Hour),
		rev("out", 1, 10*24*time.Hour), // outside the window — must be excluded
	}
	svc := service.New(store, &fakeFetcher{}, newFakeRegistry())

	sum, err := svc.GetSummary(context.Background(), "1", 48*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Total != 4 {
		t.Fatalf("total = %d, want 4 (out-of-window excluded)", sum.Total)
	}
	if sum.Average != 3.75 { // (5+4+5+1)/4
		t.Fatalf("average = %v, want 3.75", sum.Average)
	}
	want := map[int]int{1: 1, 2: 0, 3: 0, 4: 1, 5: 2}
	for star, n := range want {
		if sum.CountByStar[star] != n {
			t.Fatalf("countByStar = %v, want %v", sum.CountByStar, want)
		}
	}
	newest := store.reviews["1"][0].SubmittedAt // "a", age 1h
	if !sum.LastUpdated.Equal(newest) {
		t.Fatalf("lastUpdated = %v, want %v", sum.LastUpdated, newest)
	}
}

func TestGetSummaryEmptyWindow(t *testing.T) {
	store := newFakeStore()
	store.reviews["1"] = []domain.Review{rev("old", 5, 30*24*time.Hour)} // outside window
	svc := service.New(store, &fakeFetcher{}, newFakeRegistry())

	sum, err := svc.GetSummary(context.Background(), "1", 48*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Total != 0 || sum.Average != 0 {
		t.Fatalf("empty summary: total=%d average=%v, want 0/0", sum.Total, sum.Average)
	}
	if len(sum.CountByStar) != 5 {
		t.Fatalf("countByStar should still hold stars 1-5: %v", sum.CountByStar)
	}
	for star := 1; star <= 5; star++ {
		if sum.CountByStar[star] != 0 {
			t.Fatalf("countByStar[%d] = %d, want 0", star, sum.CountByStar[star])
		}
	}
	if !sum.LastUpdated.IsZero() {
		t.Fatalf("lastUpdated = %v, want zero", sum.LastUpdated)
	}
}

// --- fakes ---

type fakeStore struct {
	reviews map[string][]domain.Review
	listErr error
}

func newFakeStore() *fakeStore { return &fakeStore{reviews: map[string][]domain.Review{}} }

func (f *fakeStore) Save(ctx context.Context, appID string, reviews []domain.Review) error {
	existing := f.reviews[appID]
	seen := make(map[string]struct{}, len(existing))
	for _, r := range existing {
		seen[r.ID] = struct{}{}
	}
	for _, r := range reviews {
		if _, ok := seen[r.ID]; ok {
			continue
		}
		seen[r.ID] = struct{}{}
		existing = append(existing, r)
	}
	f.reviews[appID] = existing
	return nil
}

func (f *fakeStore) ListByApp(ctx context.Context, appID string) ([]domain.Review, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.reviews[appID], nil
}

type fakeFetcher struct {
	fetchResults [][]domain.Review
	fetchCalls   int
	fetchErr     error
	exists       bool
	existsErr    error
	existsCalls  int
}

func (f *fakeFetcher) Fetch(ctx context.Context, appID string) ([]domain.Review, error) {
	if f.fetchErr != nil {
		return nil, f.fetchErr
	}
	var out []domain.Review
	if f.fetchCalls < len(f.fetchResults) {
		out = f.fetchResults[f.fetchCalls]
	}
	f.fetchCalls++
	return out, nil
}

func (f *fakeFetcher) Exists(ctx context.Context, appID string) (bool, error) {
	f.existsCalls++
	if f.existsErr != nil {
		return false, f.existsErr
	}
	return f.exists, nil
}

type fakeRegistry struct {
	apps   map[string]domain.App
	addErr error
}

func newFakeRegistry() *fakeRegistry { return &fakeRegistry{apps: map[string]domain.App{}} }

func (f *fakeRegistry) List(ctx context.Context) ([]domain.App, error) {
	out := make([]domain.App, 0, len(f.apps))
	for _, a := range f.apps {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (f *fakeRegistry) Add(ctx context.Context, app domain.App) error {
	if f.addErr != nil {
		return f.addErr
	}
	f.apps[app.ID] = app
	return nil
}

func (f *fakeRegistry) Remove(ctx context.Context, appID string) error {
	delete(f.apps, appID)
	return nil
}

// --- helpers ---

func rev(id string, score int, age time.Duration) domain.Review {
	return domain.Review{
		ID:          id,
		AppID:       "1",
		Author:      "author-" + id,
		Content:     "content-" + id,
		Score:       score,
		SubmittedAt: time.Now().Add(-age),
	}
}

func assertIDs(t *testing.T, reviews []domain.Review, want []string) {
	t.Helper()
	got := make([]string, len(reviews))
	for i, r := range reviews {
		got[i] = r.ID
	}
	if len(got) != len(want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ids = %v, want %v (order matters)", got, want)
		}
	}
}

func assertAppIDs(t *testing.T, apps []domain.App, want []string) {
	t.Helper()
	got := make([]string, len(apps))
	for i, a := range apps {
		got[i] = a.ID
	}
	if len(got) != len(want) {
		t.Fatalf("app ids = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("app ids = %v, want %v", got, want)
		}
	}
}
