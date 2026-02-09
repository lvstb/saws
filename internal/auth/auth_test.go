package auth

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssooidc"
)

func init() {
	// Prevent tests from opening real browser tabs.
	openBrowser = func(url string) error { return nil }
}

// mockOIDCClient implements OIDCClient for testing.
type mockOIDCClient struct {
	registerFunc  func(ctx context.Context, params *ssooidc.RegisterClientInput, optFns ...func(*ssooidc.Options)) (*ssooidc.RegisterClientOutput, error)
	startAuthFunc func(ctx context.Context, params *ssooidc.StartDeviceAuthorizationInput, optFns ...func(*ssooidc.Options)) (*ssooidc.StartDeviceAuthorizationOutput, error)
	createToken   func(ctx context.Context, params *ssooidc.CreateTokenInput, optFns ...func(*ssooidc.Options)) (*ssooidc.CreateTokenOutput, error)
	tokenCalls    int
}

func (m *mockOIDCClient) RegisterClient(ctx context.Context, params *ssooidc.RegisterClientInput, optFns ...func(*ssooidc.Options)) (*ssooidc.RegisterClientOutput, error) {
	if m.registerFunc != nil {
		return m.registerFunc(ctx, params, optFns...)
	}
	return &ssooidc.RegisterClientOutput{
		ClientId:     aws.String("test-client-id"),
		ClientSecret: aws.String("test-client-secret"),
	}, nil
}

func (m *mockOIDCClient) StartDeviceAuthorization(ctx context.Context, params *ssooidc.StartDeviceAuthorizationInput, optFns ...func(*ssooidc.Options)) (*ssooidc.StartDeviceAuthorizationOutput, error) {
	if m.startAuthFunc != nil {
		return m.startAuthFunc(ctx, params, optFns...)
	}
	return &ssooidc.StartDeviceAuthorizationOutput{
		DeviceCode:              aws.String("test-device-code"),
		UserCode:                aws.String("TEST-CODE"),
		VerificationUri:         aws.String("https://device.sso.us-east-1.amazonaws.com/"),
		VerificationUriComplete: aws.String("https://device.sso.us-east-1.amazonaws.com/?user_code=TEST-CODE"),
		Interval:                1, // 1 second for fast tests
	}, nil
}

func (m *mockOIDCClient) CreateToken(ctx context.Context, params *ssooidc.CreateTokenInput, optFns ...func(*ssooidc.Options)) (*ssooidc.CreateTokenOutput, error) {
	m.tokenCalls++
	if m.createToken != nil {
		return m.createToken(ctx, params, optFns...)
	}
	// Succeed on first call
	return &ssooidc.CreateTokenOutput{
		AccessToken: aws.String("test-access-token"),
		ExpiresIn:   3600,
	}, nil
}

func TestAuthenticate_Success(t *testing.T) {
	mock := &mockOIDCClient{}

	var gotDeviceAuth DeviceAuthInfo
	var statuses []string

	ctx := context.Background()
	token, err := Authenticate(ctx, mock, "https://test.awsapps.com/start",
		func(info DeviceAuthInfo) {
			gotDeviceAuth = info
		},
		func(status string) {
			statuses = append(statuses, status)
		},
	)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}

	if token.AccessToken != "test-access-token" {
		t.Errorf("AccessToken = %q, want %q", token.AccessToken, "test-access-token")
	}

	if gotDeviceAuth.UserCode != "TEST-CODE" {
		t.Errorf("UserCode = %q, want %q", gotDeviceAuth.UserCode, "TEST-CODE")
	}

	if gotDeviceAuth.VerificationURI == "" {
		t.Error("VerificationURI is empty")
	}

	if len(statuses) < 2 {
		t.Errorf("expected at least 2 status messages, got %d", len(statuses))
	}
}

func TestAuthenticate_PollsUntilApproved(t *testing.T) {
	callCount := 0
	mock := &mockOIDCClient{
		createToken: func(ctx context.Context, params *ssooidc.CreateTokenInput, optFns ...func(*ssooidc.Options)) (*ssooidc.CreateTokenOutput, error) {
			callCount++
			if callCount < 3 {
				return nil, fmt.Errorf("AuthorizationPendingException: waiting for approval")
			}
			return &ssooidc.CreateTokenOutput{
				AccessToken: aws.String("approved-token"),
				ExpiresIn:   3600,
			}, nil
		},
	}

	ctx := context.Background()
	token, err := Authenticate(ctx, mock, "https://test.awsapps.com/start",
		func(info DeviceAuthInfo) {},
		func(status string) {},
	)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}

	if token.AccessToken != "approved-token" {
		t.Errorf("AccessToken = %q, want %q", token.AccessToken, "approved-token")
	}

	if callCount != 3 {
		t.Errorf("expected 3 CreateToken calls, got %d", callCount)
	}
}

func TestAuthenticate_RegisterFails(t *testing.T) {
	mock := &mockOIDCClient{
		registerFunc: func(ctx context.Context, params *ssooidc.RegisterClientInput, optFns ...func(*ssooidc.Options)) (*ssooidc.RegisterClientOutput, error) {
			return nil, fmt.Errorf("network error")
		},
	}

	ctx := context.Background()
	_, err := Authenticate(ctx, mock, "https://test.awsapps.com/start",
		func(info DeviceAuthInfo) {},
		func(status string) {},
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAuthenticate_StartAuthFails(t *testing.T) {
	mock := &mockOIDCClient{
		startAuthFunc: func(ctx context.Context, params *ssooidc.StartDeviceAuthorizationInput, optFns ...func(*ssooidc.Options)) (*ssooidc.StartDeviceAuthorizationOutput, error) {
			return nil, fmt.Errorf("invalid start URL")
		},
	}

	ctx := context.Background()
	_, err := Authenticate(ctx, mock, "https://test.awsapps.com/start",
		func(info DeviceAuthInfo) {},
		func(status string) {},
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAuthenticate_CreateTokenFails(t *testing.T) {
	mock := &mockOIDCClient{
		createToken: func(ctx context.Context, params *ssooidc.CreateTokenInput, optFns ...func(*ssooidc.Options)) (*ssooidc.CreateTokenOutput, error) {
			return nil, fmt.Errorf("AccessDeniedException: user denied")
		},
	}

	ctx := context.Background()
	_, err := Authenticate(ctx, mock, "https://test.awsapps.com/start",
		func(info DeviceAuthInfo) {},
		func(status string) {},
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAuthenticate_ContextCancelled(t *testing.T) {
	mock := &mockOIDCClient{
		createToken: func(ctx context.Context, params *ssooidc.CreateTokenInput, optFns ...func(*ssooidc.Options)) (*ssooidc.CreateTokenOutput, error) {
			return nil, fmt.Errorf("AuthorizationPendingException: waiting")
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := Authenticate(ctx, mock, "https://test.awsapps.com/start",
		func(info DeviceAuthInfo) {},
		func(status string) {},
	)
	if err == nil {
		t.Fatal("expected error on context cancel, got nil")
	}
}

func TestIsAuthPending(t *testing.T) {
	if !isAuthPending(fmt.Errorf("AuthorizationPendingException: still waiting")) {
		t.Error("expected true for AuthorizationPendingException")
	}
	if isAuthPending(fmt.Errorf("AccessDeniedException: denied")) {
		t.Error("expected false for AccessDeniedException")
	}
}

func TestIsSlowDown(t *testing.T) {
	if !isSlowDown(fmt.Errorf("SlowDownException: too many requests")) {
		t.Error("expected true for SlowDownException")
	}
	if isSlowDown(fmt.Errorf("AuthorizationPendingException: still waiting")) {
		t.Error("expected false for AuthorizationPendingException")
	}
}
