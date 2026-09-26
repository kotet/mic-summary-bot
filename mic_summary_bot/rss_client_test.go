package micsummarybot

import (
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
)

func TestFilterPublishedItems(t *testing.T) {
	now := time.Date(2026, 9, 18, 20, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Second)

	items := []*gofeed.Item{
		{Link: "no-date", PublishedParsed: nil},
		{Link: "past", PublishedParsed: &past},
		{Link: "now", PublishedParsed: &now},
		{Link: "future", PublishedParsed: &future},
	}

	var links []string
	for _, item := range filterPublishedItems(items, now) {
		links = append(links, item.Link)
	}
	assert.Equal(t, []string{"past", "now"}, links)
}
