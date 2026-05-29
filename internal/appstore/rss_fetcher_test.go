package appstore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/souzavinny/reviews-api/internal/service"
)

var _ service.ReviewFetcher = (*RSSFetcher)(nil)

func TestParsePage(t *testing.T) {
	sample, err := os.ReadFile("testdata/sample_feed.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	cases := []struct {
		name    string
		data    []byte
		wantLen int
		wantErr bool
	}{
		{"real feed maps every entry (no metadata skip)", sample, 3, false},
		{"single entry object", []byte(`{"feed":{"entry":{"id":{"label":"1"},"author":{"name":{"label":"solo"}},"content":{"label":"c"},"im:rating":{"label":"3"},"updated":{"label":"2026-05-01T00:00:00-07:00"}}}}`), 1, false},
		{"entry absent", []byte(`{"feed":{"author":{"name":{"label":"iTunes Store"}}}}`), 0, false},
		{"empty entry array", []byte(`{"feed":{"entry":[]}}`), 0, false},
		{"entry null", []byte(`{"feed":{"entry":null}}`), 0, false},
		{"malformed rating", []byte(`{"feed":{"entry":[{"id":{"label":"1"},"im:rating":{"label":"five"},"updated":{"label":"2026-05-01T00:00:00-07:00"}}]}}`), 0, true},
		{"malformed updated", []byte(`{"feed":{"entry":[{"id":{"label":"1"},"im:rating":{"label":"5"},"updated":{"label":"not-a-time"}}]}}`), 0, true},
		{"invalid json", []byte(`{not json`), 0, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parsePage(tc.data, "appid")
			if tc.wantErr {
				if err == nil {
					t.Fatal("want error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tc.wantLen {
				t.Fatalf("len = %d, want %d", len(got), tc.wantLen)
			}
			for _, r := range got {
				if r.AppID != "appid" {
					t.Errorf("AppID = %q, want %q", r.AppID, "appid")
				}
			}
		})
	}
}

func TestParsePageMapsFields(t *testing.T) {
	data, err := os.ReadFile("testdata/sample_feed.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	got, err := parsePage(data, "389801252")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}

	first := got[0]
	if first.ID != "14115629180" {
		t.Errorf("ID = %q", first.ID)
	}
	if first.AppID != "389801252" {
		t.Errorf("AppID = %q", first.AppID)
	}
	if first.Author != "SuoQin16" {
		t.Errorf("Author = %q", first.Author)
	}
	if first.Score != 4 {
		t.Errorf("Score = %d", first.Score)
	}
	if !strings.Contains(first.Content, "the music on post cannot be changed") {
		t.Errorf("Content = %q", first.Content)
	}
	if ts := first.SubmittedAt.Format(time.RFC3339); ts != "2026-05-28T06:25:43-07:00" {
		t.Errorf("SubmittedAt = %q, want 2026-05-28T06:25:43-07:00", ts)
	}

	wantIDs := []string{"14115629180", "14115585288", "14115564793"}
	for i, w := range wantIDs {
		if got[i].ID != w {
			t.Errorf("got[%d].ID = %q, want %q (order must be preserved)", i, got[i].ID, w)
		}
	}

	last := got[2]
	if last.Content != "stop banning me unreasonably" {
		t.Errorf("last.Content = %q", last.Content)
	}
	if last.Score != 1 {
		t.Errorf("last.Score = %d", last.Score)
	}
}

func TestFetch(t *testing.T) {
	now := time.Now()

	t.Run("stops once a page reaches past the window", func(t *testing.T) {
		recent := makeFeed(t, makeEntry("1", "5", now.Add(-1*time.Hour)), makeEntry("2", "4", now.Add(-2*time.Hour)))
		old := makeFeed(t, makeEntry("3", "3", now.Add(-100*time.Hour)), makeEntry("4", "2", now.Add(-101*time.Hour)))

		var maxPage int64
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := pageNum(r.URL.Path)
			storeMax(&maxPage, int64(p))
			switch p {
			case 1:
				w.Write(recent)
			case 2:
				w.Write(old)
			default:
				w.Write(makeFeed(t))
			}
		}))
		defer ts.Close()

		f := newTestFetcher(ts)
		f.window = 48 * time.Hour
		got, err := f.Fetch(context.Background(), "123")
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 4 {
			t.Fatalf("len = %d, want 4", len(got))
		}
		if p := atomic.LoadInt64(&maxPage); p != 2 {
			t.Fatalf("requested up to page %d, want 2 (should stop after the past-window page)", p)
		}
	})

	t.Run("stops at the page cap on a busy app", func(t *testing.T) {
		page := makeFeed(t, makeEntry("a", "5", now.Add(-1*time.Minute)), makeEntry("b", "5", now.Add(-2*time.Minute)))

		var maxPage int64
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			storeMax(&maxPage, int64(pageNum(r.URL.Path)))
			w.Write(page)
		}))
		defer ts.Close()

		f := newTestFetcher(ts)
		f.window = 365 * 24 * time.Hour
		f.maxPages = 3
		got, err := f.Fetch(context.Background(), "123")
		if err != nil {
			t.Fatal(err)
		}
		if p := atomic.LoadInt64(&maxPage); p != 3 {
			t.Fatalf("requested up to page %d, want 3 (page cap)", p)
		}
		if len(got) != 6 {
			t.Fatalf("len = %d, want 6", len(got))
		}
	})
}

func TestExists(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name string
		body []byte
		want bool
	}{
		{"app with reviews", makeFeed(t, makeEntry("1", "5", now)), true},
		{"unknown id returns a feed with no entries", []byte(`{"feed":{"author":{"name":{"label":"iTunes Store"}}}}`), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write(tc.body)
			}))
			defer ts.Close()
			f := newTestFetcher(ts)
			got, err := f.Exists(context.Background(), "324684580")
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("Exists = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFetchRejectsNonNumericAppID(t *testing.T) {
	var hits int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		w.Write(makeFeed(t))
	}))
	defer ts.Close()

	f := newTestFetcher(ts)
	if _, err := f.Fetch(context.Background(), "0/../evil"); err == nil {
		t.Fatal("want error for non-numeric app id, got nil")
	}
	if n := atomic.LoadInt64(&hits); n != 0 {
		t.Fatalf("made %d HTTP request(s) for an invalid app id, want 0 (must reject before the network call)", n)
	}
}

func TestFetchRejectsOversizedBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(make([]byte, 64))
	}))
	defer ts.Close()

	f := newTestFetcher(ts)
	f.maxBytes = 16
	if _, err := f.Fetch(context.Background(), "123"); err == nil {
		t.Fatal("want error for response exceeding the byte limit, got nil")
	}
}

func newTestFetcher(ts *httptest.Server) *RSSFetcher {
	f := NewRSSFetcher()
	f.baseURL = ts.URL
	f.client = ts.Client()
	return f
}

func makeFeed(t *testing.T, entries ...rssEntry) []byte {
	t.Helper()
	var fr feedResponse
	fr.Feed.Entry = entries
	b, err := json.Marshal(fr)
	if err != nil {
		t.Fatalf("marshal feed: %v", err)
	}
	return b
}

func makeEntry(id, rating string, submitted time.Time) rssEntry {
	return rssEntry{
		ID:      label{Label: id},
		Author:  rssAuthor{Name: label{Label: "reviewer-" + id}},
		Content: label{Label: "content for " + id},
		Rating:  label{Label: rating},
		Updated: label{Label: submitted.Format(time.RFC3339)},
	}
}

func pageNum(path string) int {
	i := strings.Index(path, "page=")
	if i < 0 {
		return 0
	}
	rest := path[i+len("page="):]
	if j := strings.IndexByte(rest, '/'); j >= 0 {
		rest = rest[:j]
	}
	n, _ := strconv.Atoi(rest)
	return n
}

func storeMax(p *int64, v int64) {
	for {
		cur := atomic.LoadInt64(p)
		if v <= cur || atomic.CompareAndSwapInt64(p, cur, v) {
			return
		}
	}
}
