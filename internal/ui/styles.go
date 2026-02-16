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

	// Style variables — initialized by InitStyles().
	// These are declared at package level so the rest of the codebase can
	// reference them, but they MUST NOT use lipgloss.NewStyle() here because
	// the default renderer may not yet be configured (e.g. in --export mode
	// stdout is a pipe, so lipgloss detects no color support at init time).

	// TitleStyle is the style for section titles.
	TitleStyle lipgloss.Style
	// SubtitleStyle is the style for subtitle text.
	SubtitleStyle lipgloss.Style
	// SuccessStyle is the style for success messages.
	SuccessStyle lipgloss.Style
	// ErrorStyle is the style for error messages.
	ErrorStyle lipgloss.Style
	// WarningStyle is the style for warning messages.
	WarningStyle lipgloss.Style
	// MutedStyle is the style for dimmed/secondary text.
	MutedStyle lipgloss.Style
	// BoxStyle is the style for bordered content boxes.
	BoxStyle lipgloss.Style
	// CredentialBoxStyle is the style for credential display boxes.
	CredentialBoxStyle lipgloss.Style
	// KeyStyle is the style for key labels in key-value displays.
	KeyStyle lipgloss.Style
	// ValueStyle is the style for values in key-value displays.
	ValueStyle lipgloss.Style
	// BannerStyle is the style for the ASCII art banner.
	BannerStyle lipgloss.Style
)

// InitStyles (re)initializes all lipgloss styles using the current default
// renderer. Call this after configuring the lipgloss renderer (e.g. after
// setting it to stderr in --export mode) and before any style is used.
func InitStyles() {
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

	BoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2)

	CredentialBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorSuccess).
		Padding(1, 2).
		MarginTop(1)

	KeyStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorWhite).
		Width(24)

	ValueStyle = lipgloss.NewStyle().
		Foreground(ColorPrimary)

	BannerStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		MarginBottom(1)
}

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
