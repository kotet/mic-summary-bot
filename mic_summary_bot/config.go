package micsummarybot

import (
	"os"

	"gopkg.in/yaml.v2"
)

// Config は Bot の設定情報を保持する
type Config struct {
	RSS      RSSConfig      `yaml:"rss"`
	Gemini   GeminiConfig   `yaml:"gemini"`
	Mastodon MastodonConfig `yaml:"mastodon"`
	Storage  StorageConfig  `yaml:"storage"`
	Database DatabaseConfig `yaml:"database"`
}

type RSSConfig struct {
	URL string `yaml:"url"`
}

type GeminiConfig struct {
	APIKey            string `yaml:"api_key"`
	Model             string `yaml:"model"`
	MaxTokens         int    `yaml:"max_tokens"`
	RetryCount        int    `yaml:"retry_count"`
	RetryIntervalSec  int    `yaml:"retry_interval_sec"`
	ScreeningPrompt   string `yaml:"screening_prompt"`
	SummerizingPrompt string `yaml:"summizing_prompt"`
}

type MastodonConfig struct {
	InstanceURL string `yaml:"instance_url"`
	AccessToken string `yaml:"access_token"`
}

type StorageConfig struct {
	DownloadDir   string `yaml:"download_dir"`
	KeepLocalCopy bool   `yaml:"keep_local_copy"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"`
}

// LoadConfig は指定されたパスから設定ファイルを読み込み、Config構造体にパースします。
func LoadConfig(configPath string) (*Config, error) {
	configYAML, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(configYAML, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
