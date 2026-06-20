package versioning

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mxstzdev/releasar-cli/internal/git"
)

// BumpKind represents the level of version increment derived from commit analysis.
type BumpKind int

const (
	BumpNone  BumpKind = iota
	BumpPatch BumpKind = iota
	BumpMinor BumpKind = iota
	BumpMajor BumpKind = iota
)

// ParsedCommit extends git.Commit with fields derived from the Conventional Commits spec.
type ParsedCommit struct {
	git.Commit
	Type        string
	Scope       string
	Description string
	Breaking    bool
	// Issue or PR references extracted from subject or body (e.g. "#123", "JIRA-456").
	// May also be appended by the scm layer when a PR number is known.
	Refs []string
}

var (
	ccPattern        = regexp.MustCompile(`^(\w+)(\(([^)]+)\))?(!)?: (.+)$`)
	trailingRefGroup = regexp.MustCompile(`\s+\(([^)]+)\)$`)
	refToken         = regexp.MustCompile(`^(?:#\d+|[A-Z][A-Z0-9-]*-\d+)$`)
	footerRefLine    = regexp.MustCompile(`(?i)^(?:closes?|fixes?|resolves?|refs?):?\s+(.+)$`)
	inlineRef        = regexp.MustCompile(`#\d+|[A-Z][A-Z0-9-]*-\d+`)
)

// Parse parses a Conventional Commits subject line and optional body.
// Breaking changes are detected from a ! in the subject or a BREAKING CHANGE: footer in the body.
// Issue/PR references are extracted from a trailing (ref, ...) group in the subject or from
// Closes/Fixes/Resolves/Refs footer lines in the body.
func Parse(subject, body string) (ParsedCommit, error) {
	m := ccPattern.FindStringSubmatch(strings.TrimSpace(subject))
	if m == nil {
		return ParsedCommit{}, fmt.Errorf("parsing commit %q: does not follow Conventional Commits format", subject)
	}

	breaking := m[4] == "!" || strings.Contains(body, "BREAKING CHANGE:")
	description := m[5]
	var refs []string

	// Extract trailing (ref, ref, ...) from subject description.
	if sm := trailingRefGroup.FindStringSubmatchIndex(description); sm != nil {
		inner := description[sm[2]:sm[3]]
		if extracted, ok := parseRefList(inner); ok {
			description = description[:sm[0]]
			refs = append(refs, extracted...)
		}
	}

	// Extract refs from Closes/Fixes/Resolves/Refs footer lines in body.
	for _, line := range strings.Split(body, "\n") {
		if fm := footerRefLine.FindStringSubmatch(strings.TrimSpace(line)); fm != nil {
			refs = append(refs, inlineRef.FindAllString(fm[1], -1)...)
		}
	}

	return ParsedCommit{
		Type:        m[1],
		Scope:       m[3],
		Description: description,
		Breaking:    breaking,
		Refs:        nilIfEmpty(refs),
	}, nil
}

// ParseCommit applies the Conventional Commits spec to c.Subject and c.Body.
// Non-conventional commits are retained with an empty Type and Description set to Subject.
func ParseCommit(c git.Commit) ParsedCommit {
	parsed, err := Parse(c.Subject, c.Body)
	if err != nil {
		parsed = ParsedCommit{Description: c.Subject}
	}
	parsed.Commit = c
	return parsed
}

// ParseCommits applies ParseCommit to each commit in the slice.
func ParseCommits(commits []git.Commit) []ParsedCommit {
	result := make([]ParsedCommit, len(commits))
	for i, c := range commits {
		result[i] = ParseCommit(c)
	}
	return result
}

func parseRefList(s string) ([]string, bool) {
	var refs []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if !refToken.MatchString(part) {
			return nil, false
		}
		refs = append(refs, part)
	}
	return refs, len(refs) > 0
}

func nilIfEmpty(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
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
