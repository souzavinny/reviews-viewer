// Package appstore implements service.ReviewFetcher against Apple's iTunes
// customer-reviews RSS feed (the JSON variant).
package appstore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/souzavinny/reviews-api/internal/domain"
)

const (
	defaultBaseURL  = "https://itunes.apple.com/rss/customerreviews"
	defaultMaxPages = 10
	defaultMaxBytes = 4 << 20
	defaultWindow   = 7 * 24 * time.Hour
	requestTimeout  = 15 * time.Second
)

// RSSFetcher fetches and parses reviews from the iTunes customer-reviews feed.
// maxPages caps pagination so a busy app can never loop forever; window bounds
// how far back to page — older pages aren't fetched once the window is covered.
type RSSFetcher struct {
	client   *http.Client
	baseURL  string
	maxPages int
	maxBytes int64
	window   time.Duration
}

// NewRSSFetcher returns a fetcher with production defaults.
func NewRSSFetcher() *RSSFetcher {
	return &RSSFetcher{
		client:   &http.Client{Timeout: requestTimeout},
		baseURL:  defaultBaseURL,
		maxPages: defaultMaxPages,
		maxBytes: defaultMaxBytes,
		window:   defaultWindow,
	}
}

// Fetch pages newest-first through the feed, accumulating reviews until a page
// reaches past the window or the page cap is hit. Windowing for a specific
// query is applied later by the service; this only bounds how far back to page.
func (f *RSSFetcher) Fetch(ctx context.Context, appID string) ([]domain.Review, error) {
	cutoff := time.Now().Add(-f.window)
	var all []domain.Review
	for page := 1; page <= f.maxPages; page++ {
		reviews, err := f.fetchPage(ctx, appID, page)
		if err != nil {
			return nil, err
		}
		if len(reviews) == 0 {
			break
		}
		all = append(all, reviews...)
		// sortBy=mostRecent orders pages newest-first, so once a page reaches past the
		// window every later page is older — safe to stop paging here.
		if oldest(reviews).Before(cutoff) {
			break
		}
	}
	return all, nil
}

// Exists reports whether the app has a reachable feed with at least one review.
// A page-one fetch is enough; the feed returns no entries for an unknown id.
func (f *RSSFetcher) Exists(ctx context.Context, appID string) (bool, error) {
	reviews, err := f.fetchPage(ctx, appID, 1)
	if err != nil {
		return false, err
	}
	return len(reviews) > 0, nil
}

func (f *RSSFetcher) fetchPage(ctx context.Context, appID string, page int) ([]domain.Review, error) {
	if _, err := strconv.ParseUint(appID, 10, 64); err != nil {
		return nil, fmt.Errorf("invalid app id %q: must be numeric", appID)
	}
	feedURL := fmt.Sprintf("%s/id=%s/sortBy=mostRecent/page=%d/json", f.baseURL, appID, page)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "reviews-api/1.0")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("feed for app %s page %d: status %d", appID, page, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, f.maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > f.maxBytes {
		return nil, fmt.Errorf("feed for app %s page %d: response exceeds %d-byte limit", appID, page, f.maxBytes)
	}
	return parsePage(body, appID)
}

func parsePage(data []byte, appID string) ([]domain.Review, error) {
	var fr feedResponse
	if err := json.Unmarshal(data, &fr); err != nil {
		return nil, fmt.Errorf("decode feed for app %s: %w", appID, err)
	}
	reviews := make([]domain.Review, 0, len(fr.Feed.Entry))
	for _, e := range fr.Feed.Entry {
		r, err := toReview(e, appID)
		if err != nil {
			return nil, err
		}
		reviews = append(reviews, r)
	}
	return reviews, nil
}

func toReview(e rssEntry, appID string) (domain.Review, error) {
	score, err := strconv.Atoi(strings.TrimSpace(e.Rating.Label))
	if err != nil {
		return domain.Review{}, fmt.Errorf("review %s: bad rating %q: %w", e.ID.Label, e.Rating.Label, err)
	}
	submitted, err := time.Parse(time.RFC3339, e.Updated.Label)
	if err != nil {
		return domain.Review{}, fmt.Errorf("review %s: bad updated %q: %w", e.ID.Label, e.Updated.Label, err)
	}
	return domain.Review{
		ID:          e.ID.Label,
		AppID:       appID,
		Author:      e.Author.Name.Label,
		Content:     e.Content.Label,
		Score:       score,
		SubmittedAt: submitted,
	}, nil
}

func oldest(reviews []domain.Review) time.Time {
	t := reviews[0].SubmittedAt
	for _, r := range reviews[1:] {
		if r.SubmittedAt.Before(t) {
			t = r.SubmittedAt
		}
	}
	return t
}
