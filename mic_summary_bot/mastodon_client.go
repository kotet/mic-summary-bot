package micsummarybot

import (
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/mattn/go-mastodon"
)

// MastodonClient is a client for posting to Mastodon.
type MastodonClient struct {
	client   *mastodon.Client
	template *template.Template
}

type PostInfo struct {
	Title   string
	Summary string
	URL     string
}

// NewMastodonClient initializes and returns a new MastodonClient.
func NewMastodonClient(config *Config) (*MastodonClient, error) {
	client := mastodon.NewClient(&mastodon.Config{
		Server:       config.Mastodon.InstanceURL,
		ClientID:     config.Mastodon.ClientID,
		ClientSecret: config.Mastodon.ClientSecret,
		AccessToken:  config.Mastodon.AccessToken,
	})

	t, err := template.New("post").Parse(config.Mastodon.PostTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse post template: %w", err)
	}

	return &MastodonClient{
		client:   client,
		template: t,
	}, nil
}

// PostSummary posts the summary result to Mastodon.
func (c *MastodonClient) PostSummary(task Item, summary SummerizeResult) error {
	var buf strings.Builder
	err := c.template.Execute(&buf, PostInfo{
		Title:   task.Title,
		Summary: summary.FinalSummary,
		URL:     task.URL,
	})
	if err != nil {
		pkgLogger.Error("Failed to execute template", "error", err)
		return err
	}
	status := buf.String()

	// The character limit for Mastodon is 5000 characters by default, so we don't check it here.
	// On posting error, the current post is skipped.
	s, err := c.client.PostStatus(context.Background(), &mastodon.Toot{Status: status})
	if err != nil {
		pkgLogger.Error("Failed to post to Mastodon", "error", err)
		return err
	}
	pkgLogger.Info("Successfully posted to Mastodon", "url", s.URL)
	return nil
}
