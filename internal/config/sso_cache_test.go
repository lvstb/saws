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
	err := WriteSSOCache(startURL, SSOToken{StartURL: startURL, Region: region, AccessToken: accessToken, ExpiresAt: expiresAt})
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
	err := WriteSSOCache(startURL, SSOToken{StartURL: startURL, Region: "us-east-1", AccessToken: "expired-token", ExpiresAt: time.Now().Add(-1 * time.Hour)})
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
	err := WriteSSOCache(startURL, SSOToken{StartURL: startURL, Region: "us-east-1", AccessToken: "almost-expired-token", ExpiresAt: time.Now().Add(3 * time.Minute)})
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
	err := WriteSSOCache(startURL, SSOToken{StartURL: startURL, Region: "us-east-1", AccessToken: "token-1", ExpiresAt: time.Now().Add(8 * time.Hour)})
	if err != nil {
		t.Fatalf("first WriteSSOCache() error = %v", err)
	}

	// Write second token (overwrite)
	newExpiry := time.Now().Add(8 * time.Hour).Truncate(time.Second)
	err = WriteSSOCache(startURL, SSOToken{StartURL: startURL, Region: "eu-west-1", AccessToken: "token-2", ExpiresAt: newExpiry})
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

func refreshableToken(startURL string, accessExpiry, registrationExpiry time.Time) SSOToken {
	return SSOToken{
		StartURL:              startURL,
		Region:                "eu-west-1",
		AccessToken:           "access-token",
		ExpiresAt:             accessExpiry,
		RefreshToken:          "refresh-token",
		ClientID:              "client-id",
		ClientSecret:          "client-secret",
		RegistrationExpiresAt: registrationExpiry,
	}
}

func TestSSOCacheStoresRefreshFields(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	startURL := "https://refresh.awsapps.com/start"
	regExpiry := time.Now().Add(90 * 24 * time.Hour).Truncate(time.Second)
	if err := WriteSSOCache(startURL, refreshableToken(startURL, time.Now().Add(time.Hour), regExpiry)); err != nil {
		t.Fatalf("WriteSSOCache() error = %v", err)
	}

	path, err := ssoCacheFilepath(startURL)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]string
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}

	// Field names must match the AWS CLI/SDK cache format so other tools can refresh too.
	want := map[string]string{
		"refreshToken":          "refresh-token",
		"clientId":              "client-id",
		"clientSecret":          "client-secret",
		"registrationExpiresAt": regExpiry.UTC().Format(time.RFC3339),
	}
	for k, v := range want {
		if raw[k] != v {
			t.Errorf("%s = %q, want %q", k, raw[k], v)
		}
	}
}

func TestSSOCacheOmitsEmptyRefreshFields(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	startURL := "https://norefresh.awsapps.com/start"
	if err := WriteSSOCache(startURL, SSOToken{StartURL: startURL, Region: "us-east-1", AccessToken: "t", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}

	path, _ := ssoCacheFilepath(startURL)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"refreshToken", "clientId", "clientSecret", "registrationExpiresAt"} {
		if strings.Contains(string(data), k) {
			t.Errorf("cache file should not contain %q when empty: %s", k, data)
		}
	}
}

func TestReadRefreshableSSOCache_ExpiredAccessToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	startURL := "https://refresh.awsapps.com/start"
	if err := WriteSSOCache(startURL, refreshableToken(startURL, time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))); err != nil {
		t.Fatal(err)
	}

	if ReadSSOCache(startURL) != nil {
		t.Error("ReadSSOCache() should still return nil for an expired access token")
	}

	token := ReadRefreshableSSOCache(startURL)
	if token == nil {
		t.Fatal("ReadRefreshableSSOCache() returned nil for a refreshable token")
	}
	if token.RefreshToken != "refresh-token" || token.ClientID != "client-id" || token.ClientSecret != "client-secret" {
		t.Errorf("unexpected refresh fields: %+v", token)
	}
}

func TestReadRefreshableSSOCache_NoRefreshToken(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	startURL := "https://refresh.awsapps.com/start"
	tok := refreshableToken(startURL, time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
	tok.RefreshToken = ""
	if err := WriteSSOCache(startURL, tok); err != nil {
		t.Fatal(err)
	}

	if ReadRefreshableSSOCache(startURL) != nil {
		t.Error("ReadRefreshableSSOCache() should return nil without a refresh token")
	}
}

func TestReadRefreshableSSOCache_RegistrationExpired(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	startURL := "https://refresh.awsapps.com/start"
	if err := WriteSSOCache(startURL, refreshableToken(startURL, time.Now().Add(-time.Hour), time.Now().Add(-time.Minute))); err != nil {
		t.Fatal(err)
	}

	if ReadRefreshableSSOCache(startURL) != nil {
		t.Error("ReadRefreshableSSOCache() should return nil when the client registration expired")
	}
}

func TestReadRefreshableSSOCache_Missing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if ReadRefreshableSSOCache("https://missing.awsapps.com/start") != nil {
		t.Error("ReadRefreshableSSOCache() should return nil for missing cache file")
	}
}

func TestSSOCacheKeyedBySessionName(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// sso-session profiles are cached under SHA1(session name), which is where the AWS SDKs look.
	tok := SSOToken{StartURL: "https://mycompany.awsapps.com/start", Region: "eu-west-1", AccessToken: "t", ExpiresAt: time.Now().Add(time.Hour)}
	if err := WriteSSOCache("mycompany", tok); err != nil {
		t.Fatal(err)
	}

	h := sha1.Sum([]byte("mycompany"))
	want := filepath.Join(tmpHome, ".aws", "sso", "cache", hex.EncodeToString(h[:])+".json")
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("cache file not at %s: %v", want, err)
	}
	if ReadSSOCache("mycompany") == nil {
		t.Error("ReadSSOCache(session name) returned nil")
	}
	if ReadSSOCache(tok.StartURL) != nil {
		t.Error("token should not be found under the start URL key")
	}
}
