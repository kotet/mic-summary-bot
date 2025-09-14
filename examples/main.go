package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
	micsummarybot "github.com/kotet/mic-summary-bot/mic_summary_bot"
)

func main() {
	_ = godotenv.Load()

	// Set log level dynamically (works with default logger)
	micsummarybot.SetLogLevel(slog.LevelDebug)

	// Alternative: Use custom logger (SetLogLevel may not work)
	// handlerOptions := &slog.HandlerOptions{
	// 	Level: slog.LevelDebug,
	// }
	// logger := slog.New(slog.NewTextHandler(os.Stdout, handlerOptions))
	// micsummarybot.SetLogger(logger)

	ctx := context.Background()

	config, err := micsummarybot.LoadConfig("config.yaml")
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	bot, err := micsummarybot.NewMICSummaryBot(config)
	if err != nil {
		slog.Error("Failed to create MICSummaryBot", "error", err)
		os.Exit(1)
	}

	if len(os.Args) > 1 {
		command := os.Args[1]
		switch command {
		case "update":
			if err := bot.RefreshFeedItems(ctx); err != nil {
				slog.Error("Failed to fetch and store items", "error", err)
			}
		case "screen":
			if err := bot.ScreenItem(ctx); err != nil {
				slog.Error("Failed to screen item", "error", err)
			}
		case "post":
			if err := bot.PostSummary(ctx); err != nil {
				slog.Error("Failed to pick and post item", "error", err)
			}
		default:
			slog.Error("Unknown command", "command", command)
		}
	} else {
		if err := bot.RefreshFeedItems(ctx); err != nil {
			slog.Error("Failed to fetch and store items", "error", err)
		}

		if err := bot.ScreenItem(ctx); err != nil {
			slog.Error("Failed to screen item", "error", err)
		}

		if err := bot.PostSummary(ctx); err != nil {
			slog.Error("Failed to pick and post item", "error", err)
		}
	}
}
