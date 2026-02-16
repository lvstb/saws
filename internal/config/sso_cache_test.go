package config

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteAndReadSSOCache(t *testing.T) {
	// Create a temp home directory
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	startURL := "https://mycompany.awsapps.com/start"
	region := "eu-west-1"
	accessToken := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.test-token"
	expiresAt := time.Now().Add(8 * time.Hour).Truncate(time.Second)

	// Write
	err := WriteSSOCache(startURL, region, accessToken, expiresAt)
	if err != nil {
		t.Fatalf("WriteSSOCache() error = %v", err)
	}

	// Verify the file was created at the expected path
	h := sha1.New()
	h.Write([]byte(startURL))
	expectedFilename := strings.ToLower(hex.EncodeToString(h.Sum(nil))) + ".json"
	expectedPath := filepath.Join(tmpHome, ".aws", "sso", "cache", expectedFilename)

	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Fatalf("cache file not created at %s", expectedPath)
	}

	// Verify file permissions
	info, err := os.Stat(expectedPath)
	if err != nil {
		t.Fatalf("cannot stat cache file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}

	// Verify JSON content
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("cannot read cache file: %v", err)
	}

	var raw map[string]string
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("cannot parse cache file JSON: %v", err)
	}

	if raw["startUrl"] != startURL {
		t.Errorf("startUrl = %q, want %q", raw["startUrl"], startURL)
	}
	if raw["region"] != region {
		t.Errorf("region = %q, want %q", raw["region"], region)
	}
	if raw["accessToken"] != accessToken {
		t.Errorf("accessToken = %q, want %q", raw["accessToken"], accessToken)
	}
	if raw["expiresAt"] == "" {
		t.Error("expiresAt is empty")
	}

	// Verify expiresAt is valid RFC3339
	parsedTime, err := time.Parse(time.RFC3339, raw["expiresAt"])
	if err != nil {
		t.Errorf("expiresAt %q is not valid RFC3339: %v", raw["expiresAt"], err)
	}
	if !parsedTime.UTC().Equal(expiresAt.UTC()) {
		t.Errorf("expiresAt = %v, want %v", parsedTime, expiresAt)
	}

	// Read back
	token := ReadSSOCache(startURL)
	if token == nil {
		t.Fatal("ReadSSOCache() returned nil for valid cached token")
	}

	if token.StartURL != startURL {
		t.Errorf("StartURL = %q, want %q", token.StartURL, startURL)
	}
	if token.Region != region {
		t.Errorf("Region = %q, want %q", token.Region, region)
	}
	if token.AccessToken != accessToken {
		t.Errorf("AccessToken = %q, want %q", token.AccessToken, accessToken)
	}
	if !token.ExpiresAt.UTC().Equal(expiresAt.UTC()) {
		t.Errorf("ExpiresAt = %v, want %v", token.ExpiresAt, expiresAt)
	}
}

func TestReadSSOCacheMissing(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	token := ReadSSOCache("https://nonexistent.awsapps.com/start")
	if token != nil {
		t.Error("ReadSSOCache() should return nil for missing cache file")
	}
}

func TestReadSSOCacheExpired(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	startURL := "https://expired.awsapps.com/start"

	// Write an expired token
	err := WriteSSOCache(startURL, "us-east-1", "expired-token", time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("WriteSSOCache() error = %v", err)
	}

	token := ReadSSOCache(startURL)
	if token != nil {
		t.Error("ReadSSOCache() should return nil for expired token")
	}
}

func TestReadSSOCacheAlmostExpired(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	startURL := "https://almost-expired.awsapps.com/start"

	// Write a token that expires in 3 minutes (within the 5-minute buffer)
	err := WriteSSOCache(startURL, "us-east-1", "almost-expired-token", time.Now().Add(3*time.Minute))
	if err != nil {
		t.Fatalf("WriteSSOCache() error = %v", err)
	}

	token := ReadSSOCache(startURL)
	if token != nil {
		t.Error("ReadSSOCache() should return nil for token expiring within 5 minutes")
	}
}

func TestReadSSOCacheInvalidJSON(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	startURL := "https://invalid.awsapps.com/start"
	path, err := ssoCacheFilepath(startURL)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not json"), 0600); err != nil {
		t.Fatal(err)
	}

	token := ReadSSOCache(startURL)
	if token != nil {
		t.Error("ReadSSOCache() should return nil for invalid JSON")
	}
}

func TestReadSSOCacheLegacyFormat(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	startURL := "https://legacy.awsapps.com/start"
	path, err := ssoCacheFilepath(startURL)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}

	// Write a legacy format token (with UTC suffix instead of Z)
	futureTime := time.Now().Add(8 * time.Hour)
	legacyJSON := `{
		"startUrl": "https://legacy.awsapps.com/start",
		"region": "us-east-1",
		"accessToken": "legacy-token",
		"expiresAt": "` + futureTime.UTC().Format("2006-01-02T15:04:05UTC") + `"
	}`
	if err := os.WriteFile(path, []byte(legacyJSON), 0600); err != nil {
		t.Fatal(err)
	}

	token := ReadSSOCache(startURL)
	if token == nil {
		t.Fatal("ReadSSOCache() returned nil for legacy format token")
	}
	if token.AccessToken != "legacy-token" {
		t.Errorf("AccessToken = %q, want %q", token.AccessToken, "legacy-token")
	}
}

func TestSSOCacheFilepathDeterministic(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	startURL := "https://mycompany.awsapps.com/start"

	path1, err := ssoCacheFilepath(startURL)
	if err != nil {
		t.Fatal(err)
	}
	path2, err := ssoCacheFilepath(startURL)
	if err != nil {
		t.Fatal(err)
	}

	if path1 != path2 {
		t.Errorf("ssoCacheFilepath is not deterministic: %q != %q", path1, path2)
	}

	// Different URLs should produce different paths
	path3, err := ssoCacheFilepath("https://other.awsapps.com/start")
	if err != nil {
		t.Fatal(err)
	}
	if path1 == path3 {
		t.Error("different URLs produced the same cache filepath")
	}
}

func TestSSOCacheOverwrite(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	startURL := "https://overwrite.awsapps.com/start"

	// Write first token
	err := WriteSSOCache(startURL, "us-east-1", "token-1", time.Now().Add(8*time.Hour))
	if err != nil {
		t.Fatalf("first WriteSSOCache() error = %v", err)
	}

	// Write second token (overwrite)
	newExpiry := time.Now().Add(8 * time.Hour).Truncate(time.Second)
	err = WriteSSOCache(startURL, "eu-west-1", "token-2", newExpiry)
	if err != nil {
		t.Fatalf("second WriteSSOCache() error = %v", err)
	}

	token := ReadSSOCache(startURL)
	if token == nil {
		t.Fatal("ReadSSOCache() returned nil after overwrite")
	}
	if token.AccessToken != "token-2" {
		t.Errorf("AccessToken = %q, want %q (should be overwritten)", token.AccessToken, "token-2")
	}
	if token.Region != "eu-west-1" {
		t.Errorf("Region = %q, want %q", token.Region, "eu-west-1")
	}
}
