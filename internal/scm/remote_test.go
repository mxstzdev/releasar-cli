package scm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRemoteURL(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "SSH with .git suffix",
			input:     "git@github.com:octocat/hello-world.git",
			wantOwner: "octocat",
			wantRepo:  "hello-world",
		},
		{
			name:      "SSH without .git suffix",
			input:     "git@github.com:octocat/hello-world",
			wantOwner: "octocat",
			wantRepo:  "hello-world",
		},
		{
			name:      "HTTPS with .git suffix",
			input:     "https://github.com/octocat/hello-world.git",
			wantOwner: "octocat",
			wantRepo:  "hello-world",
		},
		{
			name:      "HTTPS without .git suffix",
			input:     "https://github.com/octocat/hello-world",
			wantOwner: "octocat",
			wantRepo:  "hello-world",
		},
		{
			name:      "self-hosted HTTPS",
			input:     "https://git.example.com/myorg/myproject.git",
			wantOwner: "myorg",
			wantRepo:  "myproject",
		},
		{
			name:      "self-hosted SSH",
			input:     "git@git.example.com:myorg/myproject.git",
			wantOwner: "myorg",
			wantRepo:  "myproject",
		},
		{
			name:    "SSH missing colon",
			input:   "git@github.com/octocat/repo.git",
			wantErr: true,
		},
		{
			name:    "path with only one segment",
			input:   "https://github.com/octocat",
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := ParseRemoteURL(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantOwner, owner)
			assert.Equal(t, tt.wantRepo, repo)
		})
	}
}
