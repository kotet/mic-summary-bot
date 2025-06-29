package micsummarybot

import (
	"context"
	"fmt"
	"log/slog"
)

type MICSummaryBot struct {
	rssClient      *RSSClient
	screener       *GenAIClient
	summarizer     *GenAIClient
	mastodonClient *MastodonClient
	itemRepository *ItemRepository
	config         *Config
}

func NewMICSummaryBot(config *Config) (*MICSummaryBot, error) {
	itemRepository, err := NewItemRepository(config.Database.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to create item repository: %w", err)
	}

	summarizer, err := NewGenAIClient(&config.Gemini, &config.Storage)
	if err != nil {
		return nil, fmt.Errorf("failed to create summarizer: %w", err)
	}

	screener, err := NewGenAIClient(&config.Gemini, &config.Storage)
	if err != nil {
		return nil, fmt.Errorf("failed to create screener: %w", err)
	}

	return &MICSummaryBot{
		rssClient:      NewRSSClient(),
		screener:       screener,
		summarizer:     summarizer,
		mastodonClient: NewMastodonClient(config),
		itemRepository: itemRepository,
		config:         config,
	}, nil
}

func (b *MICSummaryBot) UpdateItems(ctx context.Context) error {
	slog.Info("Start updating items")
	items, err := b.rssClient.FetchFeed(b.config.RSS.URL)
	if err != nil {
		return fmt.Errorf("failed to fetch RSS feed: %w", err)
	}

	if err := b.itemRepository.AddItems(items); err != nil {
		return fmt.Errorf("failed to add items to repository: %w", err)
	}
	slog.Info("Finish updating items")
	return nil
}

func (b *MICSummaryBot) PostSummary(ctx context.Context) error {
	slog.Info("Start posting summary")
	items, err := b.itemRepository.GetUnprocessedItems()
	if err != nil {
		return fmt.Errorf("failed to get not posted item: %w", err)
	}
	if len(items) == 0 {
		slog.Info("No item to post")
		return nil
	}
	item := items[0]

	htmlAndDocs, err := GetHTMLSummary(item.URL)
	if err != nil {
		return fmt.Errorf("failed to parse html: %w", err)
	}

	screeningResult, err := b.screener.IsWorthSummarizing(htmlAndDocs, b.config.Gemini.ScreeningPrompt)
	if err != nil {
		return fmt.Errorf("failed to screen item: %w", err)
	}

	switch screeningResult.FinalResult {
	case WorthSummarizingYes:
		summary, err := b.summarizer.SummarizeDocument(htmlAndDocs, b.config.Gemini.SummerizingPrompt)
		if err != nil {
			return fmt.Errorf("failed to summarize content: %w", err)
		}

		if err := b.mastodonClient.PostSummary(*item, summary); err != nil {
			return fmt.Errorf("failed to post to mastodon: %w", err)
		}
		item.Status = StatusProcessed
		if err := b.itemRepository.Update(item); err != nil {
			return fmt.Errorf("failed to mark as posted: %w", err)
		}
	case WorthSummarizingNo:
		item.Status = StatusProcessed
		item.Reason = ReasonGeminiNotValuable
		if err := b.itemRepository.Update(item); err != nil {
			return fmt.Errorf("failed to mark as not valuable: %w", err)
		}
	case WorthSummarizingWait:
		item.Status = StatusDeferred
		item.Reason = ReasonGeminiPageNotReady
		if err := b.itemRepository.Update(item); err != nil {
			return fmt.Errorf("failed to mark as not ready: %w", err)
		}
	}

	slog.Info("Finish posting summary")
	return nil
}
