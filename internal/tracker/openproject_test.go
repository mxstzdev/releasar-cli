package tracker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestOpenProject(t *testing.T, mux *http.ServeMux) *openProject {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	op, err := newOpenProject(Config{Host: srv.URL, Token: "test-token", ProjectKey: "42"})
	require.NoError(t, err)
	return op
}

func TestOpenProject_RequiresConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantMsg string
	}{
		{
			name:    "missing project key",
			cfg:     Config{Host: "https://op.example.com", Token: "t"},
			wantMsg: "projectKey",
		},
		{
			name:    "missing host",
			cfg:     Config{Token: "t", ProjectKey: "42"},
			wantMsg: "host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newOpenProject(tt.cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantMsg)
		})
	}
}

func TestOpenProject_ListVersions(t *testing.T) {
	tests := []struct {
		name       string
		response   any
		statusCode int
		wantLen    int
		wantErr    bool
	}{
		{
			name: "returns open versions",
			response: map[string]any{
				"_embedded": map[string]any{
					"elements": []map[string]any{
						{"id": 1, "name": "v1.0.0", "status": "open"},
						{"id": 2, "name": "v2.0.0", "status": "open"},
					},
				},
			},
			statusCode: http.StatusOK,
			wantLen:    2,
		},
		{
			name: "empty",
			response: map[string]any{
				"_embedded": map[string]any{"elements": []any{}},
			},
			statusCode: http.StatusOK,
			wantLen:    0,
		},
		{
			name:       "API error",
			response:   map[string]string{"message": "Forbidden"},
			statusCode: http.StatusForbidden,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/api/v3/projects/42/versions", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			})
			op := newTestOpenProject(t, mux)

			versions, err := op.ListVersions()
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

func TestOpenProject_CreateVersion(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		response   any
		wantID     string
		wantErr    bool
	}{
		{
			name:       "creates version and returns ID",
			statusCode: http.StatusCreated,
			response:   map[string]any{"id": 7, "name": "v1.2.3"},
			wantID:     "7",
		},
		{
			name:       "API error",
			statusCode: http.StatusBadRequest,
			response:   map[string]string{"message": "bad request"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/api/v3/projects/42/versions", func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			})
			op := newTestOpenProject(t, mux)

			id, err := op.CreateVersion("v1.2.3", "desc")
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, id)
		})
	}
}

func TestOpenProject_ResolveRefs(t *testing.T) {
	tests := []struct {
		name    string
		refs    []string
		wantLen int
	}{
		{
			name:    "resolves numeric refs in single batch call",
			refs:    []string{"#1", "#2"},
			wantLen: 2,
		},
		{
			name:    "skips non-numeric refs",
			refs:    []string{"PROJ-1", "ABC-99"},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			mux := http.NewServeMux()
			mux.HandleFunc("/api/v3/work_packages", func(w http.ResponseWriter, r *http.Request) {
				calls++
				_ = json.NewEncoder(w).Encode(map[string]any{
					"_embedded": map[string]any{
						"elements": []map[string]any{
							{"id": 1, "subject": "Task one",
								"_links": map[string]any{
									"self":   map[string]any{"href": "/api/v3/work_packages/1"},
									"status": map[string]any{"title": "New"},
								}},
							{"id": 2, "subject": "Task two",
								"_links": map[string]any{
									"self":   map[string]any{"href": "/api/v3/work_packages/2"},
									"status": map[string]any{"title": "In Progress"},
								}},
						},
					},
				})
			})
			op := newTestOpenProject(t, mux)

			resolved, err := op.ResolveRefs(tt.refs)
			require.NoError(t, err)
			assert.Len(t, resolved, tt.wantLen)
			if tt.wantLen > 0 {
				assert.Equal(t, 1, calls, "expected single batch API call")
			}
		})
	}
}

func TestOpenProject_AssignTickets(t *testing.T) {
	tests := []struct {
		name    string
		refs    []string
		wantErr bool
	}{
		{name: "assigns numeric refs", refs: []string{"#1"}},
		{name: "skips non-numeric refs", refs: []string{"PROJ-1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/api/v3/work_packages/1", func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodGet:
					_ = json.NewEncoder(w).Encode(map[string]any{"id": 1, "lockVersion": 3})
				case http.MethodPatch:
					_ = json.NewEncoder(w).Encode(map[string]any{"id": 1})
				default:
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			})
			op := newTestOpenProject(t, mux)

			err := op.AssignTickets(tt.refs, "7")
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestOpenProject_AssignTickets_PatchError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/work_packages/1", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 1, "lockVersion": 0})
		case http.MethodPatch:
			w.WriteHeader(http.StatusConflict)
			fmt.Fprint(w, `{"message":"lock version mismatch"}`)
		}
	})
	op := newTestOpenProject(t, mux)

	err := op.AssignTickets([]string{"#1"}, "7")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "assigning tickets")
}

func TestOpenProject_CloseVersion(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{name: "closes version", statusCode: http.StatusOK},
		{name: "API error", statusCode: http.StatusNotFound, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/api/v3/versions/7", func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPatch, r.Method)
				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(map[string]any{})
			})
			op := newTestOpenProject(t, mux)

			err := op.CloseVersion("7")
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
