package tracker

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mxstzdev/releasar-cli/internal/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestGitea(t *testing.T, mux *http.ServeMux) *giteaTracker {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	g, err := newGitea(Config{Host: srv.URL, Token: "test-token", Owner: "owner", Repo: "repo"}, "", log.Nop())
	require.NoError(t, err)
	return g
}

func TestGitea_ListVersions(t *testing.T) {
	tests := []struct {
		name       string
		response   any
		statusCode int
		wantLen    int
		wantErr    bool
	}{
		{
			name: "returns open milestones",
			response: []map[string]any{
				{"id": 1, "title": "v1.0.0", "closed": false},
				{"id": 2, "title": "v2.0.0", "closed": false},
			},
			statusCode: http.StatusOK,
			wantLen:    2,
		},
		{
			name:       "empty list",
			response:   []any{},
			statusCode: http.StatusOK,
			wantLen:    0,
		},
		{
			name:       "API error",
			response:   map[string]string{"message": "Not Found"},
			statusCode: http.StatusNotFound,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/api/v1/repos/owner/repo/milestones", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			})
			g := newTestGitea(t, mux)

			versions, err := g.ListVersions()
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, versions, tt.wantLen)
			if tt.wantLen > 0 {
				assert.Equal(t, "1", versions[0].ID)
				assert.True(t, versions[0].Open)
			}
		})
	}
}

func TestGitea_CreateVersion(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		response   any
		wantID     string
		wantErr    bool
	}{
		{
			name:       "creates milestone and returns ID",
			statusCode: http.StatusCreated,
			response:   map[string]any{"id": 42, "title": "v1.2.3"},
			wantID:     "42",
		},
		{
			name:       "API error",
			statusCode: http.StatusUnprocessableEntity,
			response:   map[string]string{"message": "Validation failed"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/api/v1/repos/owner/repo/milestones", func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			})
			g := newTestGitea(t, mux)

			id, err := g.CreateVersion("v1.2.3", "desc")
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, id)
		})
	}
}

func TestGitea_ResolveRefs(t *testing.T) {
	tests := []struct {
		name    string
		refs    []string
		wantLen int
	}{
		{name: "resolves numeric refs", refs: []string{"#1"}, wantLen: 1},
		{name: "skips non-Gitea refs", refs: []string{"PROJ-1", "ABC-99"}, wantLen: 0},
		{name: "skips not-found refs", refs: []string{"#999"}, wantLen: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/api/v1/repos/owner/repo/issues/1", func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"title": "Fix bug", "state": "open", "html_url": "https://gitea.example.com/owner/repo/issues/1",
				})
			})
			mux.HandleFunc("/api/v1/repos/owner/repo/issues/999", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			})
			g := newTestGitea(t, mux)

			resolved, err := g.ResolveRefs(tt.refs)
			require.NoError(t, err)
			assert.Len(t, resolved, tt.wantLen)
		})
	}
}

func TestGitea_AssignTickets(t *testing.T) {
	tests := []struct {
		name       string
		refs       []string
		statusCode int
		wantErr    bool
	}{
		{name: "assigns numeric refs", refs: []string{"#1"}, statusCode: http.StatusCreated},
		{name: "skips non-Gitea refs", refs: []string{"PROJ-1"}},
		{name: "collects errors", refs: []string{"#1"}, statusCode: http.StatusForbidden, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/api/v1/repos/owner/repo/issues/1", func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPatch, r.Method)
				w.WriteHeader(tt.statusCode)
			})
			g := newTestGitea(t, mux)

			err := g.AssignTickets(tt.refs, "5")
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestGitea_CloseVersion(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{name: "closes milestone", statusCode: http.StatusOK},
		{name: "API error", statusCode: http.StatusNotFound, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/api/v1/repos/owner/repo/milestones/5", func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPatch, r.Method)
				w.WriteHeader(tt.statusCode)
			})
			g := newTestGitea(t, mux)

			err := g.CloseVersion("5")
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestGitea_RequiresHost(t *testing.T) {
	_, err := newGitea(Config{Token: "tok", Owner: "o", Repo: "r"}, "", log.Nop())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tracker.host is required")
}
