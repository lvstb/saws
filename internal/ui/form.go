package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/lvstb/saws/internal/profile"
)

// SSOConnection holds the minimal info needed to authenticate with AWS SSO.
type SSOConnection struct {
	StartURL string
	Region   string
}

// RunSSOConnectionForm displays a minimal form asking only for SSO Start URL and Region.
// This is used for first-time setup / auto-discovery where we authenticate first,
// then discover accounts and roles via the API.
func RunSSOConnectionForm(defaults *SSOConnection) (*SSOConnection, error) {
	var (
		startURL string
		region   string
	)

	if defaults != nil {
		startURL = defaults.StartURL
		region = defaults.Region
	}

	regionOptions := make([]huh.Option[string], len(profile.AWSRegions))
	for i, r := range profile.AWSRegions {
		regionOptions[i] = huh.NewOption(r, r)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("SSO Start URL").
				Description("Your AWS SSO portal URL (ask your IT team if unsure)").
				Placeholder("https://my-org.awsapps.com/start").
				Value(&startURL).
				Validate(profile.ValidateStartURL),

			huh.NewSelect[string]().
				Title("SSO Region").
				Description("The AWS region where your SSO instance is configured").
				Options(regionOptions...).
				Value(&region).
				Height(10),
		).Title("Connect to AWS SSO").
			Description("Enter your SSO details to discover available accounts and roles"),
	).WithTheme(sawsTheme()).WithOutput(Output)

	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("form cancelled: %w", err)
	}

	return &SSOConnection{
		StartURL: startURL,
		Region:   region,
	}, nil
}

// SuggestProfileName generates a profile name from account and role info.
// It lowercases and joins with a dash, e.g. "production-administratoraccess".
func SuggestProfileName(accountName, roleName string) string {
	name := accountName
	if name == "" {
		name = "aws"
	}
	name = strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	role := strings.ToLower(strings.ReplaceAll(roleName, " ", "-"))
	return name + "-" + role
}

// GenerateUniqueProfileNames generates unique profile names for a list of profiles.
// If two profiles would get the same name (e.g. same role across accounts with the
// same name), it appends a numeric suffix (-2, -3, etc.).
func GenerateUniqueProfileNames(profiles []profile.SSOProfile) []string {
	names := make([]string, len(profiles))
	counts := map[string]int{}

	// First pass: generate base names and count occurrences
	baseNames := make([]string, len(profiles))
	for i, p := range profiles {
		base := SuggestProfileName(p.AccountName, p.RoleName)
		baseNames[i] = base
		counts[base]++
	}

	// Second pass: append suffix for duplicates
	seen := map[string]int{}
	for i, base := range baseNames {
		if counts[base] > 1 {
			seen[base]++
			if seen[base] == 1 {
				names[i] = base
			} else {
				names[i] = fmt.Sprintf("%s-%d", base, seen[base])
			}
		} else {
			names[i] = base
		}
	}

	return names
}

// DiscoveredProfile pairs a profile with its auto-generated name for the import selector.
type DiscoveredProfile struct {
	Profile profile.SSOProfile
	Name    string // auto-generated unique profile name
}

// sawsTheme returns a custom huh theme using our style colors.
func sawsTheme() *huh.Theme {
	t := huh.ThemeDracula()

	t.Focused.Title = t.Focused.Title.Foreground(ColorPrimary)
	t.Focused.Description = t.Focused.Description.Foreground(ColorMuted)
	t.Focused.Base = t.Focused.Base.BorderForeground(ColorPrimary)
	t.Blurred.Title = t.Blurred.Title.Foreground(ColorWhite)

	return t
}
