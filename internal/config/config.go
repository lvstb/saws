// Package config handles reading and writing SSO profiles to ~/.aws/config.
package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lvstb/saws/internal/profile"
	"gopkg.in/ini.v1"
)

const (
	sawsMarker = "# managed by saws"

	sessionSectionPrefix = "sso-session "
	// registrationScopes makes `aws sso login` and the SDKs request refreshable tokens.
	registrationScopes = "sso:account:access"
)

var unsafeSessionChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// Path returns the path to the AWS config file.
func Path() (string, error) {
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

// isSawsProfile checks if a section has legacy (non-refreshable) SSO config fields.
func isSawsProfile(sec *ini.Section) bool {
	return sec.HasKey("sso_start_url") &&
		sec.HasKey("sso_region") &&
		sec.HasKey("sso_account_id") &&
		sec.HasKey("sso_role_name")
}

// isSessionProfile checks if a section is an SSO profile that refers to an [sso-session].
func isSessionProfile(sec *ini.Section) bool {
	return sec.HasKey("sso_session") &&
		sec.HasKey("sso_account_id") &&
		sec.HasKey("sso_role_name")
}

// isManagedBySaws checks if a section carries the saws marker comment.
func isManagedBySaws(sec *ini.Section) bool {
	return strings.Contains(sec.Comment, strings.TrimPrefix(sawsMarker, "# "))
}

// LoadProfiles reads all SSO profiles from the AWS config file, in both the
// sso-session and the legacy format.
func LoadProfiles() ([]profile.SSOProfile, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	cfg, err := loadOrCreateINI(path)
	if err != nil {
		return nil, err
	}

	var profiles []profile.SSOProfile
	for _, sec := range cfg.Sections() {
		// Check the format before reading keys: sec.Key() creates missing keys.
		var p profile.SSOProfile
		switch {
		case isSessionProfile(sec):
			p.SessionName = sec.Key("sso_session").String()
			sess, err := cfg.GetSection(sessionSectionPrefix + p.SessionName)
			if err != nil {
				continue // refers to a session that doesn't exist
			}
			p.StartURL = sess.Key("sso_start_url").String()
			p.Region = sess.Key("sso_region").String()
		case isSawsProfile(sec):
			p.StartURL = sec.Key("sso_start_url").String()
			p.Region = sec.Key("sso_region").String()
		default:
			continue
		}

		p.Name = profileNameFromSection(sec.Name())
		p.AccountID = sec.Key("sso_account_id").String()
		p.AccountName = sec.Key("sso_account_name").String()
		p.RoleName = sec.Key("sso_role_name").String()
		profiles = append(profiles, p)
	}
	return profiles, nil
}

// SessionName returns the [sso-session] name saws uses for a start URL: an
// existing session with that URL, or a new name derived from the URL.
func SessionName(startURL string) (string, error) {
	path, err := Path()
	if err != nil {
		return "", err
	}

	cfg, err := loadOrCreateINI(path)
	if err != nil {
		return "", err
	}
	return sessionNameFor(cfg, startURL), nil
}

func sessionNameFor(cfg *ini.File, startURL string) string {
	for _, sec := range cfg.Sections() {
		if strings.HasPrefix(sec.Name(), sessionSectionPrefix) && sec.HasKey("sso_start_url") && sec.Key("sso_start_url").String() == startURL {
			return strings.TrimPrefix(sec.Name(), sessionSectionPrefix)
		}
	}

	base := deriveSessionName(startURL)
	name := base
	for i := 2; ; i++ {
		if _, err := cfg.GetSection(sessionSectionPrefix + name); err != nil {
			return name
		}
		name = fmt.Sprintf("%s-%d", base, i)
	}
}

// deriveSessionName turns a start URL into a session name:
// "https://mycompany.awsapps.com/start" becomes "mycompany".
func deriveSessionName(startURL string) string {
	u, err := url.Parse(startURL)
	if err != nil || u.Host == "" {
		return strings.Trim(unsafeSessionChars.ReplaceAllString(startURL, "-"), "-")
	}
	if label, ok := strings.CutSuffix(u.Hostname(), ".awsapps.com"); ok && !strings.Contains(label, ".") {
		return label
	}
	return strings.Trim(unsafeSessionChars.ReplaceAllString(u.Hostname()+strings.TrimRight(u.Path, "/"), "-"), "-")
}

// SaveProfile writes an SSO profile to the AWS config file.
func SaveProfile(p profile.SSOProfile) error {
	return SaveProfiles([]profile.SSOProfile{p})
}

// SaveProfiles writes multiple SSO profiles to the AWS config file in a single
// read/write cycle. This is much faster than calling SaveProfile in a loop.
func SaveProfiles(profiles []profile.SSOProfile) error {
	path, err := Path()
	if err != nil {
		return err
	}

	cfg, err := loadOrCreateINI(path)
	if err != nil {
		return err
	}

	applyProfiles(cfg, profiles)

	if err := ensureDir(path); err != nil {
		return err
	}
	return cfg.SaveTo(path)
}

// MigrateLegacyProfiles converts saws-managed legacy profiles to the sso-session
// format so the AWS SDKs can refresh their tokens. Profiles not written by saws
// are left untouched. Returns the number of migrated profiles.
func MigrateLegacyProfiles() (int, error) {
	path, err := Path()
	if err != nil {
		return 0, err
	}

	cfg, err := loadOrCreateINI(path)
	if err != nil {
		return 0, err
	}

	var legacy []profile.SSOProfile
	for _, sec := range cfg.Sections() {
		if !isSawsProfile(sec) || !isManagedBySaws(sec) {
			continue
		}
		p := profile.SSOProfile{
			Name:      profileNameFromSection(sec.Name()),
			StartURL:  sec.Key("sso_start_url").String(),
			Region:    sec.Key("sso_region").String(),
			AccountID: sec.Key("sso_account_id").String(),
			RoleName:  sec.Key("sso_role_name").String(),
		}
		// Only read the optional key if present: sec.Key() would create it and it would be saved.
		if sec.HasKey("sso_account_name") {
			p.AccountName = sec.Key("sso_account_name").String()
		}
		legacy = append(legacy, p)
	}
	if len(legacy) == 0 {
		return 0, nil
	}

	applyProfiles(cfg, legacy)
	if err := cfg.SaveTo(path); err != nil {
		return 0, err
	}
	return len(legacy), nil
}

// applyProfiles writes profiles in the sso-session format, creating one
// [sso-session] per start URL unless a matching one already exists.
func applyProfiles(cfg *ini.File, profiles []profile.SSOProfile) {
	for _, p := range profiles {
		sessionName := sessionNameFor(cfg, p.StartURL)
		if _, err := cfg.GetSection(sessionSectionPrefix + sessionName); err != nil {
			sess := cfg.Section(sessionSectionPrefix + sessionName)
			sess.Comment = sawsMarker
			sess.Key("sso_start_url").SetValue(p.StartURL)
			sess.Key("sso_region").SetValue(p.Region)
			sess.Key("sso_registration_scopes").SetValue(registrationScopes)
		}

		secName := sectionName(p.Name)
		sec, err := cfg.NewSection(secName)
		if err != nil {
			// Section may already exist
			sec = cfg.Section(secName)
		}

		sec.Comment = sawsMarker
		sec.DeleteKey("sso_start_url")
		sec.DeleteKey("sso_region")
		sec.Key("sso_session").SetValue(sessionName)
		sec.Key("sso_account_id").SetValue(p.AccountID)
		if p.AccountName != "" {
			sec.Key("sso_account_name").SetValue(p.AccountName)
		}
		sec.Key("sso_role_name").SetValue(p.RoleName)
	}
}

// DeleteProfile removes an SSO profile from the AWS config file.
func DeleteProfile(name string) error {
	path, err := Path()
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
