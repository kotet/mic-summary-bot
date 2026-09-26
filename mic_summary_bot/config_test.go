package micsummarybot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfigMaxDeferredRetryCount(t *testing.T) {
	tests := []struct {
		name       string
		configYAML string
		want       int
		wantErr    bool
	}{
		{name: "default", configYAML: "rss:\n  url: \"https://example.com\"\n", want: 3},
		{name: "positive", configYAML: "database:\n  max_deferred_retry_count: 1\n", want: 1},
		{name: "zero", configYAML: "database:\n  max_deferred_retry_count: 0\n", wantErr: true},
		{name: "negative", configYAML: "database:\n  max_deferred_retry_count: -1\n", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "config.yaml")
			assert.NoError(t, os.WriteFile(configPath, []byte(tt.configYAML), 0644))

			config, err := LoadConfig(configPath)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, config.Database.MaxDeferredRetryCount)
		})
	}
}
