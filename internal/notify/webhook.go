package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type webhook struct {
	url     string
	headers map[string]string
	http    *http.Client
}

func newWebhook(cfg WebhookConfig) (*webhook, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("url is required")
	}
	return &webhook{
		url:     cfg.URL,
		headers: cfg.Headers,
		http:    &http.Client{},
	}, nil
}

func (w *webhook) Notify(event Event) error {
	payload := struct {
		Type    string `json:"type"`
		Tag     string `json:"tag"`
		Version string `json:"version"`
		URL     string `json:"url,omitempty"`
		Body    string `json:"body,omitempty"`
	}{
		Type:    string(event.Type),
		Tag:     event.Tag,
		Version: event.Version,
		URL:     event.URL,
		Body:    event.Body,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling webhook payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, w.url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("building webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range w.headers {
		req.Header.Set(k, v)
	}

	resp, err := w.http.Do(req)
	if err != nil {
		return fmt.Errorf("sending webhook: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned %d: %s", resp.StatusCode, body)
	}
	return nil
}
