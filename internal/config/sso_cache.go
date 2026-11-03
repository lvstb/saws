package config

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SSOToken represents a cached SSO access token in the standard AWS CLI format.
// Stored at ~/.aws/sso/cache/{SHA1(startUrl)}.json.
// The refresh fields are optional; when present the access token can be renewed
// without a new browser login.
type SSOToken struct {
	StartURL              string    `json:"startUrl"`
	Region                string    `json:"region"`
	AccessToken           string    `json:"accessToken"`
	ExpiresAt             time.Time `json:"-"` // custom marshal to RFC3339
	RefreshToken          string    `json:"refreshToken,omitempty"`
	ClientID              string    `json:"clientId,omitempty"`
	ClientSecret          string    `json:"clientSecret,omitempty"`
	RegistrationExpiresAt time.Time `json:"-"` // custom marshal to RFC3339
}

// ssoTokenJSON is the wire format for SSOToken (timestamps as strings).
type ssoTokenJSON struct {
	StartURL              string `json:"startUrl"`
	Region                string `json:"region"`
	AccessToken           string `json:"accessToken"`
	ExpiresAt             string `json:"expiresAt"`
	RefreshToken          string `json:"refreshToken,omitempty"`
	ClientID              string `json:"clientId,omitempty"`
	ClientSecret          string `json:"clientSecret,omitempty"`
	RegistrationExpiresAt string `json:"registrationExpiresAt,omitempty"`
}

// MarshalJSON implements json.Marshaler with RFC3339 expiresAt.
func (t SSOToken) MarshalJSON() ([]byte, error) {
	raw := ssoTokenJSON{
		StartURL:     t.StartURL,
		Region:       t.Region,
		AccessToken:  t.AccessToken,
		ExpiresAt:    t.ExpiresAt.UTC().Format(time.RFC3339),
		RefreshToken: t.RefreshToken,
		ClientID:     t.ClientID,
		ClientSecret: t.ClientSecret,
	}
	if !t.RegistrationExpiresAt.IsZero() {
		raw.RegistrationExpiresAt = t.RegistrationExpiresAt.UTC().Format(time.RFC3339)
	}
	return json.Marshal(raw)
}

// UnmarshalJSON implements json.Unmarshaler with RFC3339 expiresAt.
func (t *SSOToken) UnmarshalJSON(data []byte) error {
	var raw ssoTokenJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	expiresAt, err := parseCacheTime(raw.ExpiresAt)
	if err != nil {
		return fmt.Errorf("cannot parse expiresAt %q: %w", raw.ExpiresAt, err)
	}

	var registrationExpiresAt time.Time
	if raw.RegistrationExpiresAt != "" {
		registrationExpiresAt, err = parseCacheTime(raw.RegistrationExpiresAt)
		if err != nil {
			return fmt.Errorf("cannot parse registrationExpiresAt %q: %w", raw.RegistrationExpiresAt, err)
		}
	}

	t.StartURL = raw.StartURL
	t.Region = raw.Region
	t.AccessToken = raw.AccessToken
	t.ExpiresAt = expiresAt
	t.RefreshToken = raw.RefreshToken
	t.ClientID = raw.ClientID
	t.ClientSecret = raw.ClientSecret
	t.RegistrationExpiresAt = registrationExpiresAt
	return nil
}

// parseCacheTime parses RFC3339, falling back to the legacy AWS CLI format "2020-06-17T10:02:08UTC".
func parseCacheTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Parse("2006-01-02T15:04:05UTC", s)
	}
	return t, nil
}

// ssoCacheDir returns the path to the SSO cache directory.
func ssoCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".aws", "sso", "cache"), nil
}

// ssoCacheFilepath returns the cache file path for a cache key: the session name
// for sso-session profiles, or the start URL for legacy profiles.
// The filename is the SHA1 hex hash of the key, matching the AWS CLI convention.
func ssoCacheFilepath(cacheKey string) (string, error) {
	dir, err := ssoCacheDir()
	if err != nil {
		return "", err
	}

	h := sha1.New()
	h.Write([]byte(cacheKey))
	filename := strings.ToLower(hex.EncodeToString(h.Sum(nil))) + ".json"

	return filepath.Join(dir, filename), nil
}

// WriteSSOCache writes an SSO access token to the standard AWS SSO cache.
// This allows other AWS tools (CLI, SDKs) to use the cached token via AWS_PROFILE.
func WriteSSOCache(cacheKey string, token SSOToken) error {
	path, err := ssoCacheFilepath(cacheKey)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("cannot create SSO cache directory: %w", err)
	}

	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("cannot marshal SSO token: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("cannot write SSO cache file: %w", err)
	}

	return nil
}

// readSSOCacheFile reads the cache entry for the given cache key without validating it.
func readSSOCacheFile(cacheKey string) *SSOToken {
	path, err := ssoCacheFilepath(cacheKey)
	if err != nil {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var token SSOToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil
	}
	return &token
}

// ReadSSOCache reads a cached SSO access token for the given cache key.
// Returns nil if the cache file doesn't exist or the token is expired.
func ReadSSOCache(cacheKey string) *SSOToken {
	token := readSSOCacheFile(cacheKey)
	if token == nil {
		return nil
	}

	// Verify the token has required fields and is not expired.
	// Add a 5-minute buffer to avoid using tokens that are about to expire.
	if token.AccessToken == "" || token.ExpiresAt.Before(time.Now().Add(5*time.Minute)) {
		return nil
	}

	return token
}

// ReadRefreshableSSOCache returns the cached entry for the given cache key if it
// can be used to obtain a new access token: it has a refresh token and client
// credentials, and the client registration has not expired.
// The access token itself may be expired.
func ReadRefreshableSSOCache(cacheKey string) *SSOToken {
	token := readSSOCacheFile(cacheKey)
	if token == nil || token.RefreshToken == "" || token.ClientID == "" || token.ClientSecret == "" {
		return nil
	}
	if !token.RegistrationExpiresAt.IsZero() && token.RegistrationExpiresAt.Before(time.Now()) {
		return nil
	}
	return token
}
