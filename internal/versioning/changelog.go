package versioning

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const changelogHeader = "# Changelog"

// Entry represents a single versioned section in a CHANGELOG file.
type Entry struct {
	Version  string
	Date     time.Time
	Sections []Section
}

// Section is a named group of changelog items within an Entry.
type Section struct {
	Title string
	Items []string
}

// ChangelogFromCommits builds an Entry from parsed commits, following Common Changelog.
// Commits of type test, chore, ci, and build are omitted as maintenance noise.
// Breaking items sort first within their section.
func ChangelogFromCommits(version string, date time.Time, commits []ParsedCommit) Entry {
	order := []string{"Changed", "Added", "Removed", "Fixed"}
	breaking := map[string][]string{}
	normal := map[string][]string{}

	for _, c := range commits {
		section := changelogSection(c.Type)
		if section == "" {
			continue
		}
		item := formatChangelogItem(c)
		if c.Breaking {
			breaking[section] = append(breaking[section], item)
		} else {
			normal[section] = append(normal[section], item)
		}
	}

	var sections []Section
	for _, key := range order {
		items := append(breaking[key], normal[key]...)
		if len(items) > 0 {
			sections = append(sections, Section{Title: key, Items: items})
		}
	}

	return Entry{Version: version, Date: date, Sections: sections}
}

func changelogSection(commitType string) string {
	switch commitType {
	case "feat":
		return "Added"
	case "fix":
		return "Fixed"
	case "refactor", "perf", "style", "docs", "revert":
		return "Changed"
	default:
		return ""
	}
}

func formatChangelogItem(c ParsedCommit) string {
	var b strings.Builder

	switch {
	case c.Breaking && c.Scope != "":
		fmt.Fprintf(&b, "**%s (breaking):** ", c.Scope)
	case c.Breaking:
		b.WriteString("**Breaking:** ")
	case c.Scope != "":
		fmt.Fprintf(&b, "**%s:** ", c.Scope)
	}

	b.WriteString(c.Description)

	var refs []string
	refs = append(refs, c.Refs...)
	if c.Hash != "" {
		refs = append(refs, "`"+c.Hash+"`")
	}
	if len(refs) > 0 {
		fmt.Fprintf(&b, " (%s)", strings.Join(refs, ", "))
	}

	if c.Author != "" {
		fmt.Fprintf(&b, " (%s)", c.Author)
	}

	return b.String()
}

// Render returns the Markdown representation of the entry.
func (e Entry) Render() string {
	var b strings.Builder
	fmt.Fprintf(&b, "## [%s] - %s\n", e.Version, e.Date.Format("2006-01-02"))
	for _, s := range e.Sections {
		fmt.Fprintf(&b, "\n### %s\n\n", s.Title)
		for _, item := range s.Items {
			fmt.Fprintf(&b, "- %s\n", item)
		}
	}
	return b.String()
}

// PrependChangelog inserts entry at the top of the CHANGELOG at path, after the
// "# Changelog" header, creating the file if it does not exist.
// Existing file permissions are preserved; new files are created with 0o644.
func PrependChangelog(path string, entry Entry) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode()
	}

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading changelog %s: %w", path, err)
	}

	if err := os.WriteFile(path, []byte(buildChangelogContent(string(data), entry.Render())), mode); err != nil {
		return fmt.Errorf("writing changelog %s: %w", path, err)
	}
	return nil
}

// buildChangelogContent inserts rendered after the header in existing, adding the
// header if absent. existing may be empty (new file).
func buildChangelogContent(existing, rendered string) string {
	existing = strings.TrimSpace(existing)

	if existing == "" {
		return changelogHeader + "\n\n" + rendered + "\n"
	}

	if strings.HasPrefix(existing, changelogHeader) {
		newline := strings.Index(existing, "\n")
		if newline == -1 {
			return existing + "\n\n" + rendered + "\n"
		}
		head := existing[:newline+1]
		body := strings.TrimLeft(existing[newline+1:], "\n")
		if body == "" {
			return head + "\n" + rendered + "\n"
		}
		return head + "\n" + rendered + "\n" + body + "\n"
	}

	return changelogHeader + "\n\n" + rendered + "\n" + existing + "\n"
}
