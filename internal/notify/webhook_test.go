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

func newTestWebhook(t *testing.T, mux *http.ServeMux, headers map[string]string) *webhook {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	w, err := newWebhook(WebhookConfig{URL: srv.URL + "/hook", Headers: headers})
	require.NoError(t, err)
	return w
}

func TestWebhook_Notify(t *testing.T) {
	tests := []struct {
		name       string
		event      Event
		statusCode int
		wantErr    bool
	}{
		{
			name:       "succeeds on 200",
			event:      Event{Type: EventReleaseComplete, Tag: "v1.0.0", Version: "1.0.0"},
			statusCode: http.StatusOK,
		},
		{
			name:       "succeeds on 204",
			event:      Event{Type: EventReleaseComplete, Tag: "v1.0.0", Version: "1.0.0"},
			statusCode: http.StatusNoContent,
		},
		{
			name:       "returns error on 4xx",
			event:      Event{Type: EventReleaseComplete, Tag: "v1.0.0"},
			statusCode: http.StatusUnauthorized,
			wantErr:    true,
		},
		{
			name:       "returns error on 5xx",
			event:      Event{Type: EventReleaseComplete, Tag: "v1.0.0"},
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/hook", func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				w.WriteHeader(tt.statusCode)
			})
			wh := newTestWebhook(t, mux, nil)

			err := wh.Notify(tt.event)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestWebhook_Notify_Payload(t *testing.T) {
	var captured map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/hook", func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &captured)
		w.WriteHeader(http.StatusOK)
	})
	wh := newTestWebhook(t, mux, nil)

	event := Event{Type: EventReleaseComplete, Tag: "v2.0.0", Version: "2.0.0", URL: "https://example.com/release", Body: "changelog"}
	require.NoError(t, wh.Notify(event))

	assert.Equal(t, "release_complete", captured["type"])
	assert.Equal(t, "v2.0.0", captured["tag"])
	assert.Equal(t, "2.0.0", captured["version"])
	assert.Equal(t, "https://example.com/release", captured["url"])
	assert.Equal(t, "changelog", captured["body"])
}

func TestWebhook_Notify_SendsCustomHeaders(t *testing.T) {
	var authHeader string
	mux := http.NewServeMux()
	mux.HandleFunc("/hook", func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	})
	wh := newTestWebhook(t, mux, map[string]string{"Authorization": "Bearer secret"})

	require.NoError(t, wh.Notify(Event{Type: EventReleaseComplete, Tag: "v1.0.0", Version: "1.0.0"}))
	assert.Equal(t, "Bearer secret", authHeader)
}
