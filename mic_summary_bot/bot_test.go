package micsummarybot

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMarkItemDeferred は retry_count が上限に達するまでは deferred、達したら processed (ReasonRetryLimitExceeded) になることを確認する。
func TestMarkItemDeferred(t *testing.T) {
	tests := []struct {
		name           string
		retryCount     int
		wantRetryCount int
		wantStatus     ItemStatus
		wantReason     ItemReasonCode
	}{
		{"below limit", 0, 1, StatusDeferred, ReasonAPIFailed},
		{"just below limit", 1, 2, StatusDeferred, ReasonAPIFailed},
		{"reaches limit", 2, 3, StatusProcessed, ReasonRetryLimitExceeded},
		{"already over limit", 5, 6, StatusProcessed, ReasonRetryLimitExceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := &Item{Status: StatusUnprocessed, RetryCount: tt.retryCount}
			markItemDeferred(item, ReasonAPIFailed, 3)
			assert.Equal(t, tt.wantRetryCount, item.RetryCount)
			assert.Equal(t, tt.wantStatus, item.Status)
			assert.Equal(t, tt.wantReason, item.Reason)
		})
	}
}

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
