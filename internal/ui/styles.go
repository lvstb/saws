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
	// ColorPrimary is the AWS orange brand color.
	ColorPrimary = lipgloss.Color("#FF9900")
	// ColorSecondary is the AWS dark blue brand color.
	ColorSecondary = lipgloss.Color("#232F3E")
	// ColorSuccess is used for success messages.
	ColorSuccess = lipgloss.Color("#04B575")
	// ColorError is used for error messages.
	ColorError = lipgloss.Color("#FF4444")
	// ColorWarning is used for warning messages.
	ColorWarning = lipgloss.Color("#FFBB33")
	// ColorMuted is used for secondary/dimmed text.
	ColorMuted = lipgloss.Color("#626262")
	// ColorWhite is used for primary text on dark backgrounds.
	ColorWhite = lipgloss.Color("#FAFAFA")

	// TitleStyle is the style for section titles.
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			MarginBottom(1)

	// SubtitleStyle is the style for subtitle text.
	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Italic(true)

	// SuccessStyle is the style for success messages.
	SuccessStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSuccess)

	// ErrorStyle is the style for error messages.
	ErrorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorError)

	// WarningStyle is the style for warning messages.
	WarningStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	// MutedStyle is the style for dimmed/secondary text.
	MutedStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// BoxStyle is the style for bordered content boxes.
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 2)

	// CredentialBoxStyle is the style for credential display boxes.
	CredentialBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorSuccess).
				Padding(1, 2).
				MarginTop(1)

	// KeyStyle is the style for key labels in key-value displays.
	KeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorWhite).
			Width(24)

	// ValueStyle is the style for values in key-value displays.
	ValueStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary)

	// BannerStyle is the style for the ASCII art banner.
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
