package micsummarybot

import (
	"context"
	"log"
	"strings"
	"text/template"

	"github.com/mattn/go-mastodon"
)

// MastodonClient is a client for posting to Mastodon.
type MastodonClient struct {
	client       *mastodon.Client
	postTemplate string
}

type PostInfo struct {
	Title   string
	Summary string
	URL     string
}

// NewMastodonClient initializes and returns a new MastodonClient.
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

// PostSummary posts the summary result to Mastodon.
func (c *MastodonClient) PostSummary(task Item, summary SummerizeResult) error {
	t := template.Must(template.New("post").Parse(c.postTemplate))
	var buf strings.Builder
	err := t.Execute(&buf, PostInfo{
		Title:   task.Title,
		Summary: summary.FinalSummary,
		URL:     task.URL,
	})
	if err != nil {
		log.Printf("ERROR: Failed to execute template: %v", err)
		return err
	}
	status := buf.String()

	// The character limit for Mastodon is 5000 characters by default, so we don't check it here.
	// On posting error, the current post is skipped.
	s, err := c.client.PostStatus(context.Background(), &mastodon.Toot{Status: status})
	if err != nil {
		log.Printf("ERROR: Failed to post to Mastodon: %v", err)
		return err
	}
	log.Printf("INFO: Successfully posted to Mastodon: %s", s.URL)
	return nil
}
