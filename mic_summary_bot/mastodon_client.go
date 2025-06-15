package micsummarybot

import (
	"context"
	"log"
	"strings"
	"text/template"

	"github.com/mattn/go-mastodon"
)

// MastodonClient はMastodonへの投稿を行うクライアントです。
type MastodonClient struct {
	client       *mastodon.Client
	postTemplate string
}

type PostInfo struct {
	Title   string
	Summary string
	URL     string
}

// NewMastodonClient は新しいMastodonClientを初期化して返します。
func NewMastodonClient(config *Config) *MastodonClient {
	client := mastodon.NewClient(&mastodon.Config{
		Server:       config.Mastodon.InstanceURL,
		ClientID:     config.Mastodon.ClientID,
		ClientSecret: config.Mastodon.ClientSecret,
		AccessToken:  config.Mastodon.AccessToken,
	})
	return &MastodonClient{
		client:       client,
		postTemplate: config.Mastodon.PostTemplate,
	}
}

// PostSummary は要約結果をMastodonに投稿します。
func (c *MastodonClient) PostSummary(task Item, summary SummerizeResult) error {
	t := template.Must(template.New("post").Parse(c.postTemplate))
	var buf strings.Builder
	err := t.Execute(&buf, PostInfo{
		Title:   task.Title,
		Summary: summary.FinalSummary,
		URL:     task.URL,
	})
	if err != nil {
		log.Printf("ERROR: テンプレート展開失敗: %v", err)
		return err
	}
	status := buf.String()

	// Mastodonの文字数制限はデフォルト5000文字のため、ここではチェックしない。
	// 投稿エラー時は当該投稿をスキップする。
	s, err := c.client.PostStatus(context.Background(), &mastodon.Toot{Status: status})
	if err != nil {
		log.Printf("ERROR: Mastodon投稿失敗: %v", err)
		return err
	}
	log.Printf("INFO: Mastodon投稿成功: %s", s.URL)
	return nil
}
