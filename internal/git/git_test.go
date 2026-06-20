package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/mariusvslprts/releasar-cli/internal/log"
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
			input: []byte("\x00abc123\nJohn Doe\njohn@example.com\n2024-06-01T10:00:00Z\nfeat: add feature\n"),
			want: []Commit{
				{
					Hash:        "abc123",
					Author:      "John Doe",
					AuthorEmail: "john@example.com",
					Timestamp:   time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
					Subject:     "feat: add feature",
				},
			},
		},
		{
			name:  "single commit with body",
			input: []byte("\x00abc123\nJohn Doe\njohn@example.com\n2024-06-01T10:00:00Z\nfeat: add feature\nBREAKING CHANGE: removed old API"),
			want: []Commit{
				{
					Hash:        "abc123",
					Author:      "John Doe",
					AuthorEmail: "john@example.com",
					Timestamp:   time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
					Subject:     "feat: add feature",
					Body:        "BREAKING CHANGE: removed old API",
				},
			},
		},
		{
			name:  "multiple commits",
			input: []byte("\x00abc123\nJohn Doe\njohn@example.com\n2024-06-01T10:00:00Z\nfeat: first\n\x00def456\nJane Doe\njane@example.com\n2024-06-02T10:00:00Z\nfix: second\n"),
			want: []Commit{
				{
					Hash:        "abc123",
					Author:      "John Doe",
					AuthorEmail: "john@example.com",
					Timestamp:   time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
					Subject:     "feat: first",
				},
				{
					Hash:        "def456",
					Author:      "Jane Doe",
					AuthorEmail: "jane@example.com",
					Timestamp:   time.Date(2024, 6, 2, 10, 0, 0, 0, time.UTC),
					Subject:     "fix: second",
				},
			},
		},
		{
			name:    "invalid timestamp",
			input:   []byte("\x00abc123\nJohn Doe\njohn@example.com\nnot-a-timestamp\nfeat: add\n"),
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

// setupRepo initialises a temporary git repository with one commit and returns the directory,
// a run helper for raw git commands, and a fully initialised Client.
func setupRepo(t *testing.T) (dir string, run func(...string), c *Client) {
	t.Helper()
	dir = t.TempDir()
	run = func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test User")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0o644))
	run("add", ".")
	run("commit", "-m", "initial commit")

	c, err := New(dir, Config{DefaultBranch: "main", TagPrefix: "v"}, log.Get("test"))
	require.NoError(t, err)
	return
}

func TestRevParse(t *testing.T) {
	dir, _, c := setupRepo(t)

	t.Run("show-toplevel returns repo root", func(t *testing.T) {
		// Resolve symlinks: on macOS t.TempDir() returns /var/… but git resolves to /private/var/…
		resolved, err := filepath.EvalSymlinks(dir)
		require.NoError(t, err)
		out, err := c.RevParse("--show-toplevel")
		require.NoError(t, err)
		assert.Equal(t, resolved, out)
	})

	t.Run("abbrev-ref HEAD returns current branch", func(t *testing.T) {
		out, err := c.RevParse("--abbrev-ref", "HEAD")
		require.NoError(t, err)
		assert.Equal(t, "main", out)
	})
}

func TestReset(t *testing.T) {
	dir, run, c := setupRepo(t)

	initialSHA, err := c.RevParse("HEAD")
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "extra.txt"), []byte("extra"), 0o644))
	run("add", ".")
	run("commit", "-m", "second commit")

	require.NoError(t, c.Reset(initialSHA, "--hard"))

	head, err := c.RevParse("HEAD")
	require.NoError(t, err)
	assert.Equal(t, initialSHA, head)
}

func TestDeleteBranch(t *testing.T) {
	_, run, c := setupRepo(t)
	run("branch", "feature/test")

	require.NoError(t, c.DeleteBranch("feature/test"))

	err := exec.Command("git", "-C", c.rootDirectory, "rev-parse", "--verify", "feature/test").Run()
	assert.Error(t, err, "branch should no longer exist")
}

func TestDeleteTag(t *testing.T) {
	_, run, c := setupRepo(t)
	run("tag", "-a", "v1.0.0", "-m", "release")

	require.NoError(t, c.DeleteTag("1.0.0"))

	err := exec.Command("git", "-C", c.rootDirectory, "rev-parse", "--verify", "refs/tags/v1.0.0").Run()
	assert.Error(t, err, "tag should no longer exist")
}

func TestRemotes(t *testing.T) {
	t.Run("no remotes returns empty slice", func(t *testing.T) {
		_, _, c := setupRepo(t)
		remotes, err := c.Remotes()
		require.NoError(t, err)
		assert.Empty(t, remotes)
	})

	t.Run("single remote", func(t *testing.T) {
		_, run, c := setupRepo(t)
		run("remote", "add", "origin", "https://github.com/example/repo.git")
		remotes, err := c.Remotes()
		require.NoError(t, err)
		assert.Equal(t, []string{"origin"}, remotes)
	})

	t.Run("multiple remotes", func(t *testing.T) {
		_, run, c := setupRepo(t)
		run("remote", "add", "origin", "https://github.com/example/repo.git")
		run("remote", "add", "upstream", "https://github.com/upstream/repo.git")
		remotes, err := c.Remotes()
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"origin", "upstream"}, remotes)
	})
}

func TestConflictedFilesAndCheckout(t *testing.T) {
	dir, run, c := setupRepo(t)

	// Create conflicting changes on a feature branch.
	run("checkout", "-b", "feature")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("feature change"), 0o644))
	run("add", ".")
	run("commit", "-m", "feat: change file")

	// Make a conflicting change on main.
	run("checkout", "main")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("main change"), 0o644))
	run("add", ".")
	run("commit", "-m", "fix: change file on main")

	// Trigger merge conflict (expected to fail).
	cmd := exec.Command("git", "merge", "feature")
	cmd.Dir = dir
	_ = cmd.Run()

	conflicted, err := c.ConflictedFiles()
	require.NoError(t, err)
	assert.Equal(t, []string{"file.txt"}, conflicted)

	// Resolve with --ours (main's version).
	require.NoError(t, c.Checkout("--ours", "file.txt"))
	require.NoError(t, c.Add("file.txt"))

	content, err := os.ReadFile(filepath.Join(dir, "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "main change", string(content))
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
