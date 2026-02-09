// Package auth handles the AWS SSO OIDC device authorization flow.
package auth

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
	"github.com/pkg/browser"
)

// openBrowser is the function used to open a URL in the user's browser.
// It defaults to browser.OpenURL and can be overridden in tests.
var openBrowser = browser.OpenURL

func init() {
	// Redirect browser's output to stderr so it doesn't pollute stdout
	// when running under eval $(saws --export ...).
	browser.Stdout = os.Stderr
	browser.Stderr = os.Stderr
}

const (
	clientName = "saws-cli"
	clientType = "public"
	grantType  = "urn:ietf:params:oauth:grant-type:device_code"
)

// OIDCClient defines the interface for SSO OIDC operations (for testability).
type OIDCClient interface {
	RegisterClient(ctx context.Context, params *ssooidc.RegisterClientInput, optFns ...func(*ssooidc.Options)) (*ssooidc.RegisterClientOutput, error)
	StartDeviceAuthorization(ctx context.Context, params *ssooidc.StartDeviceAuthorizationInput, optFns ...func(*ssooidc.Options)) (*ssooidc.StartDeviceAuthorizationOutput, error)
	CreateToken(ctx context.Context, params *ssooidc.CreateTokenInput, optFns ...func(*ssooidc.Options)) (*ssooidc.CreateTokenOutput, error)
}

// TokenResult holds the access token obtained from SSO OIDC.
type TokenResult struct {
	AccessToken string
	ExpiresAt   time.Time
}

// DeviceAuthInfo holds information displayed to the user during authorization.
type DeviceAuthInfo struct {
	VerificationURI string
	UserCode        string
}

// StatusCallback is called during the auth flow to report status to the UI.
type StatusCallback func(status string)

// NewOIDCClient creates a real SSO OIDC client for the given region.
func NewOIDCClient(ctx context.Context, region string) (OIDCClient, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}
	return NewOIDCClientFromConfig(cfg), nil
}

// NewOIDCClientFromConfig creates a real SSO OIDC client from an existing AWS config.
// Use this to share a single LoadDefaultConfig call across multiple clients.
func NewOIDCClientFromConfig(cfg aws.Config) OIDCClient {
	return ssooidc.NewFromConfig(cfg)
}

// Authenticate performs the full SSO OIDC device authorization flow.
// It returns the access token needed to call GetRoleCredentials.
//
// The onDeviceAuth callback is called with the device auth info so the caller
// can display the verification URL and user code to the user.
// The onStatus callback is called with status messages during polling.
func Authenticate(
	ctx context.Context,
	client OIDCClient,
	startURL string,
	onDeviceAuth func(DeviceAuthInfo),
	onStatus StatusCallback,
) (*TokenResult, error) {
	// Step 1: Register client
	onStatus("Registering client...")
	registerOut, err := client.RegisterClient(ctx, &ssooidc.RegisterClientInput{
		ClientName: aws.String(clientName),
		ClientType: aws.String(clientType),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to register client: %w", err)
	}

	// Step 2: Start device authorization
	onStatus("Starting device authorization...")
	deviceOut, err := client.StartDeviceAuthorization(ctx, &ssooidc.StartDeviceAuthorizationInput{
		ClientId:     registerOut.ClientId,
		ClientSecret: registerOut.ClientSecret,
		StartUrl:     aws.String(startURL),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start device authorization: %w", err)
	}

	// Step 3: Notify caller and open browser
	verificationURI := aws.ToString(deviceOut.VerificationUriComplete)
	userCode := aws.ToString(deviceOut.UserCode)

	onDeviceAuth(DeviceAuthInfo{
		VerificationURI: verificationURI,
		UserCode:        userCode,
	})

	// Attempt to open browser (non-fatal if it fails)
	_ = openBrowser(verificationURI)

	// Step 4: Poll for token
	interval := deviceOut.Interval
	if interval == 0 {
		interval = 5
	}

	onStatus("Waiting for browser authorization...")
	token, err := pollForToken(ctx, client, registerOut, deviceOut, interval)
	if err != nil {
		return nil, err
	}

	return token, nil
}

// pollForToken polls the CreateToken endpoint until authorization is complete.
// It attempts one immediate poll before falling into the interval-based loop,
// so users who approve quickly in the browser don't wait an extra interval.
func pollForToken(
	ctx context.Context,
	client OIDCClient,
	register *ssooidc.RegisterClientOutput,
	device *ssooidc.StartDeviceAuthorizationOutput,
	intervalSecs int32,
) (*TokenResult, error) {
	interval := time.Duration(intervalSecs) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Timeout after 5 minutes
	timeout := time.After(5 * time.Minute)

	// Try once immediately, then fall into ticker loop
	first := true
	for {
		if !first {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-timeout:
				return nil, fmt.Errorf("authorization timed out after 5 minutes")
			case <-ticker.C:
			}
		}
		first = false

		tokenOut, err := client.CreateToken(ctx, &ssooidc.CreateTokenInput{
			ClientId:     register.ClientId,
			ClientSecret: register.ClientSecret,
			DeviceCode:   device.DeviceCode,
			GrantType:    aws.String(grantType),
		})
		if err != nil {
			// AuthorizationPendingException means user hasn't approved yet
			if isAuthPending(err) {
				continue
			}
			// SlowDownException means we should increase the interval
			if isSlowDown(err) {
				ticker.Reset(interval + 5*time.Second)
				continue
			}
			return nil, fmt.Errorf("failed to create token: %w", err)
		}

		expiresAt := time.Now().Add(time.Duration(tokenOut.ExpiresIn) * time.Second)
		return &TokenResult{
			AccessToken: aws.ToString(tokenOut.AccessToken),
			ExpiresAt:   expiresAt,
		}, nil
	}
}

func isAuthPending(err error) bool {
	return strings.Contains(err.Error(), "AuthorizationPendingException")
}

func isSlowDown(err error) bool {
	return strings.Contains(err.Error(), "SlowDownException")
}
