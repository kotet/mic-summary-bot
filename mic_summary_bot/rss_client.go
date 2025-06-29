package micsummarybot

import (
	"context"
	"fmt"

	"github.com/mmcdole/gofeed"
)

// RSSClient はRSSフィードの取得とパースを行うクライアント
type RSSClient struct {
	feedParser *gofeed.Parser
}

// NewRSSClient は新しいRSSClientインスタンスを作成します。
func NewRSSClient() *RSSClient {
	return &RSSClient{
		feedParser: gofeed.NewParser(),
	}
}

// FetchFeed は指定されたURLからRSSフィードを取得し、パースします。
func (c *RSSClient) FetchFeed(ctx context.Context, url string) ([]*gofeed.Item, error) {
	feed, err := c.feedParser.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RSS feed from %s: %w", url, err)
	}

	var newItems []*gofeed.Item
	for _, item := range feed.Items {
		// published_at が存在しない場合はスキップ
		if item.PublishedParsed == nil {
			continue
		}
		newItems = append(newItems, item)
	}

	return newItems, nil
}
