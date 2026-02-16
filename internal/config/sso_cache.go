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
type SSOToken struct {
	StartURL    string    `json:"startUrl"`
	Region      string    `json:"region"`
	AccessToken string    `json:"accessToken"`
	ExpiresAt   time.Time `json:"-"` // custom marshal to RFC3339
}

// ssoTokenJSON is the wire format for SSOToken (expiresAt as string).
type ssoTokenJSON struct {
	StartURL    string `json:"startUrl"`
	Region      string `json:"region"`
	AccessToken string `json:"accessToken"`
	ExpiresAt   string `json:"expiresAt"`
}

// MarshalJSON implements json.Marshaler with RFC3339 expiresAt.
func (t SSOToken) MarshalJSON() ([]byte, error) {
	return json.Marshal(ssoTokenJSON{
		StartURL:    t.StartURL,
		Region:      t.Region,
		AccessToken: t.AccessToken,
		ExpiresAt:   t.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

// UnmarshalJSON implements json.Unmarshaler with RFC3339 expiresAt.
func (t *SSOToken) UnmarshalJSON(data []byte) error {
	var raw ssoTokenJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	expiresAt, err := time.Parse(time.RFC3339, raw.ExpiresAt)
	if err != nil {
		// Also try the legacy AWS CLI format "2020-06-17T10:02:08UTC"
		expiresAt, err = time.Parse("2006-01-02T15:04:05UTC", raw.ExpiresAt)
		if err != nil {
			return fmt.Errorf("cannot parse expiresAt %q: %w", raw.ExpiresAt, err)
		}
	}

	t.StartURL = raw.StartURL
	t.Region = raw.Region
	t.AccessToken = raw.AccessToken
	t.ExpiresAt = expiresAt
	return nil
}

// ssoCacheDir returns the path to the SSO cache directory.
func ssoCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".aws", "sso", "cache"), nil
}

// ssoCacheFilepath returns the cache file path for a given start URL.
// The filename is the SHA1 hex hash of the start URL, matching the AWS CLI convention.
func ssoCacheFilepath(startURL string) (string, error) {
	dir, err := ssoCacheDir()
	if err != nil {
		return "", err
	}

	h := sha1.New()
	h.Write([]byte(startURL))
	filename := strings.ToLower(hex.EncodeToString(h.Sum(nil))) + ".json"

	return filepath.Join(dir, filename), nil
}

// WriteSSOCache writes an SSO access token to the standard AWS SSO cache.
// This allows other AWS tools (CLI, SDKs) to use the cached token via AWS_PROFILE.
func WriteSSOCache(startURL, region, accessToken string, expiresAt time.Time) error {
	path, err := ssoCacheFilepath(startURL)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("cannot create SSO cache directory: %w", err)
	}

	token := SSOToken{
		StartURL:    startURL,
		Region:      region,
		AccessToken: accessToken,
		ExpiresAt:   expiresAt,
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

// ReadSSOCache reads a cached SSO access token for the given start URL.
// Returns nil if the cache file doesn't exist or the token is expired.
func ReadSSOCache(startURL string) *SSOToken {
	path, err := ssoCacheFilepath(startURL)
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

	// Verify the token has required fields and is not expired.
	// Add a 5-minute buffer to avoid using tokens that are about to expire.
	if token.AccessToken == "" || token.ExpiresAt.Before(time.Now().Add(5*time.Minute)) {
		return nil
	}

	return &token
}
