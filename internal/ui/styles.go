// Package ui provides terminal UI components for the saws CLI.
package ui

import (
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
)

// Output is the writer used for TUI rendering. Defaults to os.Stdout.
// Set to os.Stderr when running in --export mode so that stdout remains
// clean for shell eval.
var Output io.Writer = os.Stdout

var (
	// Colors
	ColorPrimary   = lipgloss.Color("#FF9900") // AWS orange
	ColorSecondary = lipgloss.Color("#232F3E") // AWS dark blue
	ColorSuccess   = lipgloss.Color("#04B575")
	ColorError     = lipgloss.Color("#FF4444")
	ColorWarning   = lipgloss.Color("#FFBB33")
	ColorMuted     = lipgloss.Color("#626262")
	ColorWhite     = lipgloss.Color("#FAFAFA")

	// Text styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			MarginBottom(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Italic(true)

	SuccessStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSuccess)

	ErrorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorError)

	WarningStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	MutedStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// Box styles
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 2)

	CredentialBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorSuccess).
				Padding(1, 2).
				MarginTop(1)

	// Key-value display
	KeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorWhite).
			Width(24)

	ValueStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary)

	// Banner
	BannerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			MarginBottom(1)
)

// Banner returns the saws ASCII banner.
func Banner() string {
	banner := `
  ___  __ ___      _____
 / __|/ _` + "`" + ` \ \ /\ / / __|
 \__ \ (_| |\ V  V /\__ \
 |___/\__,_| \_/\_/ |___/
`
	return BannerStyle.Render(banner) + "\n" +
		SubtitleStyle.Render("  AWS SSO Credential Helper") + "\n"
}

// FormatKeyValue renders a styled key-value pair.
func FormatKeyValue(key, value string) string {
	return KeyStyle.Render(key) + ValueStyle.Render(value)
}
