package micsummarybot

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"google.golang.org/genai"
)

type ScreeningDecision string

const (
	WorthSummarizingYes  ScreeningDecision = "YES"
	WorthSummarizingNo   ScreeningDecision = "NO"
	WorthSummarizingWait ScreeningDecision = "WAIT"
)

type ScreeningResult struct {
	Criteria []struct {
		Name   string            `json:"name"`
		Result ScreeningDecision `json:"result"`
	} `json:"criteria"`
	FinalResult ScreeningDecision `json:"final_result"`
}

// IsWorthSummarizing はHTMLandDocumentsが要約する価値のあるものか判定します。
func (client *GenAIClient) IsWorthSummarizing(htmlAndDocs *HTMLandDocuments, promptTemplate string) (*ScreeningResult, error) {
	ctx := context.Background()

	// モデル設定（構造化出力用）
	modelConfig := &genai.GenerateContentConfig{
		Temperature:      new(float32), // 0
		ResponseMIMEType: "application/json",
		ResponseSchema: &genai.Schema{
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"criteria": {
					Type: genai.TypeArray,
					Items: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"name": {
								Type: genai.TypeString,
							},
							"thoughts": {
								Type: genai.TypeString,
							},
							"result": {
								Type: genai.TypeString,
								Enum: []string{"YES", "NO", "WAIT"},
							},
						},
						PropertyOrdering: []string{"name", "thoughts", "result"},
					},
				},
				"final_result": {
					Type: genai.TypeString,
					Enum: []string{"YES", "NO", "WAIT"},
				},
			},
			PropertyOrdering: []string{"criteria", "final_result"},
			Required:         []string{"final_result"},
		},
	}

	t, err := template.New("prompt").Parse(promptTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse prompt template: %w", err)
	}
	promptBuilder := &strings.Builder{}
	err = t.Execute(promptBuilder, htmlAndDocs)
	if err != nil {
		return nil, fmt.Errorf("failed to execute prompt template: %w", err)
	}
	prompt := promptBuilder.String()

	parts := []*genai.Part{}
	parts = append(parts, &genai.Part{
		InlineData: &genai.Blob{
			MIMEType: "text/html",
			Data:     htmlAndDocs.HTMLContent,
		},
	})
	parts = append(parts, genai.NewPartFromText(prompt))
	contents := []*genai.Content{genai.NewContentFromParts(parts, genai.RoleUser)}

	// Gemini APIはレートリミットが厳しいため、失敗してもその場でリトライせずエラーを返す。
	// 呼び出し元がアイテムを deferred にし、次回以降の実行で再処理する
	resp, err := client.Client.Models.GenerateContent(ctx, client.ScreeningModel, contents, modelConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get response from Gemini API: %w", err)
	}

	// レスポンスのパース
	if err := validateGeneratedContent(resp); err != nil {
		return nil, err
	}

	// LLMの応答をJSONとして取得し、パース
	responseText := resp.Text()

	var jsonResult ScreeningResult
	err = json.Unmarshal([]byte(responseText), &jsonResult)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON response from Gemini API: %w", err)
	}

	return &jsonResult, nil
}
