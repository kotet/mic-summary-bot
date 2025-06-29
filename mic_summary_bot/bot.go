package micsummarybot

import (
	"context"
	"fmt"
)

type MICSummaryBot struct {
	rssClient      *RSSClient
	genAIClient    *GenAIClient
	mastodonClient *MastodonClient
	itemRepository *ItemRepository
	config         *Config
}

func NewMICSummaryBot(config *Config) (*MICSummaryBot, error) {
	itemRepository, err := NewItemRepository(config.Database.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to create item repository: %w", err)
	}

	genAIClient, err := NewGenAIClient(&config.Gemini, &config.Storage)
	if err != nil {
		return nil, fmt.Errorf("failed to create GenAI client: %w", err)
	}

	mastodonClient, err := NewMastodonClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Mastodon client: %w", err)
	}

	return &MICSummaryBot{
		rssClient:      NewRSSClient(),
		genAIClient:    genAIClient,
		mastodonClient: mastodonClient,
		itemRepository: itemRepository,
		config:         config,
	}, nil
}

func (b *MICSummaryBot) UpdateItems(ctx context.Context) error {
	pkgLogger.Info("Start updating items")
	items, err := b.rssClient.FetchFeed(ctx, b.config.RSS.URL)
	if err != nil {
		return fmt.Errorf("failed to fetch RSS feed: %w", err)
	}

	addedCount, err := b.itemRepository.AddItems(ctx, items)
	if err != nil {
		return fmt.Errorf("failed to add items to repository: %w", err)
	}
	pkgLogger.Info("Added new items", "count", addedCount)

	unprocessedCount, err := b.itemRepository.CountUnprocessedItems(ctx)
	if err != nil {
		return fmt.Errorf("failed to count unprocessed items: %w", err)
	}
	pkgLogger.Info("Unprocessed items", "count", unprocessedCount)

	pkgLogger.Info("Finish updating items")
	return nil
}

func (b *MICSummaryBot) PostSummary(ctx context.Context) error {
	pkgLogger.Info("Start posting summary")
	items, err := b.itemRepository.GetUnprocessedItems(ctx)
	if err != nil {
		return fmt.Errorf("failed to get not posted item: %w", err)
	}
	if len(items) == 0 {
		pkgLogger.Info("No item to post")
		return nil
	}
	item := items[0]
	pkgLogger.Info("Processing item", "url", item.URL)

	htmlAndDocs, err := GetHTMLSummary(item.URL)
	if err != nil {
		return fmt.Errorf("failed to parse html: %w", err)
	}

	screeningResult, err := b.genAIClient.IsWorthSummarizing(htmlAndDocs, b.config.Gemini.ScreeningPrompt)
	if err != nil {
		return fmt.Errorf("failed to screen item: %w", err)
	}
	pkgLogger.Info("Item screening result", "url", item.URL, "result", screeningResult.FinalResult)

	switch screeningResult.FinalResult {
	case WorthSummarizingYes:
		summary, err := b.genAIClient.SummarizeDocument(htmlAndDocs, b.config.Gemini.SummarizingPrompt)
		if err != nil {
			return fmt.Errorf("failed to summarize content: %w", err)
		}

		if err := b.mastodonClient.PostSummary(*item, summary); err != nil {
			return fmt.Errorf("failed to post to mastodon: %w", err)
		}
		item.Status = StatusProcessed
		if err := b.itemRepository.Update(ctx, item); err != nil {
			return fmt.Errorf("failed to mark as posted: %w", err)
		}
	case WorthSummarizingNo:
		item.Status = StatusProcessed
		item.Reason = ReasonGeminiNotValuable
		if err := b.itemRepository.Update(ctx, item); err != nil {
			return fmt.Errorf("failed to mark as not valuable: %w", err)
		}
	case WorthSummarizingWait:
		item.Status = StatusDeferred
		item.Reason = ReasonGeminiPageNotReady
		if err := b.itemRepository.Update(ctx, item); err != nil {
			return fmt.Errorf("failed to mark as not ready: %w", err)
		}
	}

	pkgLogger.Info("Finish posting summary")
	return nil
}
