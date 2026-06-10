package versioning

import (
	"testing"

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
