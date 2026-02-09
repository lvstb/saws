// Package config handles reading and writing SSO profiles to ~/.aws/config.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lvstb/saws/internal/profile"
	"gopkg.in/ini.v1"
)

const (
	sawsMarker = "# managed by saws"
)

// ConfigPath returns the path to the AWS config file.
func ConfigPath() (string, error) {
	// Respect AWS_CONFIG_FILE env var
	if p := os.Getenv("AWS_CONFIG_FILE"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".aws", "config"), nil
}

// CredentialsPath returns the path to the AWS credentials file.
func CredentialsPath() (string, error) {
	if p := os.Getenv("AWS_SHARED_CREDENTIALS_FILE"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".aws", "credentials"), nil
}

// ensureDir creates the parent directory for a file path if it doesn't exist.
func ensureDir(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0700)
}

// loadOrCreateINI loads an INI file or creates a new empty one.
func loadOrCreateINI(path string) (*ini.File, error) {
	if err := ensureDir(path); err != nil {
		return nil, fmt.Errorf("cannot create directory for %s: %w", path, err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return ini.Empty(), nil
	}

	cfg, err := ini.LoadSources(ini.LoadOptions{
		Insensitive:             false,
		AllowNonUniqueSections:  false,
		SkipUnrecognizableLines: true,
	}, path)
	if err != nil {
		return nil, fmt.Errorf("cannot parse %s: %w", path, err)
	}
	return cfg, nil
}

// sectionName returns the INI section name for a profile.
// AWS config uses "profile <name>" for non-default profiles.
func sectionName(name string) string {
	if name == "default" {
		return "default"
	}
	return "profile " + name
}

// profileNameFromSection extracts the profile name from an INI section name.
func profileNameFromSection(section string) string {
	if section == "default" {
		return "default"
	}
	return strings.TrimPrefix(section, "profile ")
}

// isSawsProfile checks if a section has SSO config fields (indicating saws management).
func isSawsProfile(sec *ini.Section) bool {
	return sec.HasKey("sso_start_url") &&
		sec.HasKey("sso_region") &&
		sec.HasKey("sso_account_id") &&
		sec.HasKey("sso_role_name")
}

// LoadProfiles reads all SSO profiles from the AWS config file.
func LoadProfiles() ([]profile.SSOProfile, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	cfg, err := loadOrCreateINI(path)
	if err != nil {
		return nil, err
	}

	var profiles []profile.SSOProfile
	for _, sec := range cfg.Sections() {
		if !isSawsProfile(sec) {
			continue
		}

		p := profile.SSOProfile{
			Name:        profileNameFromSection(sec.Name()),
			StartURL:    sec.Key("sso_start_url").String(),
			Region:      sec.Key("sso_region").String(),
			AccountID:   sec.Key("sso_account_id").String(),
			AccountName: sec.Key("sso_account_name").String(),
			RoleName:    sec.Key("sso_role_name").String(),
		}
		profiles = append(profiles, p)
	}
	return profiles, nil
}

// SaveProfile writes an SSO profile to the AWS config file.
func SaveProfile(p profile.SSOProfile) error {
	return SaveProfiles([]profile.SSOProfile{p})
}

// SaveProfiles writes multiple SSO profiles to the AWS config file in a single
// read/write cycle. This is much faster than calling SaveProfile in a loop.
func SaveProfiles(profiles []profile.SSOProfile) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	cfg, err := loadOrCreateINI(path)
	if err != nil {
		return err
	}

	for _, p := range profiles {
		secName := sectionName(p.Name)
		sec, err := cfg.NewSection(secName)
		if err != nil {
			// Section may already exist
			sec = cfg.Section(secName)
		}

		sec.Comment = sawsMarker
		sec.Key("sso_start_url").SetValue(p.StartURL)
		sec.Key("sso_region").SetValue(p.Region)
		sec.Key("sso_account_id").SetValue(p.AccountID)
		if p.AccountName != "" {
			sec.Key("sso_account_name").SetValue(p.AccountName)
		}
		sec.Key("sso_role_name").SetValue(p.RoleName)
	}

	if err := ensureDir(path); err != nil {
		return err
	}
	return cfg.SaveTo(path)
}

// DeleteProfile removes an SSO profile from the AWS config file.
func DeleteProfile(name string) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	cfg, err := loadOrCreateINI(path)
	if err != nil {
		return err
	}

	secName := sectionName(name)
	cfg.DeleteSection(secName)

	return cfg.SaveTo(path)
}

// WriteCredentials writes temporary credentials to the AWS credentials file.
func WriteCredentials(profileName, accessKeyID, secretAccessKey, sessionToken string) error {
	path, err := CredentialsPath()
	if err != nil {
		return err
	}

	cfg, err := loadOrCreateINI(path)
	if err != nil {
		return err
	}

	sec, err := cfg.NewSection(profileName)
	if err != nil {
		sec = cfg.Section(profileName)
	}

	sec.Comment = sawsMarker
	sec.Key("aws_access_key_id").SetValue(accessKeyID)
	sec.Key("aws_secret_access_key").SetValue(secretAccessKey)
	sec.Key("aws_session_token").SetValue(sessionToken)

	if err := ensureDir(path); err != nil {
		return err
	}
	return cfg.SaveTo(path)
}
