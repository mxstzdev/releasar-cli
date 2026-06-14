package versioning

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mariusvslprts/releasar-cli/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustGitCommit(hash string) git.Commit {
	return git.Commit{Hash: hash}
}

func mustGitCommitAuthor(author string) git.Commit {
	return git.Commit{Author: author}
}

var testDate = time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)

func TestChangelogFromCommits_sections(t *testing.T) {
	tests := []struct {
		name       string
		commits    []ParsedCommit
		wantTitles []string
	}{
		{
			name:       "empty commits produces no sections",
			commits:    []ParsedCommit{},
			wantTitles: nil,
		},
		{
			name:       "feat goes to Added",
			commits:    []ParsedCommit{{Type: "feat", Description: "add login"}},
			wantTitles: []string{"Added"},
		},
		{
			name:       "fix goes to Fixed",
			commits:    []ParsedCommit{{Type: "fix", Description: "fix crash"}},
			wantTitles: []string{"Fixed"},
		},
		{
			name:       "refactor goes to Changed",
			commits:    []ParsedCommit{{Type: "refactor", Description: "simplify auth"}},
			wantTitles: []string{"Changed"},
		},
		{
			name:       "perf goes to Changed",
			commits:    []ParsedCommit{{Type: "perf", Description: "cache queries"}},
			wantTitles: []string{"Changed"},
		},
		{
			name:       "docs goes to Changed",
			commits:    []ParsedCommit{{Type: "docs", Description: "update readme"}},
			wantTitles: []string{"Changed"},
		},
		{
			name:       "chore is omitted",
			commits:    []ParsedCommit{{Type: "chore", Description: "update deps"}},
			wantTitles: nil,
		},
		{
			name:       "test is omitted",
			commits:    []ParsedCommit{{Type: "test", Description: "add unit tests"}},
			wantTitles: nil,
		},
		{
			name:       "ci is omitted",
			commits:    []ParsedCommit{{Type: "ci", Description: "add workflow"}},
			wantTitles: nil,
		},
		{
			name:       "build is omitted",
			commits:    []ParsedCommit{{Type: "build", Description: "update makefile"}},
			wantTitles: nil,
		},
		{
			name: "section order is Changed Added Removed Fixed",
			commits: []ParsedCommit{
				{Type: "fix", Description: "fix crash"},
				{Type: "feat", Description: "add login"},
				{Type: "refactor", Description: "simplify auth"},
			},
			wantTitles: []string{"Changed", "Added", "Fixed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := ChangelogFromCommits("1.0.0", testDate, tt.commits)
			var titles []string
			for _, s := range entry.Sections {
				titles = append(titles, s.Title)
			}
			assert.Equal(t, tt.wantTitles, titles)
		})
	}
}

func TestChangelogFromCommits_breakingSort(t *testing.T) {
	commits := []ParsedCommit{
		{Type: "feat", Description: "add oauth"},
		{Type: "feat", Description: "drop legacy API", Breaking: true},
	}
	entry := ChangelogFromCommits("2.0.0", testDate, commits)
	require.Len(t, entry.Sections, 1)
	assert.Equal(t, "Added", entry.Sections[0].Title)
	assert.Contains(t, entry.Sections[0].Items[0], "**Breaking:**")
	assert.Contains(t, entry.Sections[0].Items[1], "add oauth")
}

func TestFormatChangelogItem(t *testing.T) {
	tests := []struct {
		name string
		c    ParsedCommit
		want string
	}{
		{
			name: "plain description",
			c:    ParsedCommit{Description: "fix crash"},
			want: "fix crash",
		},
		{
			name: "with scope",
			c:    ParsedCommit{Scope: "auth", Description: "fix token expiry"},
			want: "**auth:** fix token expiry",
		},
		{
			name: "breaking without scope",
			c:    ParsedCommit{Breaking: true, Description: "remove legacy API"},
			want: "**Breaking:** remove legacy API",
		},
		{
			name: "breaking with scope",
			c:    ParsedCommit{Breaking: true, Scope: "auth", Description: "rename login endpoint"},
			want: "**auth (breaking):** rename login endpoint",
		},
		{
			name: "with refs",
			c:    ParsedCommit{Description: "fix crash", Refs: []string{"#42"}},
			want: "fix crash (#42)",
		},
		{
			name: "with hash from embedded Commit",
			c:    ParsedCommit{Description: "fix crash", Commit: mustGitCommit("abc1234")},
			want: "fix crash (`abc1234`)",
		},
		{
			name: "with refs and hash",
			c:    ParsedCommit{Description: "fix crash", Refs: []string{"#42"}, Commit: mustGitCommit("abc1234")},
			want: "fix crash (#42, `abc1234`)",
		},
		{
			name: "with author from embedded Commit",
			c:    ParsedCommit{Description: "fix crash", Commit: mustGitCommitAuthor("Alice")},
			want: "fix crash (Alice)",
		},
		{
			name: "refs and author",
			c:    ParsedCommit{Description: "fix crash", Refs: []string{"#42"}, Commit: mustGitCommitAuthor("Alice")},
			want: "fix crash (#42) (Alice)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatChangelogItem(tt.c))
		})
	}
}

func TestEntry_Render(t *testing.T) {
	entry := Entry{
		Version: "1.2.0",
		Date:    testDate,
		Sections: []Section{
			{Title: "Added", Items: []string{"add login", "add logout"}},
			{Title: "Fixed", Items: []string{"fix crash"}},
		},
	}

	want := "## [1.2.0] - 2026-06-13\n\n### Added\n\n- add login\n- add logout\n\n### Fixed\n\n- fix crash\n"
	assert.Equal(t, want, entry.Render())
}

func TestEntry_Render_empty(t *testing.T) {
	entry := Entry{Version: "1.0.0", Date: testDate}
	assert.Equal(t, "## [1.0.0] - 2026-06-13\n", entry.Render())
}

func TestBuildChangelogContent(t *testing.T) {
	rendered := "## [1.1.0] - 2026-06-13\n\n### Added\n\n- add login\n"

	tests := []struct {
		name     string
		existing string
		want     string
	}{
		{
			name:     "empty file",
			existing: "",
			want:     "# Changelog\n\n" + rendered + "\n",
		},
		{
			name:     "header only",
			existing: "# Changelog",
			want:     "# Changelog\n\n" + rendered + "\n",
		},
		{
			name:     "header with existing entry",
			existing: "# Changelog\n\n## [1.0.0] - 2026-01-01\n\n### Added\n\n- initial release\n",
			want:     "# Changelog\n\n" + rendered + "\n## [1.0.0] - 2026-01-01\n\n### Added\n\n- initial release\n",
		},
		{
			name:     "no header prepends one",
			existing: "## [1.0.0] - 2026-01-01\n\n### Added\n\n- initial release\n",
			want:     "# Changelog\n\n" + rendered + "\n## [1.0.0] - 2026-01-01\n\n### Added\n\n- initial release\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, buildChangelogContent(tt.existing, rendered))
		})
	}
}

func TestPrependChangelog(t *testing.T) {
	entry := Entry{
		Version: "1.1.0",
		Date:    testDate,
		Sections: []Section{
			{Title: "Added", Items: []string{"add login"}},
		},
	}

	t.Run("creates file when absent", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "CHANGELOG.md")
		require.NoError(t, PrependChangelog(path, entry))
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Contains(t, string(data), "# Changelog")
		assert.Contains(t, string(data), "## [1.1.0] - 2026-06-13")
		assert.Contains(t, string(data), "- add login")
	})

	t.Run("prepends to existing file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "CHANGELOG.md")
		existing := "# Changelog\n\n## [1.0.0] - 2026-01-01\n\n### Added\n\n- initial release\n"
		require.NoError(t, os.WriteFile(path, []byte(existing), 0o644))

		require.NoError(t, PrependChangelog(path, entry))

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		content := string(data)

		assert.Contains(t, content, "## [1.1.0] - 2026-06-13")
		assert.Contains(t, content, "## [1.0.0] - 2026-01-01")
		assert.Less(t, strings.Index(content, "1.1.0"), strings.Index(content, "1.0.0"))
	})

	t.Run("preserves original file permissions", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "CHANGELOG.md")
		require.NoError(t, os.WriteFile(path, []byte("# Changelog\n"), 0o640))

		require.NoError(t, PrependChangelog(path, entry))

		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o640), info.Mode())
	})

	t.Run("returns error on unwritable path", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "subdir", "CHANGELOG.md")
		err := PrependChangelog(path, entry)
		assert.Error(t, err)
	})
}
