package domain_test

import (
	"testing"
	"time"

	"github.com/souzavinny/reviews-api/internal/domain"
)

func TestWithinLast(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name      string
		submitted time.Time
		window    time.Duration
		want      bool
	}{
		{"recent, inside window", now.Add(-1 * time.Hour), 48 * time.Hour, true},
		{"older than window", now.Add(-50 * time.Hour), 48 * time.Hour, false},
		{"future date is excluded", now.Add(1 * time.Hour), 48 * time.Hour, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := domain.Review{SubmittedAt: tc.submitted}
			if got := r.WithinLast(tc.window); got != tc.want {
				t.Fatalf("WithinLast = %v, want %v", got, tc.want)
			}
		})
	}
}
