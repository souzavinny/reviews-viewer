package appstore

import (
	"bytes"
	"encoding/json"
)

// feedResponse models the iTunes customer-reviews JSON document. Store/feed
// metadata lives at the feed level (author, title); every feed.entry is a
// customer review — the JSON variant has no leading metadata entry.
type feedResponse struct {
	Feed struct {
		Entry entryList `json:"entry"`
	} `json:"feed"`
}

// entryList decodes feed.entry, which the feed emits as an array of reviews, a
// single review object when only one exists, or omits entirely when none do.
type entryList []rssEntry

func (l *entryList) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if string(trimmed) == "null" {
		return nil
	}
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var arr []rssEntry
		if err := json.Unmarshal(trimmed, &arr); err != nil {
			return err
		}
		*l = arr
		return nil
	}
	var single rssEntry
	if err := json.Unmarshal(trimmed, &single); err != nil {
		return err
	}
	*l = entryList{single}
	return nil
}

type rssEntry struct {
	ID      label     `json:"id"`
	Author  rssAuthor `json:"author"`
	Content label     `json:"content"`
	Rating  label     `json:"im:rating"`
	Updated label     `json:"updated"`
}

type rssAuthor struct {
	Name label `json:"name"`
}

type label struct {
	Label string `json:"label"`
}
