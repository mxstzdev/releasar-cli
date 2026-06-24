package ui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
)

type alertKind int

const (
	kindNote alertKind = iota
	kindTip
	kindImportant
	kindWarning
	kindError
	kindCaution
)

var alertMeta = map[alertKind]struct {
	title string
	icon  string
	color lipgloss.Color
}{
	kindNote:      {"Note", "ℹ", lipgloss.Color("#3B82F6")},
	kindTip:       {"Tip", "✦", lipgloss.Color("#22C55E")},
	kindImportant: {"Important", "★", lipgloss.Color("#A855F7")},
	kindWarning:   {"Warning", "⚠", lipgloss.Color("#F59E0B")},
	kindError:     {"Error", "", lipgloss.Color("#EF4444")},
	kindCaution:   {"Caution", "✗", lipgloss.Color("#EF4444")},
}

func contentWidth() int {
	w, _, err := term.GetSize(os.Stdout.Fd())
	if err != nil || w == 0 {
		return 80
	}
	if w > 120 {
		return 120
	}
	return w
}

func alert(kind alertKind, title, msg string) string {
	meta := alertMeta[kind]
	if title == "" {
		title = meta.title
	}

	titleStyle := lipgloss.NewStyle().
		Foreground(meta.color).
		Bold(true)

	// subtract border (1) + padding (1) on the left to get inner width
	innerWidth := contentWidth() - 2

	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.ThickBorder()).
		BorderLeft(true).
		BorderForeground(meta.color).
		PaddingLeft(1).
		Width(innerWidth)

	var renderedTitle string
	if meta.icon != "" {
		renderedTitle = titleStyle.Render(fmt.Sprintf("%s %s", meta.icon, title))
	} else {
		renderedTitle = titleStyle.Render(title)
	}

	return boxStyle.Render(renderedTitle + "\n" + msg)
}

func firstOf(ss []string) string {
	if len(ss) > 0 {
		return ss[0]
	}
	return ""
}

// Note renders a blue informational alert. An optional title overrides the default "Note" heading.
func Note(msg string, title ...string) string { return alert(kindNote, firstOf(title), msg) }

// Tip renders a green tip alert. An optional title overrides the default "Tip" heading.
func Tip(msg string, title ...string) string { return alert(kindTip, firstOf(title), msg) }

// Important renders a purple important alert. An optional title overrides the default "Important" heading.
func Important(msg string, title ...string) string { return alert(kindImportant, firstOf(title), msg) }

// Warning renders an amber warning alert. An optional title overrides the default "Warning" heading.
func Warning(msg string, title ...string) string { return alert(kindWarning, firstOf(title), msg) }

// Error renders a red alert without icon. An optional title overrides the default "Error" heading.
func Error(msg string, title ...string) string { return alert(kindError, firstOf(title), msg) }

// Caution renders a red caution alert. An optional title overrides the default "Caution" heading.
func Caution(msg string, title ...string) string { return alert(kindCaution, firstOf(title), msg) }
