package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/souzavinny/reviews-api/internal/domain"
)

// ErrAppNotFound is returned by AddApp when the app id has no reachable feed.
var ErrAppNotFound = errors.New("app not found")

// Service is the application logic over the three store/fetcher/registry seams.
type Service struct {
	store    ReviewStore
	fetcher  ReviewFetcher
	registry AppRegistry
}

// New wires the service to its collaborators.
func New(store ReviewStore, fetcher ReviewFetcher, registry AppRegistry) *Service {
	return &Service{store: store, fetcher: fetcher, registry: registry}
}

// PollApp fetches the app's latest reviews and persists the new ones.
func (s *Service) PollApp(ctx context.Context, appID string) error {
	reviews, err := s.fetcher.Fetch(ctx, appID)
	if err != nil {
		return fmt.Errorf("poll app %s: %w", appID, err)
	}
	if err := s.store.Save(ctx, appID, reviews); err != nil {
		return fmt.Errorf("poll app %s: %w", appID, err)
	}
	return nil
}

// GetRecentReviews returns the app's reviews within the window, newest first.
func (s *Service) GetRecentReviews(ctx context.Context, appID string, window time.Duration) ([]domain.Review, error) {
	reviews, err := s.store.ListByApp(ctx, appID)
	if err != nil {
		return nil, err
	}
	recent := within(reviews, window)
	sort.Slice(recent, func(i, j int) bool {
		if !recent[i].SubmittedAt.Equal(recent[j].SubmittedAt) {
			return recent[i].SubmittedAt.After(recent[j].SubmittedAt)
		}
		return recent[i].ID > recent[j].ID
	})
	return recent, nil
}

// AddApp validates the app id against the live feed, registers it, and triggers
// an initial poll. A failed initial poll is logged, not fatal: the app stays
// registered and the scheduler polls it on the next tick.
func (s *Service) AddApp(ctx context.Context, appID string) (domain.App, error) {
	exists, err := s.fetcher.Exists(ctx, appID)
	if err != nil {
		return domain.App{}, fmt.Errorf("validate app %s: %w", appID, err)
	}
	if !exists {
		return domain.App{}, fmt.Errorf("%w: %s", ErrAppNotFound, appID)
	}

	app := domain.App{ID: appID}
	if err := s.registry.Add(ctx, app); err != nil {
		return domain.App{}, fmt.Errorf("add app %s: %w", appID, err)
	}
	if err := s.PollApp(ctx, appID); err != nil {
		log.Printf("service: initial poll of app %s failed (registered; will retry on schedule): %v", appID, err)
	}
	return app, nil
}

// ListApps returns the monitored apps.
func (s *Service) ListApps(ctx context.Context) ([]domain.App, error) {
	return s.registry.List(ctx)
}

// RemoveApp stops monitoring an app; removing an unmonitored app is a no-op.
func (s *Service) RemoveApp(ctx context.Context, appID string) error {
	return s.registry.Remove(ctx, appID)
}

// GetSummary aggregates the app's reviews within the window: total, average
// score, count per star (1-5), and the most recent submission time. It is
// windowed — not the full stored set — so the boundary-page reviews the fetcher
// over-reads can't skew the stats, and the summary matches the reviews list.
func (s *Service) GetSummary(ctx context.Context, appID string, window time.Duration) (domain.Summary, error) {
	reviews, err := s.store.ListByApp(ctx, appID)
	if err != nil {
		return domain.Summary{}, err
	}
	recent := within(reviews, window)

	summary := domain.Summary{
		Total:       len(recent),
		CountByStar: map[int]int{1: 0, 2: 0, 3: 0, 4: 0, 5: 0},
	}
	if len(recent) == 0 {
		return summary, nil
	}
	sum := 0
	for _, r := range recent {
		sum += r.Score
		summary.CountByStar[r.Score]++
		if r.SubmittedAt.After(summary.LastUpdated) {
			summary.LastUpdated = r.SubmittedAt
		}
	}
	summary.Average = float64(sum) / float64(len(recent))
	return summary, nil
}

func within(reviews []domain.Review, window time.Duration) []domain.Review {
	out := make([]domain.Review, 0, len(reviews))
	for _, r := range reviews {
		if r.WithinLast(window) {
			out = append(out, r)
		}
	}
	return out
}
