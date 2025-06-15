package micsummarybot

import "google.golang.org/genai"

type GenAIClient struct {
	Client         *genai.Client
	MaxRetry       int
	ScreeningModel string
}
