package micsummarybot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"text/template"
	"time"

	"github.com/google/uuid"
	"google.golang.org/genai"
)

type DocumentSummary struct {
	Summary   string   `json:"summary"`
	Metadata  string   `json:"metadata"`
	KeyPoints []string `json:"keyPoints"`
}

type SummarizeResult struct {
	Documents    []DocumentSummary `json:"documents"`
	FirstSummary string            `json:"first_summary"`
	Omissibles   []string          `json:"omissibles"`
	MissedItems  []string          `json:"missed_items"`
	FinalSummary string            `json:"final_summary"`
}

const (
	// MaxDocumentSize represents the maximum file size (50MB) for documents
	// that can be uploaded to Gemini API. This limitation is imposed by
	// Google's Gemini API to ensure reasonable processing times and resource usage.
	// Documents larger than this size will be skipped during summarization.
	MaxDocumentSize = 50 * 1024 * 1024 // 50MB in bytes
)

// getPartMetadata extracts metadata from a genai.Part for safe logging
func getPartMetadata(part *genai.Part) map[string]interface{} {
	metadata := make(map[string]interface{})

	if part == nil {
		metadata["type"] = "nil"
		return metadata
	}

	// Check if part has InlineData (HTML content, file data)
	if part.InlineData != nil {
		metadata["type"] = "inline_data"
		metadata["mime_type"] = part.InlineData.MIMEType
		if part.InlineData.Data != nil {
			metadata["data_size"] = len(part.InlineData.Data)
		}
		return metadata
	}

	// Check if part has FileData (uploaded file reference)
	if part.FileData != nil {
		metadata["type"] = "file_data"
		metadata["mime_type"] = part.FileData.MIMEType
		metadata["file_uri"] = part.FileData.FileURI
		return metadata
	}

	// Check if part has Text
	if part.Text != "" {
		metadata["type"] = "text"
		metadata["text_length"] = len(part.Text)
		// Only show first 100 characters of text for debugging
		if len(part.Text) > 100 {
			metadata["text_preview"] = part.Text[:100] + "..."
		} else {
			metadata["text_preview"] = part.Text
		}
		return metadata
	}

	metadata["type"] = "unknown"
	return metadata
}

// downloadFile は指定されたURLからファイルをダウンロードし、指定されたローカルパスに保存します。
var downloadFile = func(url string, filepath string) error {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

// SummarizeDocument はHTMLandDocumentsを要約します。
func (client *GenAIClient) SummarizeDocument(htmlAndDocs *HTMLandDocuments, promptTemplate string) (SummarizeResult, error) {
	ctx := context.Background()

	pkgLogger.Info("Starting document summarization process")

	// モデル設定（構造化出力）
	modelConfig := &genai.GenerateContentConfig{
		Temperature:      new(float32), // 0
		ResponseMIMEType: "application/json",
		ResponseSchema: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"documents": {
					Type: genai.TypeArray,
					Items: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"metadata": {
								Type: genai.TypeString,
							},
							"keyPoints": {
								Type: genai.TypeArray,
								Items: &genai.Schema{
									Type: genai.TypeString,
								},
							},
							"summary": {
								Type: genai.TypeString,
							},
						},
						PropertyOrdering: []string{"metadata", "keyPoints", "summary"},
					},
				},
				"first_summary": {
					Type: genai.TypeString,
				},
				"omissibles": {
					Type: genai.TypeArray,
					Items: &genai.Schema{
						Type: genai.TypeString,
					},
				},
				"missed_items": {
					Type: genai.TypeArray,
					Items: &genai.Schema{
						Type: genai.TypeString,
					},
				},
				"final_summary": {
					Type: genai.TypeString,
				},
			},
			PropertyOrdering: []string{"documents", "omissibles", "final_summary"},
			Required:         []string{"final_summary"},
		},
	}

	pkgLogger.Debug("Parsing prompt template")
	t, err := template.New("prompt").Parse(promptTemplate)
	if err != nil {
		pkgLogger.Error("Failed to parse prompt template", "error", err)
		return SummarizeResult{}, fmt.Errorf("failed to parse prompt template: %w", err)
	}
	promptBuilder := &strings.Builder{}
	err = t.Execute(promptBuilder, struct{}{})
	if err != nil {
		pkgLogger.Error("Failed to execute prompt template", "error", err)
		return SummarizeResult{}, fmt.Errorf("failed to execute prompt template: %w", err)
	}
	prompt := promptBuilder.String()
	if prompt == "" {
		pkgLogger.Error("Generated prompt is empty")
		return SummarizeResult{}, fmt.Errorf("prompt is empty")
	}
	pkgLogger.Debug("Prompt template processed successfully")

	parts := []*genai.Part{}
	parts = append(parts, &genai.Part{
		InlineData: &genai.Blob{
			MIMEType: "text/html",
			Data:     htmlAndDocs.HTMLContent,
		},
	})
	pkgLogger.Debug("Added HTML content to parts")

	pkgLogger.Debug("Creating download directory", "path", client.DownloadDir)
	err = os.MkdirAll(client.DownloadDir, 0755)
	if err != nil {
		pkgLogger.Error("Failed to create download directory", "path", client.DownloadDir, "error", err)
		return SummarizeResult{}, fmt.Errorf("failed to create download directory: %w", err)
	}

	pkgLogger.Info("Processing documents for download", "count", len(htmlAndDocs.Documents))
	for i, doc := range htmlAndDocs.Documents {
		pkgLogger.Debug("Processing document", "index", i, "url", doc.URL, "size", doc.Size)
		if doc.Size > MaxDocumentSize {
			pkgLogger.Debug("Skipping document due to size limit", "url", doc.URL, "size", doc.Size, "max_size", MaxDocumentSize)
			continue
		}
		id := uuid.New().String()
		ext := path.Ext(doc.URL)
		ext = strings.ToLower(ext)
		if ext != ".pdf" {
			pkgLogger.Debug("Skipping non-PDF document", "url", doc.URL, "extension", ext)
			continue
		}
		localPath := path.Join(client.DownloadDir, fmt.Sprintf("%s.pdf", id))
		pkgLogger.Info("Downloading PDF file", "url", doc.URL, "local_path", localPath)
		err = downloadFile(doc.URL, localPath)
		if err != nil {
			pkgLogger.Error("Failed to download file", "url", doc.URL, "local_path", localPath, "error", err)
			return SummarizeResult{}, fmt.Errorf("failed to download file: %w", err)
		}
		pkgLogger.Debug("Uploading file to Gemini", "local_path", localPath)
		f, err := client.Client.Files.UploadFromPath(ctx, localPath, &genai.UploadFileConfig{})
		if err != nil {
			pkgLogger.Error("Failed to upload file to Gemini", "local_path", localPath, "error", err)
			return SummarizeResult{}, fmt.Errorf("failed to upload file: %w", err)
		}
		pkgLogger.Debug("File uploaded to Gemini successfully", "uri", f.URI, "mime_type", f.MIMEType)
		parts = append(parts, genai.NewPartFromURI(f.URI, f.MIMEType))
		if !client.KeepLocalCopy {
			pkgLogger.Debug("Removing local copy", "local_path", localPath)
			os.Remove(localPath)
		}
	}

	parts = append(parts, genai.NewPartFromText(prompt))
	pkgLogger.Debug("Added prompt text to parts", "total_parts", len(parts))

	for i, part := range parts {
		metadata := getPartMetadata(part)
		pkgLogger.Debug("part created", "index", i, "metadata", metadata)
	}

	contents := []*genai.Content{genai.NewContentFromParts(parts, genai.RoleUser)}

	// LLMへのリクエストとリトライ処理
	pkgLogger.Info("Starting Gemini API calls with retry", "max_retry", client.MaxRetry+1)
	var resp *genai.GenerateContentResponse
	for i := 0; i < client.MaxRetry+1; i++ {
		pkgLogger.Debug("Calling Gemini API", "attempt", i+1, "model", client.SummarizingModel)
		resp, err = client.Client.Models.GenerateContent(ctx, client.SummarizingModel, contents, modelConfig)
		if err == nil {
			pkgLogger.Debug("Gemini API call succeeded", "attempt", i+1)
			break // 成功したらループを抜ける
		}
		pkgLogger.Warn("Gemini API call failed", "attempt", i+1, "max_retry", client.MaxRetry+1, "error", err, "retrying_in_seconds", client.RetryIntervalSec)
		time.Sleep(time.Duration(client.RetryIntervalSec) * time.Second)
	}

	if err != nil {
		pkgLogger.Error("All Gemini API calls failed", "total_attempts", client.MaxRetry+1, "error", err)
		return SummarizeResult{}, fmt.Errorf("failed to get response from Gemini API after %d retries: %w", client.MaxRetry, err)
	}

	// レスポンスのパース
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		pkgLogger.Error("No content generated by Gemini API")
		return SummarizeResult{}, fmt.Errorf("no content generated by Gemini API")
	}

	// LLMの応答をテキストとして取得
	responseText := resp.Text()

	pkgLogger.Debug("Gemini API response", "response", responseText)
	pkgLogger.Debug("Parsing JSON response from Gemini API")

	var jsonResult SummarizeResult
	err = json.Unmarshal([]byte(responseText), &jsonResult)
	if err != nil {
		pkgLogger.Error("Failed to parse JSON response", "response", responseText, "error", err)
		return SummarizeResult{}, fmt.Errorf("failed to parse JSON response from Gemini API: %w", err)
	}

	pkgLogger.Debug("Document summarization completed successfully")
	return jsonResult, nil
}
