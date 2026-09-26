package micsummarybot

import (
	"strings"
	"testing"

	"google.golang.org/genai"
)

// TestValidateGeneratedContent は生成コンテンツが欠けたレスポンスをパニックせずエラーとして扱い、
// 原因の調査に使える情報をエラーに含めることを確認する。
func TestValidateGeneratedContent(t *testing.T) {
	tests := []struct {
		name        string
		resp        *genai.GenerateContentResponse
		wantErrPart string
	}{
		{
			name: "valid content",
			resp: &genai.GenerateContentResponse{Candidates: []*genai.Candidate{
				{Content: genai.NewContentFromText("ok", genai.RoleModel)},
			}},
		},
		{
			name:        "no candidates",
			resp:        &genai.GenerateContentResponse{},
			wantErrPart: "no candidates generated",
		},
		{
			name: "no candidates with block reason",
			resp: &genai.GenerateContentResponse{
				PromptFeedback: &genai.GenerateContentResponsePromptFeedback{BlockReason: genai.BlockedReasonSafety},
			},
			wantErrPart: "block_reason=SAFETY",
		},
		{
			name: "nil content",
			resp: &genai.GenerateContentResponse{Candidates: []*genai.Candidate{
				{FinishReason: genai.FinishReasonSafety},
			}},
			wantErrPart: "finish_reason=SAFETY",
		},
		{
			name: "empty parts",
			resp: &genai.GenerateContentResponse{Candidates: []*genai.Candidate{
				{Content: &genai.Content{}, FinishReason: genai.FinishReasonMaxTokens},
			}},
			wantErrPart: "finish_reason=MAX_TOKENS",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGeneratedContent(tt.resp)
			if tt.wantErrPart == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErrPart) {
				t.Errorf("expected error containing %q, got %v", tt.wantErrPart, err)
			}
		})
	}
}
