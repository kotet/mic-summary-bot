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
	MaxTokens         int    `yaml:"max_tokens"`
	RetryCount        int    `yaml:"retry_count"`
	RetryIntervalSec  int    `yaml:"retry_interval_sec"`
	ScreeningModel    string `yaml:"screening_model"`
	ScreeningPrompt   string `yaml:"screening_prompt"`
	SummerizingModel  string `yaml:"summerizing_model"`
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

func DefaultConfig() *Config {
	return &Config{
		RSS: RSSConfig{
			URL: "https://www.soumu.go.jp/news.rdf",
		},
		Gemini: GeminiConfig{
			APIKey:           "",
			ScreeningModel:   "gemini-2.0-flash",
			SummerizingModel: "gemini-2.5-flash-preview-05-20",
			MaxTokens:        65535,
			RetryCount:       3,
			RetryIntervalSec: 5,
			ScreeningPrompt: `Webページの内容を見て、要約する価値があるかどうかを判断してください。

要約する価値のないWebページは以下のような特徴を持ちます。以下のいずれかの特徴を持つ場合、NOと判定してください。
1. 添付資料がない、もしくはすべて非公開等で利用できない状態にある

要約する価値のあるWebページは以下のような特徴を持ちます。この場合はYESと判定してください。
1. 適切にフォーマットされている
2. タイトルが前のドキュメントからの機械的な繰り返しではなく、人間が新たに書いたものだと思われる
3. 添付資料があり、適切に添付されている

判断を待つべきWebページは以下のような特徴を持ちます。この場合はWAITと判定してください。
1. 添付資料が準備できておらず、後日掲載などと書かれている
2. その他、時間経過により要約する価値のあるページになると考えられる記述が含まれる

Webページに含まれる添付資料 {{ len .Documents }}件:
{{ range .Documents }}
- URL: {{ .URL }}, Size: {{ .Size }} bytes{{ end }}
`,
			SummerizingPrompt: `あなたは「総務省会議議事録要約ツール」です。
  1) 会議のメタ情報を含むWebページ(HTMLテキスト)
  2) 議事録や参考資料を含むPDFファイル
を渡すため、各PDFファイルの要約を出力し、それらの内容をもとに会議全体を要約してください。

【ドキュメント要約出力形式】
- metadata: タイトル、日時、出席者、開催場所、議題などのメタ情報
- keyPoints: 重要な発言や決定事項を3~5項目列挙
- summary: 文書の要点を日本語で簡潔にまとめる

【最終要約出力形式】
- final_summary: 会議の特に重要な部分を取り上げ、ですます調、3~5文、全体で250文字程度の日本語にまとめる
`,
		},
		Mastodon: MastodonConfig{
			InstanceURL: "",
			AccessToken: "",
		},
		Storage: StorageConfig{
			DownloadDir:   "./mic_summary_bot/downloads",
			KeepLocalCopy: true,
		},
		Database: DatabaseConfig{
			Path: "./mic_summary_bot/database.sqlite",
		},
	}
}
