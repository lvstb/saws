package appstatus

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/lvstb/saws/internal/config"
	"github.com/lvstb/saws/internal/profile"
)

func setupTestEnv(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(tmpDir, "config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(tmpDir, "credentials"))
}

func TestLoadSessionStatus_WithSavedProfilesAndValidCache(t *testing.T) {
	setupTestEnv(t)

	profiles := []profile.SSOProfile{
		{
			Name:        "dev-admin",
			StartURL:    "https://example.awsapps.com/start",
			Region:      "eu-west-1",
			AccountID:   "123456789012",
			AccountName: "Development",
			RoleName:    "AdministratorAccess",
		},
	}
	if err := config.SaveProfiles(profiles); err != nil {
		t.Fatalf("SaveProfiles() error = %v", err)
	}
	expiresAt := time.Now().Add(8 * time.Hour).Truncate(time.Second)
	if err := config.WriteSSOCache(profiles[0].StartURL, profiles[0].Region, "token", expiresAt); err != nil {
		t.Fatalf("WriteSSOCache() error = %v", err)
	}

	status, err := LoadSessionStatus()
	if err != nil {
		t.Fatalf("LoadSessionStatus() error = %v", err)
	}

	if !status.HasProfiles {
		t.Fatal("HasProfiles = false, want true")
	}
	if status.SelectedProfile == nil || status.SelectedProfile.Name != "dev-admin" {
		t.Fatal("SelectedProfile not set from saved profile")
	}
	if status.CacheExpiresAt == nil || !status.CacheExpiresAt.Equal(expiresAt) {
		t.Fatal("CacheExpiresAt not set from valid cache")
	}
}

func TestLoadSessionStatus_NoProfiles(t *testing.T) {
	setupTestEnv(t)

	status, err := LoadSessionStatus()
	if err != nil {
		t.Fatalf("LoadSessionStatus() error = %v", err)
	}

	if status.HasProfiles {
		t.Fatal("HasProfiles = true, want false")
	}
	if status.SelectedProfile != nil {
		t.Fatal("SelectedProfile should be nil when no profiles exist")
	}
	if status.CacheExpiresAt != nil {
		t.Fatal("CacheExpiresAt should be nil when no profiles exist")
	}
}

func TestLoadSessionStatus_ExpiredCache(t *testing.T) {
	setupTestEnv(t)

	profiles := []profile.SSOProfile{
		{
			Name:        "prod-admin",
			StartURL:    "https://example.awsapps.com/start",
			Region:      "us-east-1",
			AccountID:   "210987654321",
			AccountName: "Production",
			RoleName:    "AdministratorAccess",
		},
	}
	if err := config.SaveProfiles(profiles); err != nil {
		t.Fatalf("SaveProfiles() error = %v", err)
	}
	if err := config.WriteSSOCache(profiles[0].StartURL, profiles[0].Region, "expired-token", time.Now().Add(-1*time.Hour)); err != nil {
		t.Fatalf("WriteSSOCache() error = %v", err)
	}

	status, err := LoadSessionStatus()
	if err != nil {
		t.Fatalf("LoadSessionStatus() error = %v", err)
	}

	if !status.HasProfiles {
		t.Fatal("HasProfiles = false, want true")
	}
	if status.SelectedProfile == nil || status.SelectedProfile.Name != "prod-admin" {
		t.Fatal("SelectedProfile not set from saved profile")
	}
	if status.CacheExpiresAt != nil {
		t.Fatal("CacheExpiresAt should be nil when cache is expired")
	}
}
