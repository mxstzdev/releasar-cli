package notify

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mxstzdev/releasar-cli/internal/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestTelegram(t *testing.T, mux *http.ServeMux) *telegram {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	tg, err := newTelegram(TelegramConfig{Token: "test-token", ChatID: "-100123"}, log.Nop())
	require.NoError(t, err)
	tg.baseURL = srv.URL
	return tg
}

func TestTelegram_Notify(t *testing.T) {
	tests := []struct {
		name       string
		event      Event
		statusCode int
		wantErr    bool
		checkBody  func(t *testing.T, body map[string]any)
	}{
		{
			name:       "sends message with tag and URL",
			event:      Event{Type: EventReleaseComplete, Tag: "v1.2.3", Version: "1.2.3", URL: "https://github.com/org/repo/releases/tag/v1.2.3"},
			statusCode: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]any) {
				t.Helper()
				text, _ := body["text"].(string)
				assert.Contains(t, text, "v1.2.3")
				assert.Contains(t, text, "https://github.com/org/repo/releases/tag/v1.2.3")
				assert.Equal(t, "-100123", body["chat_id"])
			},
		},
		{
			name:       "sends message without URL",
			event:      Event{Type: EventReleaseComplete, Tag: "v2.0.0", Version: "2.0.0"},
			statusCode: http.StatusOK,
			checkBody: func(t *testing.T, body map[string]any) {
				t.Helper()
				assert.Contains(t, body["text"], "v2.0.0")
			},
		},
		{
			name:       "returns error on non-200 response",
			event:      Event{Type: EventReleaseComplete, Tag: "v1.0.0"},
			statusCode: http.StatusUnauthorized,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/bottest-token/sendMessage", func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(`{"ok":true}`))
			})
			tg := newTestTelegram(t, mux)

			err := tg.Notify(tt.event)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestTelegram_Notify_SendsCorrectPayload(t *testing.T) {
	var captured map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/bottest-token/sendMessage", func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &captured)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	tg := newTestTelegram(t, mux)

	event := Event{Type: EventReleaseComplete, Tag: "v1.0.0", Version: "1.0.0", URL: "https://example.com/release"}
	require.NoError(t, tg.Notify(event))

	assert.Equal(t, "-100123", captured["chat_id"])
	text, _ := captured["text"].(string)
	assert.Contains(t, text, "v1.0.0")
	assert.Contains(t, text, "https://example.com/release")
}
