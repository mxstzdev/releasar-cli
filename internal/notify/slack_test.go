package notify

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSlack(t *testing.T, mux *http.ServeMux) (*slack, string) {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	webhookURL := srv.URL + "/webhook"
	s, err := newSlack(SlackConfig{WebhookURL: webhookURL})
	require.NoError(t, err)
	return s, webhookURL
}

func TestSlack_Notify(t *testing.T) {
	tests := []struct {
		name       string
		event      Event
		statusCode int
		wantErr    bool
	}{
		{
			name:       "succeeds on 200",
			event:      Event{Type: EventReleaseComplete, Tag: "v1.0.0", Version: "1.0.0", Body: "## Changes\n- feat: added thing"},
			statusCode: http.StatusOK,
		},
		{
			name:       "returns error on non-200",
			event:      Event{Type: EventReleaseComplete, Tag: "v1.0.0"},
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				w.WriteHeader(tt.statusCode)
			})
			s, _ := newTestSlack(t, mux)

			err := s.Notify(tt.event)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSlack_Notify_BlockKitPayload(t *testing.T) {
	var captured map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &captured)
		w.WriteHeader(http.StatusOK)
	})
	s, _ := newTestSlack(t, mux)

	event := Event{
		Type:    EventReleaseComplete,
		Tag:     "v1.2.3",
		Version: "1.2.3",
		Body:    "## Changes\n- feat: added thing",
		URL:     "https://example.com/release",
	}
	require.NoError(t, s.Notify(event))

	blocks, ok := captured["blocks"].([]any)
	require.True(t, ok, "payload must contain blocks array")
	assert.GreaterOrEqual(t, len(blocks), 3, "header + section + actions blocks expected")

	header := blocks[0].(map[string]any)
	assert.Equal(t, "header", header["type"])
	headerText := header["text"].(map[string]any)
	assert.Contains(t, headerText["text"], "v1.2.3")
}
