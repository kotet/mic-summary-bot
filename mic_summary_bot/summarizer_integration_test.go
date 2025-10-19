//go:build integration

package micsummarybot

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/japanese"
	"google.golang.org/genai"
)

func TestSummarizeDocument(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY environment variable not set, skipping integration test.")
	}

	ctx := context.Background()
	// The screener_integration_test.go has a strange way to create a client.
	// We follow the constructor defined in genai.go
	genaiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	require.NoError(t, err)

	config := DefaultConfig()
	// In a real scenario, NewGenAIClient would be used.
	// For this test, we construct it directly to control dependencies.
	client := &GenAIClient{
		Client:           genaiClient,
		MaxRetry:         config.Gemini.RetryCount,
		RetryIntervalSec: config.Gemini.RetryIntervalSec,
		ScreeningModel:   config.Gemini.ScreeningModel,
		SummarizingModel: config.Gemini.SummarizingModel,
		DownloadDir:      t.TempDir(), // Use a temporary directory for downloads
		KeepLocalCopy:    true,        // Keep files for inspection
	}

	// --- Mock downloadFile function ---
	// Store original downloadFile function
	originalDownloadFile := downloadFile
	// Defer restoration of the original function
	t.Cleanup(func() {
		downloadFile = originalDownloadFile
	})
	// Define a mocked downloadFile that copies from a local source
	downloadFile = func(url, localPath string) error {
		// The "url" in the test case will be the local source file path
		sourceFile, err := os.Open(url)
		if err != nil {
			return fmt.Errorf("failed to open mock source file: %w", err)
		}
		defer sourceFile.Close()

		destFile, err := os.Create(localPath)
		if err != nil {
			return fmt.Errorf("failed to create mock destination file: %w", err)
		}
		defer destFile.Close()

		_, err = io.Copy(destFile, sourceFile)
		return err
	}
	// --- End Mock ---

	promptTemplate := config.Gemini.SummarizingPrompt
	require.NotEmpty(t, promptTemplate, "Summarization prompt should not be empty")

	testCases := []struct {
		name             string
		htmlBodyFilePath string
		documents        []Document // Now using the real Document struct
		expectError      bool
	}{
		{
			name:             "Summarize page with one PDF",
			htmlBodyFilePath: filepath.Join("..", "resources", "example_for_summerize", "01toukei08_01000325.htm"),
			documents: []Document{
				// For the mocked download, URL is the local file path.
				{URL: filepath.Join("..", "resources", "example_for_summerize", "001034183.pdf"), Size: 100000}, // Size can be arbitrary but > 0
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			htmlBytes, err := os.ReadFile(tc.htmlBodyFilePath)
			require.NoError(t, err, "Failed to read HTML body file: %s", tc.htmlBodyFilePath)

			shiftJISReader := japanese.ShiftJIS.NewDecoder().Reader(bytes.NewReader(htmlBytes))
			b, err := io.ReadAll(shiftJISReader)
			require.NoError(t, err, "Failed to read HTML body")

			body, err := extractContentsBody(string(b))
			require.NoError(t, err, "Failed to extract contents body")

			htmlAndDocs := &HTMLandDocuments{
				HTMLContent: []byte(body),
				Documents:   tc.documents,
			}

			result, err := client.SummarizeDocument(htmlAndDocs, promptTemplate)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, result.FinalSummary, "FinalSummary should not be empty")
				// Also check if document summary was generated
				assert.NotEmpty(t, result.Documents, "Document summaries should be generated")
				t.Logf("Generated Summary: %#v", result)
			}
		})
	}
}
