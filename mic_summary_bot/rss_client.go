package micsummarybot

import (
	"context"
	"fmt"
	"time"

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
// 発行日時が無いアイテムと、発行日時が現在時刻より未来のアイテムは除外します。
func (c *RSSClient) FetchFeed(ctx context.Context, url string) ([]*gofeed.Item, error) {
	feed, err := c.feedParser.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RSS feed from %s: %w", url, err)
	}

	return filterPublishedItems(feed.Items, time.Now()), nil
}

// filterPublishedItems は発行日時がnow以前のアイテムだけを返します。
// 未来日付のアイテムを登録すると、ItemRepository.AddItems の「前回の最新より古いものは処理済みとする」判定の基準が未来にずれ、
// 以降の新着がすべて処理済み扱いになるため除外します。
// 除外したアイテムは、日付が過ぎた後または配信元で日付が修正された後の取得で改めて登録対象になります。
func filterPublishedItems(items []*gofeed.Item, now time.Time) []*gofeed.Item {
	var publishedItems []*gofeed.Item
	for _, item := range items {
		// published_at が存在しない場合はスキップ
		if item.PublishedParsed == nil {
			continue
		}
		if item.PublishedParsed.After(now) {
			pkgLogger.Warn("Skipping RSS item with future published date", "url", item.Link, "published_at", *item.PublishedParsed)
			continue
		}
		publishedItems = append(publishedItems, item)
	}
	return publishedItems
}
