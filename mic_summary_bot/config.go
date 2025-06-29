package micsummarybot

import (
	"os"

	"gopkg.in/yaml.v2"

	_ "embed"
)

//go:embed config.example.yaml
var exampleConfig string

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
	MaxTokens         int    `yaml:"max_tokens"`
	RetryCount        int    `yaml:"retry_count"`
	RetryIntervalSec  int    `yaml:"retry_interval_sec"`
	ScreeningModel    string `yaml:"screening_model"`
	ScreeningPrompt   string `yaml:"screening_prompt"`
	SummarizingModel  string `yaml:"summarizing_model"`
	SummarizingPrompt string `yaml:"summarizing_prompt"`
}

type MastodonConfig struct {
	InstanceURL  string `yaml:"instance_url"`
	AccessToken  string `yaml:"access_token"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	PostTemplate string `yaml:"post_template"`
}

type StorageConfig struct {
	DownloadDir   string `yaml:"download_dir"`
	KeepLocalCopy bool   `yaml:"keep_local_copy"`
}

type DatabaseConfig struct {
	Path                string `yaml:"path"`
	MaxDeferredRetryCount int    `yaml:"max_deferred_retry_count"`
}

// LoadConfig は指定されたパスから設定ファイルを読み込み、Config構造体にパースします。記述されていない項目はデフォルト値が使われます
func LoadConfig(configPath string) (*Config, error) {
	configYAML, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	config := DefaultConfig()
	err = yaml.Unmarshal(configYAML, config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func DefaultConfig() *Config {
	var config Config
	err := yaml.Unmarshal([]byte(exampleConfig), &config)
	if err != nil {
		panic(err)
	}
	return &config
}
