package versioning

import (
	"testing"
	"time"

	"github.com/mxstzdev/releasar-cli/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		subject string
		body    string
		want    ParsedCommit
		wantErr bool
	}{
		{
			name:    "feat with scope",
			subject: "feat(auth): added OAuth2 support",
			want:    ParsedCommit{Type: "feat", Scope: "auth", Description: "added OAuth2 support"},
		},
		{
			name:    "fix without scope",
			subject: "fix: corrected nil pointer in parser",
			want:    ParsedCommit{Type: "fix", Scope: "", Description: "corrected nil pointer in parser"},
		},
		{
			name:    "breaking change via exclamation mark",
			subject: "feat(api)!: removed deprecated endpoints",
			want:    ParsedCommit{Type: "feat", Scope: "api", Description: "removed deprecated endpoints", Breaking: true},
		},
		{
			name:    "breaking change via body footer",
			subject: "feat: changed response format",
			body:    "BREAKING CHANGE: response is now JSON instead of XML",
			want:    ParsedCommit{Type: "feat", Scope: "", Description: "changed response format", Breaking: true},
		},
		{
			name:    "breaking change via both exclamation and footer",
			subject: "feat!: overhauled config format",
			body:    "BREAKING CHANGE: config keys renamed",
			want:    ParsedCommit{Type: "feat", Scope: "", Description: "overhauled config format", Breaking: true},
		},
		{
			name:    "chore without scope",
			subject: "chore: updated dependencies",
			want:    ParsedCommit{Type: "chore", Scope: "", Description: "updated dependencies"},
		},
		{
			name:    "leading whitespace stripped",
			subject: "  fix: trimmed subject",
			want:    ParsedCommit{Type: "fix", Scope: "", Description: "trimmed subject"},
		},
		{
			name:    "non-conventional commit",
			subject: "updated stuff",
			wantErr: true,
		},
		{
			name:    "empty subject",
			subject: "",
			wantErr: true,
		},
		{
			name:    "missing description",
			subject: "feat: ",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.subject, tt.body)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParse_refs(t *testing.T) {
	tests := []struct {
		name        string
		subject     string
		body        string
		wantDesc    string
		wantRefs    []string
	}{
		{
			name:     "single ref in subject",
			subject:  "feat: add login (#123)",
			wantDesc: "add login",
			wantRefs: []string{"#123"},
		},
		{
			name:     "multiple refs in subject",
			subject:  "fix: patch crash (#123, #456)",
			wantDesc: "patch crash",
			wantRefs: []string{"#123", "#456"},
		},
		{
			name:     "jira ref in subject",
			subject:  "feat: add export (JIRA-789)",
			wantDesc: "add export",
			wantRefs: []string{"JIRA-789"},
		},
		{
			name:     "non-ref trailing parens kept in description",
			subject:  "refactor: replace cache (LRU)",
			wantDesc: "replace cache (LRU)",
			wantRefs: nil,
		},
		{
			name:     "ref from Closes footer",
			subject:  "fix: handle nil pointer",
			body:     "Closes: #99",
			wantDesc: "handle nil pointer",
			wantRefs: []string{"#99"},
		},
		{
			name:     "ref from Fixes footer without colon",
			subject:  "fix: handle nil pointer",
			body:     "Fixes #42",
			wantDesc: "handle nil pointer",
			wantRefs: []string{"#42"},
		},
		{
			name:     "multiple refs from Ref footer",
			subject:  "feat: batch export",
			body:     "Ref: #10, #11",
			wantDesc: "batch export",
			wantRefs: []string{"#10", "#11"},
		},
		{
			name:     "refs from subject and body combined",
			subject:  "feat: add login (#5)",
			body:     "Closes: #6",
			wantDesc: "add login",
			wantRefs: []string{"#5", "#6"},
		},
		{
			name:     "no refs",
			subject:  "feat: add login",
			wantDesc: "add login",
			wantRefs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.subject, tt.body)
			require.NoError(t, err)
			assert.Equal(t, tt.wantDesc, got.Description)
			assert.Equal(t, tt.wantRefs, got.Refs)
		})
	}
}

func TestParseCommit(t *testing.T) {
	ts := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)

	t.Run("conventional commit parsed correctly", func(t *testing.T) {
		c := git.Commit{Hash: "abc123", Author: "Alice", Timestamp: ts, Subject: "feat(auth): add login (#42)"}
		got := ParseCommit(c)
		assert.Equal(t, "feat", got.Type)
		assert.Equal(t, "auth", got.Scope)
		assert.Equal(t, "add login", got.Description)
		assert.Equal(t, []string{"#42"}, got.Refs)
		assert.Equal(t, c, got.Commit)
	})

	t.Run("non-conventional commit retains subject as description", func(t *testing.T) {
		c := git.Commit{Hash: "def456", Author: "Bob", Timestamp: ts, Subject: "updated stuff"}
		got := ParseCommit(c)
		assert.Equal(t, "", got.Type)
		assert.Equal(t, "updated stuff", got.Description)
		assert.Equal(t, c, got.Commit)
	})

	t.Run("breaking commit detected", func(t *testing.T) {
		c := git.Commit{Subject: "feat!: drop v1 API"}
		got := ParseCommit(c)
		assert.True(t, got.Breaking)
	})
}

func TestParseCommits(t *testing.T) {
	commits := []git.Commit{
		{Subject: "feat: add login"},
		{Subject: "not conventional"},
		{Subject: "fix: patch crash"},
	}
	got := ParseCommits(commits)
	require.Len(t, got, 3)
	assert.Equal(t, "feat", got[0].Type)
	assert.Equal(t, "", got[1].Type)
	assert.Equal(t, "fix", got[2].Type)
}

func TestRecommend(t *testing.T) {
	tests := []struct {
		name    string
		commits []ParsedCommit
		want    BumpKind
	}{
		{
			name:    "empty slice returns none",
			commits: []ParsedCommit{},
			want:    BumpNone,
		},
		{
			name:    "only chores return patch",
			commits: []ParsedCommit{{Type: "chore"}, {Type: "docs"}},
			want:    BumpPatch,
		},
		{
			name:    "feat returns minor",
			commits: []ParsedCommit{{Type: "fix"}, {Type: "feat"}},
			want:    BumpMinor,
		},
		{
			name:    "breaking change returns major",
			commits: []ParsedCommit{{Type: "feat"}, {Type: "fix", Breaking: true}},
			want:    BumpMajor,
		},
		{
			name:    "breaking exclamation returns major",
			commits: []ParsedCommit{{Type: "feat", Breaking: true}},
			want:    BumpMajor,
		},
		{
			name:    "unknown type returns none",
			commits: []ParsedCommit{{Type: "unknown"}},
			want:    BumpNone,
		},
		{
			name:    "highest bump wins",
			commits: []ParsedCommit{{Type: "chore"}, {Type: "feat"}, {Breaking: true}},
			want:    BumpMajor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Recommend(tt.commits))
		})
	}
}
