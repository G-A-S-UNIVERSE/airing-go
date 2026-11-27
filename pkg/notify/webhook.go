package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultWebhookTimeout = 5 * time.Second
	maxErrorBodyBytes     = 1024
)

// WebhookNotifier posts events as JSON to a generic HTTP webhook endpoint.
type WebhookNotifier struct {
	URL    string
	Token  string
	Client *http.Client
}

// NewWebhook returns a WebhookNotifier with a 5s HTTP client timeout.
func NewWebhook(url, token string) *WebhookNotifier {
	return &WebhookNotifier{
		URL:    strings.TrimSpace(url),
		Token:  strings.TrimSpace(token),
		Client: &http.Client{Timeout: defaultWebhookTimeout},
	}
}

// Name implements Notifier.
func (w *WebhookNotifier) Name() string { return "webhook" }

// Notify sends the event via HTTP POST with optional Bearer token auth.
func (w *WebhookNotifier) Notify(ctx context.Context, event Event) error {
	if w == nil || w.URL == "" {
		return fmt.Errorf("webhook notifier: empty URL")
	}
	client := w.Client
	if client == nil {
		client = &http.Client{Timeout: defaultWebhookTimeout}
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("webhook notifier: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook notifier: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if w.Token != "" {
		req.Header.Set("Authorization", "Bearer "+w.Token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook notifier: post: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return fmt.Errorf("webhook notifier: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return nil
}
