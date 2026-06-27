package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	outputMarker        = lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6")).Bold(true).Render("→")
	outputMarkerSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E")).Bold(true).Render("✓")
	outputMarkerFailure = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Bold(true).Render("✗")
	outputMarkerSkipped = lipgloss.NewStyle().Foreground(lipgloss.Color("#8B5CF6")).Bold(true).Render("⊘")
	outputDesc          = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	styleHighlight      = lipgloss.NewStyle().Foreground(lipgloss.Color("#EAB308"))
	styleMuted          = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
)

// Print outputs a titled output block to stdout. Each description element is
// rendered as an indented, subdued line below the title; newlines within a
// description element are split and each sub-line is indented individually.
func Print(title string, description ...string) {
	fmt.Println(Sprint(title, description...))
}

// PrintSuccess outputs a titled output block prefixed with a checkmark icon to stdout.
// Each description element is rendered as an indented, subdued line below the title;
// newlines within a description element are split and each sub-line is indented individually.
func PrintSuccess(title string, description ...string) {
	fmt.Println(SprintSuccess(title, description...))
}

// PrintFailure outputs a titled output block prefixed with a times icon to stdout.
// Each description element is rendered as an indented, subdued line below the title;
// newlines within a description element are split and each sub-line is indented individually.
func PrintFailure(title string, description ...string) {
	fmt.Println(SprintFailure(title, description...))
}

// PrintSkipped outputs a titled output block prefixed with a skipped icon to stdout.
// Each description element is rendered as an indented, subdued line below the title;
// newlines within a description element are split and each sub-line is indented individually.
func PrintSkipped(title string, description ...string) {
	fmt.Println(SprintSkipped(title, description...))
}

// Sprint renders a titled output block as a string without printing it.
func Sprint(title string, description ...string) string {
	return sprintBlock(outputMarker, title, description)
}

// SprintSuccess renders a checkmark-prefixed output block as a string without printing it.
func SprintSuccess(title string, description ...string) string {
	return sprintBlock(outputMarkerSuccess, title, description)
}

// SprintFailure renders a times-prefixed output block as a string without printing it.
func SprintFailure(title string, description ...string) string {
	return sprintBlock(outputMarkerFailure, title, description)
}

// SprintSkipped renders a skipped-prefixed output block as a string without printing it.
func SprintSkipped(title string, description ...string) string {
	return sprintBlock(outputMarkerSkipped, title, description)
}

// Highlight renders text in a yellow accent color for emphasis within a line.
func Highlight(text string) string {
	return styleHighlight.Render(text)
}

// Mute renders text in a subdued gray color for de-emphasis within a line.
func Mute(text string) string {
	return styleMuted.Render(text)
}

func sprintBlock(marker, title string, description []string) string {
	var sb strings.Builder
	sb.WriteString(marker + " " + title)
	for _, d := range description {
		for line := range strings.SplitSeq(d, "\n") {
			sb.WriteByte('\n')
			sb.WriteString(outputDesc.Render("  " + line))
		}
	}
	return sb.String()
}
