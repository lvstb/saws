package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lvstb/saws/internal/profile"
	"gopkg.in/ini.v1"
)

// setupTestConfig creates a temporary directory and sets AWS_CONFIG_FILE and
// AWS_SHARED_CREDENTIALS_FILE to point to files in that directory.
// Returns a cleanup function.
func setupTestConfig(t *testing.T) func() {
	t.Helper()
	tmpDir := t.TempDir()

	configFile := filepath.Join(tmpDir, "config")
	credsFile := filepath.Join(tmpDir, "credentials")

	os.Setenv("AWS_CONFIG_FILE", configFile)
	os.Setenv("AWS_SHARED_CREDENTIALS_FILE", credsFile)

	return func() {
		os.Unsetenv("AWS_CONFIG_FILE")
		os.Unsetenv("AWS_SHARED_CREDENTIALS_FILE")
	}
}

func TestSaveAndLoadProfile(t *testing.T) {
	cleanup := setupTestConfig(t)
	defer cleanup()

	p := profile.SSOProfile{
		Name:      "test-profile",
		StartURL:  "https://test.awsapps.com/start",
		Region:    "us-east-1",
		AccountID: "123456789012",
		RoleName:  "TestRole",
	}

	// Save
	if err := SaveProfile(p); err != nil {
		t.Fatalf("SaveProfile() error = %v", err)
	}

	// Load
	profiles, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles() error = %v", err)
	}

	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(profiles))
	}

	got := profiles[0]
	if got.Name != p.Name {
		t.Errorf("Name = %q, want %q", got.Name, p.Name)
	}
	if got.StartURL != p.StartURL {
		t.Errorf("StartURL = %q, want %q", got.StartURL, p.StartURL)
	}
	if got.Region != p.Region {
		t.Errorf("Region = %q, want %q", got.Region, p.Region)
	}
	if got.AccountID != p.AccountID {
		t.Errorf("AccountID = %q, want %q", got.AccountID, p.AccountID)
	}
	if got.RoleName != p.RoleName {
		t.Errorf("RoleName = %q, want %q", got.RoleName, p.RoleName)
	}
}

func TestSaveMultipleProfiles(t *testing.T) {
	cleanup := setupTestConfig(t)
	defer cleanup()

	profiles := []profile.SSOProfile{
		{
			Name:      "dev",
			StartURL:  "https://dev.awsapps.com/start",
			Region:    "us-east-1",
			AccountID: "111111111111",
			RoleName:  "DevRole",
		},
		{
			Name:      "prod",
			StartURL:  "https://prod.awsapps.com/start",
			Region:    "eu-west-1",
			AccountID: "222222222222",
			RoleName:  "ProdRole",
		},
	}

	for _, p := range profiles {
		if err := SaveProfile(p); err != nil {
			t.Fatalf("SaveProfile(%s) error = %v", p.Name, err)
		}
	}

	loaded, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles() error = %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(loaded))
	}
}

func TestLoadProfilesEmpty(t *testing.T) {
	cleanup := setupTestConfig(t)
	defer cleanup()

	profiles, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles() error = %v", err)
	}

	if len(profiles) != 0 {
		t.Errorf("expected 0 profiles, got %d", len(profiles))
	}
}

func TestDeleteProfile(t *testing.T) {
	cleanup := setupTestConfig(t)
	defer cleanup()

	p := profile.SSOProfile{
		Name:      "to-delete",
		StartURL:  "https://test.awsapps.com/start",
		Region:    "us-east-1",
		AccountID: "123456789012",
		RoleName:  "TestRole",
	}

	if err := SaveProfile(p); err != nil {
		t.Fatalf("SaveProfile() error = %v", err)
	}

	if err := DeleteProfile("to-delete"); err != nil {
		t.Fatalf("DeleteProfile() error = %v", err)
	}

	profiles, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles() error = %v", err)
	}

	if len(profiles) != 0 {
		t.Errorf("expected 0 profiles after delete, got %d", len(profiles))
	}
}

func TestSaveProfileOverwrite(t *testing.T) {
	cleanup := setupTestConfig(t)
	defer cleanup()

	p := profile.SSOProfile{
		Name:      "my-profile",
		StartURL:  "https://old.awsapps.com/start",
		Region:    "us-east-1",
		AccountID: "111111111111",
		RoleName:  "OldRole",
	}

	if err := SaveProfile(p); err != nil {
		t.Fatalf("SaveProfile() error = %v", err)
	}

	// Update the profile
	p.StartURL = "https://new.awsapps.com/start"
	p.AccountID = "222222222222"
	p.RoleName = "NewRole"

	if err := SaveProfile(p); err != nil {
		t.Fatalf("SaveProfile() error = %v", err)
	}

	profiles, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles() error = %v", err)
	}

	if len(profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(profiles))
	}

	if profiles[0].StartURL != "https://new.awsapps.com/start" {
		t.Errorf("StartURL not updated, got %q", profiles[0].StartURL)
	}
	if profiles[0].AccountID != "222222222222" {
		t.Errorf("AccountID not updated, got %q", profiles[0].AccountID)
	}
}

func TestWriteCredentials(t *testing.T) {
	cleanup := setupTestConfig(t)
	defer cleanup()

	err := WriteCredentials("test-profile", "AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", "FwoGZXIvYXdzEBYaD...")
	if err != nil {
		t.Fatalf("WriteCredentials() error = %v", err)
	}

	// Verify the file was created and has content
	credsPath, _ := CredentialsPath()
	data, err := os.ReadFile(credsPath)
	if err != nil {
		t.Fatalf("cannot read credentials file: %v", err)
	}

	content := string(data)
	if !contains(content, "AKIAIOSFODNN7EXAMPLE") {
		t.Error("credentials file missing access key ID")
	}
	if !contains(content, "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY") {
		t.Error("credentials file missing secret access key")
	}
	if !contains(content, "FwoGZXIvYXdzEBYaD...") {
		t.Error("credentials file missing session token")
	}
}

func TestDefaultProfileSectionName(t *testing.T) {
	if got := sectionName("default"); got != "default" {
		t.Errorf("sectionName(default) = %q, want %q", got, "default")
	}
	if got := sectionName("my-profile"); got != "profile my-profile" {
		t.Errorf("sectionName(my-profile) = %q, want %q", got, "profile my-profile")
	}
}

func TestProfileNameFromSection(t *testing.T) {
	if got := profileNameFromSection("default"); got != "default" {
		t.Errorf("profileNameFromSection(default) = %q, want %q", got, "default")
	}
	if got := profileNameFromSection("profile my-profile"); got != "my-profile" {
		t.Errorf("profileNameFromSection(profile my-profile) = %q, want %q", got, "my-profile")
	}
}

func TestPreservesExistingConfig(t *testing.T) {
	cleanup := setupTestConfig(t)
	defer cleanup()

	configPath, _ := Path()

	// Write some existing non-saws config
	existing := `[profile existing-profile]
region = us-west-2
output = json
`
	if err := os.WriteFile(configPath, []byte(existing), 0600); err != nil {
		t.Fatalf("failed to write existing config: %v", err)
	}

	// Save a saws profile
	p := profile.SSOProfile{
		Name:      "saws-profile",
		StartURL:  "https://test.awsapps.com/start",
		Region:    "us-east-1",
		AccountID: "123456789012",
		RoleName:  "TestRole",
	}
	if err := SaveProfile(p); err != nil {
		t.Fatalf("SaveProfile() error = %v", err)
	}

	// Verify existing profile is preserved
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("cannot read config: %v", err)
	}

	content := string(data)
	if !contains(content, "existing-profile") {
		t.Error("existing profile was removed")
	}
	if !contains(content, "saws-profile") {
		t.Error("saws profile was not added")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsHelper(s, substr)
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestSaveProfilesBatch(t *testing.T) {
	cleanup := setupTestConfig(t)
	defer cleanup()

	profiles := []profile.SSOProfile{
		{
			Name:        "batch-1",
			StartURL:    "https://test.awsapps.com/start",
			Region:      "us-east-1",
			AccountID:   "111111111111",
			AccountName: "Dev",
			RoleName:    "Admin",
		},
		{
			Name:        "batch-2",
			StartURL:    "https://test.awsapps.com/start",
			Region:      "us-east-1",
			AccountID:   "222222222222",
			AccountName: "Staging",
			RoleName:    "ReadOnly",
		},
		{
			Name:        "batch-3",
			StartURL:    "https://test.awsapps.com/start",
			Region:      "eu-west-1",
			AccountID:   "333333333333",
			AccountName: "Prod",
			RoleName:    "Admin",
		},
	}

	if err := SaveProfiles(profiles); err != nil {
		t.Fatalf("SaveProfiles() error = %v", err)
	}

	loaded, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles() error = %v", err)
	}

	if len(loaded) != 3 {
		t.Fatalf("expected 3 profiles, got %d", len(loaded))
	}

	// Verify all profiles saved correctly
	byName := map[string]profile.SSOProfile{}
	for _, p := range loaded {
		byName[p.Name] = p
	}

	for _, want := range profiles {
		got, ok := byName[want.Name]
		if !ok {
			t.Errorf("profile %q not found after batch save", want.Name)
			continue
		}
		if got.AccountID != want.AccountID {
			t.Errorf("profile %q: AccountID = %q, want %q", want.Name, got.AccountID, want.AccountID)
		}
		if got.RoleName != want.RoleName {
			t.Errorf("profile %q: RoleName = %q, want %q", want.Name, got.RoleName, want.RoleName)
		}
	}
}

func writeConfig(t *testing.T, content string) {
	t.Helper()
	path, _ := Path()
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func readConfig(t *testing.T) *ini.File {
	t.Helper()
	path, _ := Path()
	cfg, err := ini.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestSaveProfilesWritesSSOSession(t *testing.T) {
	defer setupTestConfig(t)()

	p := profile.SSOProfile{
		Name:      "dev",
		StartURL:  "https://mycompany.awsapps.com/start",
		Region:    "eu-west-1",
		AccountID: "111111111111",
		RoleName:  "Admin",
	}
	if err := SaveProfile(p); err != nil {
		t.Fatalf("SaveProfile() error = %v", err)
	}

	cfg := readConfig(t)
	sess, err := cfg.GetSection("sso-session mycompany")
	if err != nil {
		t.Fatalf("sso-session section missing: %v", err)
	}
	if got := sess.Key("sso_start_url").String(); got != p.StartURL {
		t.Errorf("session sso_start_url = %q, want %q", got, p.StartURL)
	}
	if got := sess.Key("sso_region").String(); got != p.Region {
		t.Errorf("session sso_region = %q, want %q", got, p.Region)
	}
	if got := sess.Key("sso_registration_scopes").String(); got != "sso:account:access" {
		t.Errorf("session sso_registration_scopes = %q, want sso:account:access", got)
	}

	sec := cfg.Section("profile dev")
	if got := sec.Key("sso_session").String(); got != "mycompany" {
		t.Errorf("profile sso_session = %q, want mycompany", got)
	}
	if sec.HasKey("sso_start_url") || sec.HasKey("sso_region") {
		t.Error("profile should not contain legacy sso_start_url/sso_region keys")
	}

	loaded, err := LoadProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(loaded))
	}
	got := loaded[0]
	if got.StartURL != p.StartURL || got.Region != p.Region || got.SessionName != "mycompany" {
		t.Errorf("loaded profile = %+v", got)
	}
}

func TestSaveProfilesSharesSession(t *testing.T) {
	defer setupTestConfig(t)()

	url := "https://mycompany.awsapps.com/start"
	err := SaveProfiles([]profile.SSOProfile{
		{Name: "a", StartURL: url, Region: "eu-west-1", AccountID: "111111111111", RoleName: "Admin"},
		{Name: "b", StartURL: url, Region: "eu-west-1", AccountID: "222222222222", RoleName: "Admin"},
	})
	if err != nil {
		t.Fatal(err)
	}

	sessions := 0
	for _, sec := range readConfig(t).Sections() {
		if strings.HasPrefix(sec.Name(), "sso-session ") {
			sessions++
		}
	}
	if sessions != 1 {
		t.Errorf("expected 1 sso-session section, got %d", sessions)
	}
}

func TestSaveProfilesReusesExistingSession(t *testing.T) {
	defer setupTestConfig(t)()

	writeConfig(t, `[sso-session corp]
sso_start_url = https://mycompany.awsapps.com/start
sso_region = eu-west-1
`)

	err := SaveProfile(profile.SSOProfile{
		Name: "dev", StartURL: "https://mycompany.awsapps.com/start", Region: "eu-west-1",
		AccountID: "111111111111", RoleName: "Admin",
	})
	if err != nil {
		t.Fatal(err)
	}

	cfg := readConfig(t)
	if got := cfg.Section("profile dev").Key("sso_session").String(); got != "corp" {
		t.Errorf("profile sso_session = %q, want corp (existing session)", got)
	}
	if _, err := cfg.GetSection("sso-session mycompany"); err == nil {
		t.Error("should not create a second session for the same start URL")
	}
}

func TestSaveProfilesAvoidsSessionNameClash(t *testing.T) {
	defer setupTestConfig(t)()

	writeConfig(t, `[sso-session mycompany]
sso_start_url = https://other.example.com/start
sso_region = us-east-1
`)

	err := SaveProfile(profile.SSOProfile{
		Name: "dev", StartURL: "https://mycompany.awsapps.com/start", Region: "eu-west-1",
		AccountID: "111111111111", RoleName: "Admin",
	})
	if err != nil {
		t.Fatal(err)
	}

	cfg := readConfig(t)
	if got := cfg.Section("sso-session mycompany").Key("sso_start_url").String(); got != "https://other.example.com/start" {
		t.Errorf("existing session was overwritten: sso_start_url = %q", got)
	}
	if got := cfg.Section("profile dev").Key("sso_session").String(); got != "mycompany-2" {
		t.Errorf("profile sso_session = %q, want mycompany-2", got)
	}
}

func TestLoadProfilesLegacyFormat(t *testing.T) {
	defer setupTestConfig(t)()

	writeConfig(t, `[profile legacy]
sso_start_url = https://mycompany.awsapps.com/start
sso_region = eu-west-1
sso_account_id = 111111111111
sso_role_name = Admin
`)

	loaded, err := LoadProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(loaded))
	}
	if loaded[0].SessionName != "" || loaded[0].StartURL != "https://mycompany.awsapps.com/start" {
		t.Errorf("loaded legacy profile = %+v", loaded[0])
	}
}

func TestLoadProfilesSkipsMissingSession(t *testing.T) {
	defer setupTestConfig(t)()

	writeConfig(t, `[profile broken]
sso_session = nope
sso_account_id = 111111111111
sso_role_name = Admin
`)

	loaded, err := LoadProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 0 {
		t.Errorf("expected profile with unknown session to be skipped, got %+v", loaded)
	}
}

func TestMigrateLegacyProfiles(t *testing.T) {
	defer setupTestConfig(t)()

	writeConfig(t, `# managed by saws
[profile mine]
sso_start_url = https://mycompany.awsapps.com/start
sso_region = eu-west-1
sso_account_id = 111111111111
sso_account_name = Dev
sso_role_name = Admin

[profile handwritten]
sso_start_url = https://mycompany.awsapps.com/start
sso_region = eu-west-1
sso_account_id = 222222222222
sso_role_name = Admin
`)

	n, err := MigrateLegacyProfiles()
	if err != nil {
		t.Fatalf("MigrateLegacyProfiles() error = %v", err)
	}
	if n != 1 {
		t.Errorf("migrated %d profiles, want 1", n)
	}

	cfg := readConfig(t)
	mine := cfg.Section("profile mine")
	if mine.Key("sso_session").String() != "mycompany" || mine.HasKey("sso_start_url") {
		t.Errorf("saws profile not migrated: %v", mine.KeysHash())
	}
	if mine.Key("sso_account_name").String() != "Dev" {
		t.Error("migration lost sso_account_name")
	}
	hand := cfg.Section("profile handwritten")
	if hand.HasKey("sso_session") || !hand.HasKey("sso_start_url") {
		t.Errorf("hand-written profile should be left alone: %v", hand.KeysHash())
	}

	// Running again is a no-op
	if n, err := MigrateLegacyProfiles(); err != nil || n != 0 {
		t.Errorf("second MigrateLegacyProfiles() = %d, %v; want 0, nil", n, err)
	}
}

func TestSessionName(t *testing.T) {
	defer setupTestConfig(t)()

	tests := map[string]string{
		"https://mycompany.awsapps.com/start":                      "mycompany",
		"https://mycompany.awsapps.com/start/":                     "mycompany",
		"https://identitycenter.amazonaws.com/ssoins-1234567890ab": "identitycenter.amazonaws.com-ssoins-1234567890ab",
	}
	for url, want := range tests {
		got, err := SessionName(url)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("SessionName(%q) = %q, want %q", url, got, want)
		}
	}

	writeConfig(t, `[sso-session corp]
sso_start_url = https://mycompany.awsapps.com/start
sso_region = eu-west-1
`)
	if got, _ := SessionName("https://mycompany.awsapps.com/start"); got != "corp" {
		t.Errorf("SessionName() = %q, want existing session corp", got)
	}
}

func TestMigrateLegacyProfilesDoesNotAddEmptyKeys(t *testing.T) {
	defer setupTestConfig(t)()

	writeConfig(t, `# managed by saws
[profile mine]
sso_start_url = https://mycompany.awsapps.com/start
sso_region = eu-west-1
sso_account_id = 111111111111
sso_role_name = Admin
`)

	if _, err := MigrateLegacyProfiles(); err != nil {
		t.Fatal(err)
	}
	if readConfig(t).Section("profile mine").HasKey("sso_account_name") {
		t.Error("migration added an empty sso_account_name key")
	}
}
