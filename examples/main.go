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

	if err := bot.UpdateItems(ctx); err != nil {
		slog.Error("Failed to fetch and store items", "error", err)
	}

	if err := bot.PostSummary(ctx); err != nil {
		slog.Error("Failed to pick and post item", "error", err)
	}
}
