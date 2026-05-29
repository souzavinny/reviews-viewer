package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/souzavinny/reviews-api/internal/domain"
	"github.com/souzavinny/reviews-api/internal/httpapi"
	"github.com/souzavinny/reviews-api/internal/service"
	"github.com/souzavinny/reviews-api/internal/storage"
)

var _ service.ReviewFetcher = (*fakeFetcher)(nil)

func TestGetReviewsDefaultsTo48hNewestFirst(t *testing.T) {
	now := time.Now()
	fetcher := &fakeFetcher{exists: true, reviews: []domain.Review{
		{ID: "r1", AppID: "123", Author: "a", Content: "c", Score: 5, SubmittedAt: now.Add(-1 * time.Hour)},
		{ID: "r2", AppID: "123", Author: "a", Content: "c", Score: 4, SubmittedAt: now.Add(-30 * time.Hour)},
		{ID: "r3", AppID: "123", Author: "a", Content: "c", Score: 3, SubmittedAt: now.Add(-50 * time.Hour)},
	}}
	h, svc := newTestHandler(t, fetcher, nil)
	if err := svc.PollApp(context.Background(), "123"); err != nil {
		t.Fatal(err)
	}

	rec := do(h, http.MethodGet, "/apps/123/reviews", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q", ct)
	}
	got := decodeReviews(t, rec)
	if want := []string{"r1", "r2"}; !equal(reviewIDs(got), want) { // r3 is outside 48h
		t.Fatalf("default-window reviews = %v, want %v (newest-first)", reviewIDs(got), want)
	}

	rec = do(h, http.MethodGet, "/apps/123/reviews?hours=24", "")
	if want := []string{"r1"}; !equal(reviewIDs(decodeReviews(t, rec)), want) {
		t.Fatalf("hours=24 reviews = %v, want %v", reviewIDs(decodeReviews(t, rec)), want)
	}
}

func TestGetReviewsUnknownAppReturnsEmptyArray(t *testing.T) {
	h, _ := newTestHandler(t, &fakeFetcher{}, nil)
	rec := do(h, http.MethodGet, "/apps/999/reviews", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != "[]" {
		t.Fatalf("body = %q, want []", body)
	}
}

func TestReviewsRejectsNonNumericAppID(t *testing.T) {
	h, _ := newTestHandler(t, &fakeFetcher{}, nil)
	rec := do(h, http.MethodGet, "/apps/abc/reviews", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for non-numeric app id", rec.Code)
	}
}

func TestListApps(t *testing.T) {
	h, _ := newTestHandler(t, &fakeFetcher{}, []domain.App{{ID: "1"}, {ID: "2"}})
	rec := do(h, http.MethodGet, "/apps", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var apps []domain.App
	if err := json.Unmarshal(rec.Body.Bytes(), &apps); err != nil {
		t.Fatal(err)
	}
	if len(apps) != 2 {
		t.Fatalf("listed %d apps, want 2", len(apps))
	}
}

func TestAddAppCreatesAndPolls(t *testing.T) {
	fetcher := &fakeFetcher{exists: true, reviews: []domain.Review{
		{ID: "r1", AppID: "123", Score: 5, SubmittedAt: time.Now().Add(-1 * time.Hour)},
	}}
	h, svc := newTestHandler(t, fetcher, nil)

	rec := do(h, http.MethodPost, "/apps", `{"id":"123"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	var app domain.App
	if err := json.Unmarshal(rec.Body.Bytes(), &app); err != nil {
		t.Fatal(err)
	}
	if app.ID != "123" {
		t.Fatalf("created app id = %q, want 123", app.ID)
	}
	if apps, _ := svc.ListApps(context.Background()); len(apps) != 1 {
		t.Fatal("app was not registered")
	}
	// the OnAppAdded hook (wired to a synchronous poll in tests) populated reviews
	if reviews, _ := svc.GetRecentReviews(context.Background(), "123", 48*time.Hour); len(reviews) != 1 {
		t.Fatal("initial poll did not populate reviews")
	}
}

func TestAddAppPollFailureStillCreates(t *testing.T) {
	fetcher := &fakeFetcher{exists: true, fetchErr: errTest}
	h, svc := newTestHandler(t, fetcher, nil)

	rec := do(h, http.MethodPost, "/apps", `{"id":"123"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 despite poll failure", rec.Code)
	}
	if apps, _ := svc.ListApps(context.Background()); len(apps) != 1 {
		t.Fatal("app should be registered even when the initial poll fails")
	}
}

func TestAddAppRejectsBadInput(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		exists bool
		want   int
	}{
		{"non-numeric id", `{"id":"abc"}`, true, http.StatusBadRequest},
		{"missing id", `{}`, true, http.StatusBadRequest},
		{"malformed json", `{`, true, http.StatusBadRequest},
		{"unknown app", `{"id":"999"}`, false, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newTestHandler(t, &fakeFetcher{exists: tc.exists}, nil)
			rec := do(h, http.MethodPost, "/apps", tc.body)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestRemoveApp(t *testing.T) {
	h, svc := newTestHandler(t, &fakeFetcher{}, []domain.App{{ID: "123"}})
	rec := do(h, http.MethodDelete, "/apps/123", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if apps, _ := svc.ListApps(context.Background()); len(apps) != 0 {
		t.Fatal("app was not removed")
	}
}

func TestGetSummary(t *testing.T) {
	now := time.Now()
	fetcher := &fakeFetcher{exists: true, reviews: []domain.Review{
		{ID: "r1", AppID: "123", Score: 5, SubmittedAt: now.Add(-1 * time.Hour)},
		{ID: "r2", AppID: "123", Score: 3, SubmittedAt: now.Add(-2 * time.Hour)},
	}}
	h, svc := newTestHandler(t, fetcher, nil)
	if err := svc.PollApp(context.Background(), "123"); err != nil {
		t.Fatal(err)
	}

	rec := do(h, http.MethodGet, "/apps/123/summary", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var summary domain.Summary
	if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Total != 2 || summary.Average != 4 {
		t.Fatalf("summary total=%d average=%v, want 2/4", summary.Total, summary.Average)
	}
}

func TestCORS(t *testing.T) {
	h, _ := newTestHandler(t, &fakeFetcher{}, nil)

	pre := do(h, http.MethodOptions, "/apps", "")
	if pre.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", pre.Code)
	}
	if pre.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("preflight missing Access-Control-Allow-Origin")
	}
	if m := pre.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(m, "DELETE") {
		t.Errorf("allowed methods = %q, want DELETE included", m)
	}

	get := do(h, http.MethodGet, "/apps", "")
	if get.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Error("GET response missing CORS origin header")
	}
}

func TestHealthz(t *testing.T) {
	h, _ := newTestHandler(t, &fakeFetcher{}, nil)
	rec := do(h, http.MethodGet, "/healthz", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

// --- helpers & fakes ---

var errTest = errors.New("boom")

func newTestHandler(t *testing.T, fetcher service.ReviewFetcher, seed []domain.App) (http.Handler, *service.Service) {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := storage.NewAppStore(dir, seed)
	if err != nil {
		t.Fatal(err)
	}
	svc := service.New(store, fetcher, registry)
	h := httpapi.New(svc, httpapi.Config{
		DefaultWindow: 48 * time.Hour,
		AllowedOrigin: "*",
		OnAppAdded:    func(id string) { _ = svc.PollApp(context.Background(), id) },
	})
	return h, svc
}

func do(h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, target, nil)
	} else {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func decodeReviews(t *testing.T, rec *httptest.ResponseRecorder) []domain.Review {
	t.Helper()
	var reviews []domain.Review
	if err := json.Unmarshal(rec.Body.Bytes(), &reviews); err != nil {
		t.Fatalf("decode reviews: %v", err)
	}
	return reviews
}

func reviewIDs(reviews []domain.Review) []string {
	out := make([]string, len(reviews))
	for i, r := range reviews {
		out[i] = r.ID
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type fakeFetcher struct {
	exists   bool
	reviews  []domain.Review
	fetchErr error
}

func (f *fakeFetcher) Fetch(ctx context.Context, appID string) ([]domain.Review, error) {
	if f.fetchErr != nil {
		return nil, f.fetchErr
	}
	return f.reviews, nil
}

func (f *fakeFetcher) Exists(ctx context.Context, appID string) (bool, error) {
	return f.exists, nil
}
