// Package profile defines SSO profile data types and validation.
package profile

import (
	"fmt"
	"regexp"
	"strings"
)

// SSOProfile holds all configuration needed for an AWS SSO login.
type SSOProfile struct {
	Name        string `ini:"-"` // profile name (used as section key)
	StartURL    string `ini:"sso_start_url"`
	Region      string `ini:"sso_region"`
	AccountID   string `ini:"sso_account_id"`
	AccountName string `ini:"sso_account_name"` // human-friendly account alias
	RoleName    string `ini:"sso_role_name"`
}

// AWSRegions is the list of valid AWS regions for selection.
var AWSRegions = []string{
	"us-east-1", "us-east-2", "us-west-1", "us-west-2",
	"af-south-1",
	"ap-east-1", "ap-south-1", "ap-south-2", "ap-southeast-1", "ap-southeast-2",
	"ap-southeast-3", "ap-northeast-1", "ap-northeast-2", "ap-northeast-3",
	"ca-central-1",
	"eu-central-1", "eu-central-2", "eu-west-1", "eu-west-2", "eu-west-3",
	"eu-south-1", "eu-south-2", "eu-north-1",
	"me-south-1", "me-central-1",
	"sa-east-1",
}

var (
	accountIDRegex = regexp.MustCompile(`^\d{12}$`)
	urlRegex       = regexp.MustCompile(`^https://[a-zA-Z0-9._-]+\.awsapps\.com/start/?$`)
	urlLooseRegex  = regexp.MustCompile(`^https?://`)
)

// ValidateStartURL checks that the SSO start URL is a valid HTTPS URL.
func ValidateStartURL(url string) error {
	url = strings.TrimSpace(url)
	if url == "" {
		return fmt.Errorf("SSO start URL is required")
	}
	if !urlLooseRegex.MatchString(url) {
		return fmt.Errorf("SSO start URL must begin with https://")
	}
	return nil
}

// ValidateAccountID checks that the account ID is a 12-digit number.
func ValidateAccountID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("account ID is required")
	}
	if !accountIDRegex.MatchString(id) {
		return fmt.Errorf("account ID must be exactly 12 digits")
	}
	return nil
}

// ValidateRoleName checks that the role name is non-empty.
func ValidateRoleName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("role name is required")
	}
	return nil
}

// ValidateProfileName checks that the profile name is non-empty and safe for INI sections.
func ValidateProfileName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("profile name is required")
	}
	if strings.ContainsAny(name, "[]") {
		return fmt.Errorf("profile name cannot contain '[' or ']'")
	}
	return nil
}

// ValidateRegion checks that the region is in the known list.
func ValidateRegion(region string) error {
	region = strings.TrimSpace(region)
	if region == "" {
		return fmt.Errorf("region is required")
	}
	for _, r := range AWSRegions {
		if r == region {
			return nil
		}
	}
	return fmt.Errorf("unknown AWS region: %s", region)
}

// Validate checks all fields of the profile.
func (p *SSOProfile) Validate() error {
	if err := ValidateProfileName(p.Name); err != nil {
		return fmt.Errorf("profile name: %w", err)
	}
	if err := ValidateStartURL(p.StartURL); err != nil {
		return fmt.Errorf("start URL: %w", err)
	}
	if err := ValidateRegion(p.Region); err != nil {
		return fmt.Errorf("region: %w", err)
	}
	if err := ValidateAccountID(p.AccountID); err != nil {
		return fmt.Errorf("account ID: %w", err)
	}
	if err := ValidateRoleName(p.RoleName); err != nil {
		return fmt.Errorf("role name: %w", err)
	}
	return nil
}

// DisplayName returns a formatted string for UI display.
func (p *SSOProfile) DisplayName() string {
	if p.AccountName != "" {
		return fmt.Sprintf("%s (%s / %s)", p.Name, p.AccountName, p.RoleName)
	}
	return fmt.Sprintf("%s (%s / %s)", p.Name, p.AccountID, p.RoleName)
}

// AccountGroup represents an AWS account with one or more SSO roles.
type AccountGroup struct {
	AccountID   string
	AccountName string
	StartURL    string
	Region      string
	Roles       []SSOProfile // all profiles sharing this account
}

// DisplayName returns a formatted string for the account group.
func (g *AccountGroup) DisplayName() string {
	if g.AccountName != "" {
		return fmt.Sprintf("%s (%s)", g.AccountName, g.AccountID)
	}
	return g.AccountID
}

// GroupByAccount groups profiles by their SSO start URL + account ID.
// Profiles with the same start URL and account ID are grouped together,
// differing only by role name.
func GroupByAccount(profiles []SSOProfile) []AccountGroup {
	type key struct {
		startURL  string
		accountID string
	}

	order := []key{}
	groups := map[key]*AccountGroup{}

	for _, p := range profiles {
		k := key{startURL: p.StartURL, accountID: p.AccountID}
		if g, ok := groups[k]; ok {
			g.Roles = append(g.Roles, p)
			// Use the first non-empty account name found
			if g.AccountName == "" && p.AccountName != "" {
				g.AccountName = p.AccountName
			}
		} else {
			order = append(order, k)
			groups[k] = &AccountGroup{
				AccountID:   p.AccountID,
				AccountName: p.AccountName,
				StartURL:    p.StartURL,
				Region:      p.Region,
				Roles:       []SSOProfile{p},
			}
		}
	}

	result := make([]AccountGroup, 0, len(order))
	for _, k := range order {
		result = append(result, *groups[k])
	}
	return result
}
