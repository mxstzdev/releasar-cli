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

func newTestGitHub(t *testing.T, mux *http.ServeMux) *gitHub {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return newGitHub(Config{Host: srv.URL, Token: "test-token", Owner: "owner", Repo: "repo"}, log.Nop())
}

func TestGitHub_ListVersions(t *testing.T) {
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
				{"number": 1, "title": "v1.0.0", "state": "open"},
				{"number": 2, "title": "v2.0.0", "state": "open"},
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
			mux.HandleFunc("/repos/owner/repo/milestones", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			})
			g := newTestGitHub(t, mux)

			versions, err := g.ListVersions()
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, versions, tt.wantLen)
			if tt.wantLen > 0 {
				assert.Equal(t, "1", versions[0].ID)
				assert.Equal(t, "v1.0.0", versions[0].Name)
				assert.True(t, versions[0].Open)
			}
		})
	}
}

func TestGitHub_CreateVersion(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		response   any
		wantID     string
		wantErr    bool
	}{
		{
			name:       "creates milestone and returns number as ID",
			statusCode: http.StatusCreated,
			response:   map[string]any{"number": 42, "title": "v1.2.3"},
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
			mux.HandleFunc("/repos/owner/repo/milestones", func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			})
			g := newTestGitHub(t, mux)

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

func TestGitHub_ResolveRefs(t *testing.T) {
	tests := []struct {
		name      string
		refs      []string
		wantLen   int
		wantState string
	}{
		{
			name:      "resolves numeric refs",
			refs:      []string{"#1"},
			wantLen:   1,
			wantState: "open",
		},
		{
			name:    "skips non-GitHub refs",
			refs:    []string{"PROJ-123", "ABC-1"},
			wantLen: 0,
		},
		{
			name:    "skips not-found refs",
			refs:    []string{"#999"},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/repos/owner/repo/issues/1", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"title": "Fix bug", "state": "open", "html_url": "https://github.com/owner/repo/issues/1",
				})
			})
			mux.HandleFunc("/repos/owner/repo/issues/999", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			})
			g := newTestGitHub(t, mux)

			resolved, err := g.ResolveRefs(tt.refs)
			require.NoError(t, err)
			assert.Len(t, resolved, tt.wantLen)
			if tt.wantLen > 0 {
				assert.Equal(t, tt.wantState, resolved[0].State)
			}
		})
	}
}

func TestGitHub_AssignTickets(t *testing.T) {
	tests := []struct {
		name       string
		refs       []string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "assigns numeric refs",
			refs:       []string{"#1"},
			statusCode: http.StatusOK,
		},
		{
			name:    "skips non-GitHub refs",
			refs:    []string{"PROJ-1"},
			// no handler needed — zero HTTP calls expected
		},
		{
			name:       "collects errors across refs",
			refs:       []string{"#1"},
			statusCode: http.StatusForbidden,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/repos/owner/repo/issues/1", func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPatch, r.Method)
				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(map[string]any{})
			})
			g := newTestGitHub(t, mux)

			err := g.AssignTickets(tt.refs, "7")
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestGitHub_CloseVersion(t *testing.T) {
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
			mux.HandleFunc("/repos/owner/repo/milestones/7", func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPatch, r.Method)
				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(map[string]any{})
			})
			g := newTestGitHub(t, mux)

			err := g.CloseVersion("7")
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
