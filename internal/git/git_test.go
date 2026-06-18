package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLog(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    []Commit
		wantErr bool
	}{
		{
			name:  "empty input",
			input: []byte{},
			want:  nil,
		},
		{
			name:  "whitespace only",
			input: []byte("   \n  "),
			want:  nil,
		},
		{
			name:  "single commit without body",
			input: []byte("\x00abc123\nJohn Doe\n2024-06-01T10:00:00Z\nfeat: add feature\n"),
			want: []Commit{
				{
					Hash:      "abc123",
					Author:    "John Doe",
					Timestamp: time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
					Subject:   "feat: add feature",
				},
			},
		},
		{
			name:  "single commit with body",
			input: []byte("\x00abc123\nJohn Doe\n2024-06-01T10:00:00Z\nfeat: add feature\nBREAKING CHANGE: removed old API"),
			want: []Commit{
				{
					Hash:      "abc123",
					Author:    "John Doe",
					Timestamp: time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
					Subject:   "feat: add feature",
					Body:      "BREAKING CHANGE: removed old API",
				},
			},
		},
		{
			name:  "multiple commits",
			input: []byte("\x00abc123\nJohn Doe\n2024-06-01T10:00:00Z\nfeat: first\n\x00def456\nJane Doe\n2024-06-02T10:00:00Z\nfix: second\n"),
			want: []Commit{
				{
					Hash:      "abc123",
					Author:    "John Doe",
					Timestamp: time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
					Subject:   "feat: first",
				},
				{
					Hash:      "def456",
					Author:    "Jane Doe",
					Timestamp: time.Date(2024, 6, 2, 10, 0, 0, 0, time.UTC),
					Subject:   "fix: second",
				},
			},
		},
		{
			name:    "invalid timestamp",
			input:   []byte("\x00abc123\nJohn Doe\nnot-a-timestamp\nfeat: add\n"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLog(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLog_FirstParent(t *testing.T) {
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}

	run("init", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test User")

	pkgDir := filepath.Join(dir, "packages", "api")
	require.NoError(t, os.MkdirAll(pkgDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "init.go"), []byte("package api"), 0o644))
	run("add", ".")
	run("commit", "-m", "chore: initial commit")
	run("tag", "v0.1.0")

	// create dev branch, then add a commit on main that touches the package
	run("checkout", "-b", "dev")
	run("checkout", "main")
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "hotfix.go"), []byte("package api"), 0o644))
	run("add", ".")
	run("commit", "-m", "fix: hotfix from main")

	// merge main into dev and add a dev-native commit
	run("checkout", "dev")
	run("merge", "main", "--no-ff", "-m", "chore: merge main into dev")
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "feature.go"), []byte("package api"), 0o644))
	run("add", ".")
	run("commit", "-m", "feat: add new feature")

	c := &Client{
		rootDirectory:    dir,
		workingDirectory: pkgDir,
	}

	commits, err := c.Log("v0.1.0")
	require.NoError(t, err)

	subjects := make([]string, len(commits))
	for i, commit := range commits {
		subjects[i] = commit.Subject
	}

	assert.NotContains(t, subjects, "fix: hotfix from main", "commit from main must not appear via --first-parent")
	assert.Contains(t, subjects, "feat: add new feature")
}

func TestTagName(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		input  string
		want   string
	}{
		{"no prefix configured", "", "1.0.0", "1.0.0"},
		{"prefix applied", "myapp/", "1.0.0", "myapp/1.0.0"},
		{"prefix already present", "myapp/", "myapp/1.0.0", "myapp/1.0.0"},
		{"colon prefix applied", "myapp:", "1.0.0", "myapp:1.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{cfg: Config{TagPrefix: tt.prefix}}
			assert.Equal(t, tt.want, c.tagName(tt.input))
		})
	}
}
