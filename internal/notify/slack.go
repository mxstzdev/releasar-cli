package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type slack struct {
	webhookURL string
	http       *http.Client
}

func newSlack(cfg SlackConfig) (*slack, error) {
	if cfg.WebhookURL == "" {
		return nil, fmt.Errorf("webhook URL is required")
	}
	return &slack{webhookURL: cfg.WebhookURL, http: &http.Client{}}, nil
}

func (s *slack) Notify(event Event) error {
	blocks := []map[string]any{
		{
			"type": "header",
			"text": map[string]any{
				"type": "plain_text",
				"text": fmt.Sprintf("Released %s", event.Tag),
			},
		},
	}

	if event.Body != "" {
		excerpt := event.Body
		const maxLen = 2900
		if len(excerpt) > maxLen {
			excerpt = excerpt[:maxLen] + "…"
		}
		blocks = append(blocks, map[string]any{
			"type": "section",
			"text": map[string]any{
				"type": "mrkdwn",
				"text": excerpt,
			},
		})
	}

	if event.URL != "" {
		blocks = append(blocks, map[string]any{
			"type": "actions",
			"elements": []map[string]any{
				{
					"type": "button",
					"text": map[string]any{
						"type": "plain_text",
						"text": "View Release",
					},
					"url": event.URL,
				},
			},
		})
	}

	payload := map[string]any{"blocks": blocks}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling slack payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, s.webhookURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("building slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("sending slack message: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack webhook returned %d: %s", resp.StatusCode, body)
	}
	return nil
}
