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
	kindCaution
	kindSuccess
	kindFailure
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
	kindCaution:   {"Caution", "✖", lipgloss.Color("#EF4444")},
	kindSuccess:   {"Success", "✓", lipgloss.Color("#22C55E")},
	kindFailure:   {"Failure", "✗", lipgloss.Color("#EF4444")},
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

func alert(kind alertKind, msg string) string {
	meta := alertMeta[kind]

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

	title := titleStyle.Render(fmt.Sprintf("%s %s", meta.icon, meta.title))
	return boxStyle.Render(title + "\n" + msg)
}

// Note renders a blue informational alert.
func Note(msg string) string { return alert(kindNote, msg) }

// Tip renders a green tip alert.
func Tip(msg string) string { return alert(kindTip, msg) }

// Important renders a purple important alert.
func Important(msg string) string { return alert(kindImportant, msg) }

// Warning renders an amber warning alert.
func Warning(msg string) string { return alert(kindWarning, msg) }

// Caution renders a red caution alert.
func Caution(msg string) string { return alert(kindCaution, msg) }

// Success renders a green success alert.
func Success(msg string) string { return alert(kindSuccess, msg) }

// Failure renders a red failure alert.
func Failure(msg string) string { return alert(kindFailure, msg) }
