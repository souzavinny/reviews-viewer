// Package domain defines the core types shared across the reviews API.
// It depends on no other layer.
package domain

import "time"

// Review is a single customer review of an app, normalized from the feed.
type Review struct {
	ID          string    `json:"id"`
	AppID       string    `json:"appId"`
	Author      string    `json:"author"`
	Content     string    `json:"content"`
	Score       int       `json:"score"`
	SubmittedAt time.Time `json:"submittedAt"`
}

// WithinLast reports whether the review was submitted within the last d — that
// is, between now-d and now. Future-dated reviews are excluded.
func (r Review) WithinLast(d time.Duration) bool {
	age := time.Since(r.SubmittedAt)
	return age >= 0 && age <= d
}
