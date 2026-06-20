package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	outputMarker = lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6")).Bold(true).Render("→")
	outputDesc   = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
)

// Print outputs a titled output block to stdout. Each description element is
// rendered as an indented, subdued line below the title; newlines within a
// description element are split and each sub-line is indented individually.
func Print(title string, description ...string) {
	fmt.Println(outputMarker + " " + title)
	for _, d := range description {
		for _, line := range strings.Split(d, "\n") {
			fmt.Println(outputDesc.Render("  " + line))
		}
	}
}
