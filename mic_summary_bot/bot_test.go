package micsummarybot

import (
	"context"
	"strings"
	"testing"
)

// TestPanicIsRecoveredAsError は処理中のパニックがプロセスを落とさずエラーとして返ることを確認する。
// ゼロ値の MICSummaryBot は itemRepository が nil のため、最初のDBアクセスでパニックする。
func TestPanicIsRecoveredAsError(t *testing.T) {
	tests := []struct {
		functionName string
		run          func(b *MICSummaryBot) error
	}{
		{"PostSummary", func(b *MICSummaryBot) error { return b.PostSummary(context.Background()) }},
		{"ScreenItem", func(b *MICSummaryBot) error { return b.ScreenItem(context.Background()) }},
	}
	for _, tt := range tests {
		t.Run(tt.functionName, func(t *testing.T) {
			err := tt.run(&MICSummaryBot{})
			if err == nil {
				t.Fatal("expected error from recovered panic, got nil")
			}
			if !strings.Contains(err.Error(), "panic occurred in "+tt.functionName) {
				t.Errorf("unexpected error message: %v", err)
			}
		})
	}
}
