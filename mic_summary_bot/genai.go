package micsummarybot

import (
	"context"

	"google.golang.org/genai"
)

type GenAIClient struct {
	Client           *genai.Client
	MaxRetry         int
	ScreeningModel   string
	SummarizingModel string
	DownloadDir      string
	KeepLocalCopy    bool
}

// NewGenAIClient は新しいGenAIClientインスタンスを作成します。
func NewGenAIClient(gemini *GeminiConfig, storage *StorageConfig) (*GenAIClient, error) {
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey: gemini.APIKey,
	})
	if err != nil {
		return nil, err
	}

	return &GenAIClient{
		Client:           client,
		MaxRetry:         gemini.RetryCount,
		ScreeningModel:   gemini.ScreeningModel,
		SummarizingModel: gemini.SummerizingModel,
		DownloadDir:      storage.DownloadDir,
		KeepLocalCopy:    storage.KeepLocalCopy,
	}, nil
}
