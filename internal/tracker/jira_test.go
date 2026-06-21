package tracker

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestJira(t *testing.T, mux *http.ServeMux) *jira {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	j, err := newJira(Config{
		Host:       srv.URL,
		Token:      "test-token",
		Email:      "user@example.com",
		ProjectKey: "PROJ",
	})
	require.NoError(t, err)
	return j
}

func TestJira_RequiresConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantMsg string
	}{
		{
			name:    "missing project key",
			cfg:     Config{Host: "https://x.atlassian.net", Token: "t", Email: "e@x.com"},
			wantMsg: "projectKey",
		},
		{
			name:    "missing email",
			cfg:     Config{Host: "https://x.atlassian.net", Token: "t", ProjectKey: "PROJ"},
			wantMsg: "email",
		},
		{
			name:    "missing host",
			cfg:     Config{Token: "t", Email: "e@x.com", ProjectKey: "PROJ"},
			wantMsg: "host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newJira(tt.cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantMsg)
		})
	}
}

func TestJira_ListVersions(t *testing.T) {
	tests := []struct {
		name       string
		response   any
		statusCode int
		wantLen    int
		wantErr    bool
	}{
		{
			name: "returns unreleased versions",
			response: map[string]any{
				"values": []map[string]any{
					{"id": "10001", "name": "v1.0.0", "released": false},
					{"id": "10002", "name": "v2.0.0", "released": false},
				},
			},
			statusCode: http.StatusOK,
			wantLen:    2,
		},
		{
			name:       "empty",
			response:   map[string]any{"values": []any{}},
			statusCode: http.StatusOK,
			wantLen:    0,
		},
		{
			name:       "API error",
			response:   map[string]string{"errorMessages": "Forbidden"},
			statusCode: http.StatusForbidden,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/rest/api/3/project/PROJ/version", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			})
			j := newTestJira(t, mux)

			versions, err := j.ListVersions()
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, versions, tt.wantLen)
			if tt.wantLen > 0 {
				assert.Equal(t, "10001", versions[0].ID)
				assert.True(t, versions[0].Open)
			}
		})
	}
}

func TestJira_CreateVersion(t *testing.T) {
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
			response:   map[string]any{"id": "10003", "name": "v1.2.3"},
			wantID:     "10003",
		},
		{
			name:       "API error",
			statusCode: http.StatusBadRequest,
			response:   map[string]string{"errorMessages": "bad request"},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/rest/api/3/version", func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(tt.response)
			})
			j := newTestJira(t, mux)

			id, err := j.CreateVersion("v1.2.3", "desc")
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, id)
		})
	}
}

func TestJira_ResolveRefs(t *testing.T) {
	twoIssues := []map[string]any{
		{"key": "PROJ-1", "self": "https://x.atlassian.net/rest/api/3/issue/PROJ-1",
			"fields": map[string]any{"summary": "Bug one", "status": map[string]any{"name": "Open"}}},
		{"key": "PROJ-2", "self": "https://x.atlassian.net/rest/api/3/issue/PROJ-2",
			"fields": map[string]any{"summary": "Bug two", "status": map[string]any{"name": "In Progress"}}},
	}
	oneIssue := []map[string]any{twoIssues[0]}

	tests := []struct {
		name        string
		refs        []string
		serverIssues []map[string]any
		wantLen     int
	}{
		{
			name:        "resolves Jira-style refs in single batch call",
			refs:        []string{"PROJ-1", "PROJ-2"},
			serverIssues: twoIssues,
			wantLen:     2,
		},
		{
			name:    "skips numeric-only refs",
			refs:    []string{"#123", "#456"},
			wantLen: 0,
		},
		{
			// Jira server returns only what matches the JQL (just PROJ-1 was queried)
			name:        "mixed refs: only Jira ones resolved",
			refs:        []string{"PROJ-1", "#99"},
			serverIssues: oneIssue,
			wantLen:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			mux := http.NewServeMux()
			mux.HandleFunc("/rest/api/3/search", func(w http.ResponseWriter, r *http.Request) {
				calls++
				assert.Equal(t, http.MethodPost, r.Method)
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"issues": tt.serverIssues})
			})
			j := newTestJira(t, mux)

			resolved, err := j.ResolveRefs(tt.refs)
			require.NoError(t, err)
			assert.Len(t, resolved, tt.wantLen)
			if tt.wantLen > 0 {
				assert.Equal(t, 1, calls, "expected single batch API call")
			}
		})
	}
}

func TestJira_AssignTickets(t *testing.T) {
	tests := []struct {
		name       string
		refs       []string
		statusCode int
		wantErr    bool
	}{
		{name: "assigns Jira refs", refs: []string{"PROJ-1"}, statusCode: http.StatusNoContent},
		{name: "skips numeric refs", refs: []string{"#1"}},
		{name: "collects errors", refs: []string{"PROJ-1"}, statusCode: http.StatusForbidden, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/rest/api/3/issue/PROJ-1", func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPut, r.Method)
				w.WriteHeader(tt.statusCode)
			})
			j := newTestJira(t, mux)

			err := j.AssignTickets(tt.refs, "10001")
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestJira_CloseVersion(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{name: "releases version", statusCode: http.StatusOK},
		{name: "API error", statusCode: http.StatusNotFound, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/rest/api/3/version/10001", func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPut, r.Method)
				w.WriteHeader(tt.statusCode)
				_ = json.NewEncoder(w).Encode(map[string]any{})
			})
			j := newTestJira(t, mux)

			err := j.CloseVersion("10001")
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}
