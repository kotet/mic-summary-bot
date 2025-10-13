package micsummarybot

import (
	"context"
	"fmt"
	"runtime/debug"
)

// handlePanic is a helper function for consistent panic handling
func handlePanic(functionName string) error {
	if r := recover(); r != nil {
		stack := string(debug.Stack())
		pkgLogger.Error("Panic occurred",
			"function", functionName,
			"panic", r,
			"stack_trace", stack)
		return fmt.Errorf("panic occurred in %s: %v", functionName, r)
	}
	return nil
}

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

// setItemToDeferred はアイテムをDeferredステータスに更新するヘルパー関数
// エラー処理の共通化により、コードの重複を避け、保守性を向上させる
func (b *MICSummaryBot) setItemToDeferred(ctx context.Context, item *Item, reason ItemReasonCode, originalErr error, logMsg string) {
	pkgLogger.Error(logMsg, "url", item.URL, "error", originalErr)
	item.Status = StatusDeferred
	item.Reason = reason
	item.RetryCount++
	if updateErr := b.itemRepository.Update(ctx, item); updateErr != nil {
		pkgLogger.Error("Failed to update item status after processing error", "url", item.URL, "original_error_context", logMsg, "update_error", updateErr)
	}
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

func (b *MICSummaryBot) PostSummary(ctx context.Context) (err error) {
	defer func() {
		if panicErr := handlePanic("PostSummary"); panicErr != nil {
			err = panicErr
		}
	}()

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

	pkgLogger.Debug("Starting HTML parsing", "url", item.URL)
	htmlAndDocs, err := GetHTMLSummary(item.URL)
	if err != nil {
		b.setItemToDeferred(ctx, item, ReasonDownloadFailed, err, "Failed to parse HTML")
		return fmt.Errorf("failed to parse html: %w", err)
	}
	pkgLogger.Debug("HTML parsing completed successfully", "url", item.URL)

	pkgLogger.Debug("Starting document summarization", "url", item.URL)
	summary, err := b.genAIClient.SummarizeDocument(htmlAndDocs, b.config.Gemini.SummarizingPrompt)
	if err != nil {
		b.setItemToDeferred(ctx, item, ReasonAPIFailed, err, "Failed to summarize content")
		return fmt.Errorf("failed to summarize content: %w", err)
	}
	pkgLogger.Debug("Document summarization completed", "url", item.URL)

	pkgLogger.Debug("Starting Mastodon post", "url", item.URL)
	if err := b.mastodonClient.PostSummary(ctx, *item, summary); err != nil {
		b.setItemToDeferred(ctx, item, ReasonAPIFailed, err, "Failed to post to Mastodon")
		return fmt.Errorf("failed to post to mastodon: %w", err)
	}
	pkgLogger.Debug("Mastodon post completed successfully", "url", item.URL)

	pkgLogger.Debug("Updating item status", "url", item.URL)
	item.Status = StatusProcessed
	item.Reason = ReasonNone
	if err := b.itemRepository.Update(ctx, item); err != nil {
		pkgLogger.Error("Failed to update item status", "url", item.URL, "error", err)
		return fmt.Errorf("failed to mark as posted: %w", err)
	}
	pkgLogger.Debug("Item status updated successfully", "url", item.URL)

	pkgLogger.Info("Finish posting summary")
	return nil
}

func (b *MICSummaryBot) ScreenItem(ctx context.Context) (err error) {
	defer func() {
		if panicErr := handlePanic("ScreenItem"); panicErr != nil {
			err = panicErr
		}
	}()

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
		b.setItemToDeferred(ctx, item, ReasonDownloadFailed, err, "Failed to parse HTML")
		return fmt.Errorf("failed to parse html: %w", err)
	}

	screeningResult, err := b.genAIClient.IsWorthSummarizing(htmlAndDocs, b.config.Gemini.ScreeningPrompt)
	if err != nil {
		b.setItemToDeferred(ctx, item, ReasonAPIFailed, err, "Failed to screen item")
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
		if err := b.itemRepository.Update(ctx, item); err != nil {
			return fmt.Errorf("failed to mark as not valuable: %w", err)
		}
		if err := b.mastodonClient.PostNoValue(ctx, *item); err != nil {
			return fmt.Errorf("failed to post no value message to mastodon: %w", err)
		}
	case WorthSummarizingWait:
		item.Status = StatusDeferred
		item.Reason = ReasonGeminiPageNotReady
		item.RetryCount++
		if err := b.itemRepository.Update(ctx, item); err != nil {
			return fmt.Errorf("failed to mark as not ready: %w", err)
		}
	}

	return nil
}
