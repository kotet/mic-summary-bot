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
	itemRepository, err := NewItemRepository(config.Database.Path, config.Database.MaxDeferredRetryCount)
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

func (b *MICSummaryBot) RefreshFeedItems(ctx context.Context) error {
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

	item, err := b.itemRepository.GetItemForSummarization(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pending item: %w", err)
	}

	if item == nil {
		pkgLogger.Info("No pending items to summarize")
		return nil
	}

	pkgLogger.Info("Processing pending item for summarization", "url", item.URL)

	htmlAndDocs, err := GetHTMLSummary(item.URL)
	if err != nil {
		return fmt.Errorf("failed to parse html: %w", err)
	}

	summary, err := b.genAIClient.SummarizeDocument(htmlAndDocs, b.config.Gemini.SummarizingPrompt)
	if err != nil {
		return fmt.Errorf("failed to summarize content: %w", err)
	}

	if err := b.mastodonClient.PostSummary(ctx, *item, summary); err != nil {
		return fmt.Errorf("failed to post to mastodon: %w", err)
	}

	item.Status = StatusProcessed
	if err := b.itemRepository.Update(ctx, item); err != nil {
		return fmt.Errorf("failed to mark as posted: %w", err)
	}

	pkgLogger.Info("Finish posting summary")
	return nil
}

func (b *MICSummaryBot) ScreenItem(ctx context.Context) error {
	item, err := b.itemRepository.GetItemForScreening(ctx)
	if err != nil {
		return fmt.Errorf("failed to get item for screening: %w", err)
	}

	if item == nil {
		pkgLogger.Info("No items to screen")
		return nil
	}

	pkgLogger.Info("Screening item", "url", item.URL)

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
		item.Status = StatusPending
		if err := b.itemRepository.Update(ctx, item); err != nil {
			return fmt.Errorf("failed to mark as pending: %w", err)
		}
	case WorthSummarizingNo:
		item.Status = StatusProcessed
		item.Reason = ReasonGeminiNotValuable
		if err := b.mastodonClient.PostNoValue(ctx, *item); err != nil {
			return fmt.Errorf("failed to post no value message to mastodon: %w", err)
		}
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

	return nil
}
