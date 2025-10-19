//go:build integration

package micsummarybot

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/japanese"
	"google.golang.org/genai"
)

func TestIsWorthSummarizing(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY environment variable not set, skipping integration test.")
	}

	ctx := context.Background()
	genaiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	require.NoError(t, err, "Failed to create GenAI client")

	config := DefaultConfig()
	client := &GenAIClient{
		Client:         genaiClient,
		MaxRetry:       config.Gemini.RetryCount,
		ScreeningModel: config.Gemini.ScreeningModel,
	}

	// Use the default prompt from config
	defaultConfig := DefaultConfig()
	promptTemplate := defaultConfig.Gemini.ScreeningPrompt
	require.NotEmpty(t, promptTemplate, "Screening prompt should not be empty")

	testCases := []struct {
		name             string
		htmlBodyFilePath string
		documents        []Document
		expectedResponse ScreeningDecision
		expectError      bool
	}{
		{
			name:             "Page with attachments",
			htmlBodyFilePath: filepath.Join("..", "resources/example_only_pdf.htm"),
			documents: []Document{
				{URL: "https://www.soumu.go.jp/main_content/001014168.pdf", Size: 444019},
				{URL: "https://www.soumu.go.jp/main_content/001014169.pdf", Size: 199732},
				{URL: "https://www.soumu.go.jp/main_content/001014170.pdf", Size: 2057400},
			},
			expectedResponse: WorthSummarizingYes,
			expectError:      false,
		},
		{
			name:             "Page with no attachments",
			htmlBodyFilePath: filepath.Join("..", "resources/example_no_pdf.htm"),
			documents:        []Document{},
			expectedResponse: WorthSummarizingNo,
			expectError:      false,
		},
		{
			name:             "Page with attachments, but not be ready. waiting for attachments",
			htmlBodyFilePath: filepath.Join("..", "resources/example_not_ready.htm"),
			documents: []Document{
				{URL: "https://www.soumu.go.jp/main_content/001014570.pdf", Size: 660518},
				{URL: "https://www.soumu.go.jp/main_content/001014571.pdf", Size: 915693},
				{URL: "https://www.soumu.go.jp/main_content/001014572.pdf", Size: 428836},
				{URL: "https://www.soumu.go.jp/main_content/001014943.pdf", Size: 170943},
			},
			expectedResponse: WorthSummarizingWait,
			expectError:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			htmlBytes, err := os.ReadFile(tc.htmlBodyFilePath)
			require.NoError(t, err, "Failed to read HTML body file: %s", tc.htmlBodyFilePath)

			// read as shift-jis
			shiftJISReader := japanese.ShiftJIS.NewDecoder().Reader(bytes.NewReader(htmlBytes))

			b, err := io.ReadAll(shiftJISReader)
			require.NoError(t, err, "Failed to read HTML body")

			body, err := extractContentsBody(string(b))
			require.NoError(t, err, "Failed to extract contents body")

			htmlBodyContent := []byte(body)

			htmlAndDocs := &HTMLandDocuments{
				HTMLContent: htmlBodyContent,
				Documents:   tc.documents,
			}

			response, err := client.IsWorthSummarizing(htmlAndDocs, promptTemplate)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedResponse, response.FinalResult, "Unexpected screening result: %#v", response)
			}
		})
	}
}
