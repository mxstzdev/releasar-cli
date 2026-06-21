package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const telegramBaseURL = "https://api.telegram.org"

type telegram struct {
	token   string
	chatID  string
	baseURL string
	http    *http.Client
}

func newTelegram(cfg TelegramConfig) (*telegram, error) {
	if cfg.Token == "" {
		return nil, fmt.Errorf("token is required")
	}
	if cfg.ChatID == "" {
		return nil, fmt.Errorf("chatId is required")
	}
	return &telegram{
		token:   cfg.Token,
		chatID:  cfg.ChatID,
		baseURL: telegramBaseURL,
		http:    &http.Client{},
	}, nil
}

func (t *telegram) Notify(event Event) error {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Released %s", event.Tag))
	if event.URL != "" {
		sb.WriteString("\n" + event.URL)
	}
	if event.Body != "" {
		const maxBody = 3500
		body := event.Body
		if len(body) > maxBody {
			body = body[:maxBody] + "…"
		}
		sb.WriteString("\n\n" + body)
	}

	payload := struct {
		ChatID string `json:"chat_id"`
		Text   string `json:"text"`
	}{
		ChatID: t.chatID,
		Text:   sb.String(),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling telegram payload: %w", err)
	}

	endpoint := fmt.Sprintf("%s/bot%s/sendMessage", t.baseURL, t.token)
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("building telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.http.Do(req)
	if err != nil {
		return fmt.Errorf("sending telegram message: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned %d: %s", resp.StatusCode, body)
	}
	return nil
}
