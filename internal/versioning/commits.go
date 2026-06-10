package versioning

import (
	"fmt"
	"regexp"
	"strings"
)

// BumpKind represents the level of version increment derived from commit analysis.
type BumpKind int

const (
	BumpNone  BumpKind = iota
	BumpPatch BumpKind = iota
	BumpMinor BumpKind = iota
	BumpMajor BumpKind = iota
)

// ParsedCommit is the result of applying the Conventional Commits spec to a commit message.
type ParsedCommit struct {
	Type        string
	Scope       string
	Description string
	Breaking    bool
}

var conventionalCommitPattern = regexp.MustCompile(`^(\w+)(\(([^)]+)\))?(!)?: (.+)$`)

// Parse parses a Conventional Commits subject line and optional body.
// Breaking changes are detected from either a ! in the subject or a BREAKING CHANGE: footer in the body.
func Parse(subject, body string) (ParsedCommit, error) {
	m := conventionalCommitPattern.FindStringSubmatch(strings.TrimSpace(subject))
	if m == nil {
		return ParsedCommit{}, fmt.Errorf("parsing commit %q: does not follow Conventional Commits format", subject)
	}

	breaking := m[4] == "!" || strings.Contains(body, "BREAKING CHANGE:")

	return ParsedCommit{
		Type:        m[1],
		Scope:       m[3],
		Description: m[5],
		Breaking:    breaking,
	}, nil
}

// Recommend returns the highest BumpKind required by the given commits.
func Recommend(commits []ParsedCommit) BumpKind {
	bump := BumpNone
	for _, c := range commits {
		if b := bumpForCommit(c); b > bump {
			bump = b
		}
	}
	return bump
}

func bumpForCommit(c ParsedCommit) BumpKind {
	if c.Breaking {
		return BumpMajor
	}
	switch c.Type {
	case "feat":
		return BumpMinor
	case "fix", "perf", "refactor", "docs", "style", "test", "chore", "ci", "build", "revert":
		return BumpPatch
	default:
		return BumpNone
	}
}
