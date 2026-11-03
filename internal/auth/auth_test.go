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

func TestAuthenticate_RequestsRefreshableToken(t *testing.T) {
	var gotScopes []string
	mock := &mockOIDCClient{
		registerFunc: func(ctx context.Context, params *ssooidc.RegisterClientInput, optFns ...func(*ssooidc.Options)) (*ssooidc.RegisterClientOutput, error) {
			gotScopes = params.Scopes
			return &ssooidc.RegisterClientOutput{
				ClientId:              aws.String("cid"),
				ClientSecret:          aws.String("csecret"),
				ClientSecretExpiresAt: 2000000000,
			}, nil
		},
		createToken: func(ctx context.Context, params *ssooidc.CreateTokenInput, optFns ...func(*ssooidc.Options)) (*ssooidc.CreateTokenOutput, error) {
			return &ssooidc.CreateTokenOutput{
				AccessToken:  aws.String("access"),
				RefreshToken: aws.String("refresh"),
				ExpiresIn:    3600,
			}, nil
		},
	}

	token, err := Authenticate(context.Background(), mock, "https://test.awsapps.com/start",
		func(info DeviceAuthInfo) {}, func(status string) {})
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}

	if len(gotScopes) != 1 || gotScopes[0] != "sso:account:access" {
		t.Errorf("RegisterClient scopes = %v, want [sso:account:access]", gotScopes)
	}
	if token.RefreshToken != "refresh" {
		t.Errorf("RefreshToken = %q, want %q", token.RefreshToken, "refresh")
	}
	if token.ClientID != "cid" || token.ClientSecret != "csecret" {
		t.Errorf("client = %q/%q, want cid/csecret", token.ClientID, token.ClientSecret)
	}
	if !token.RegistrationExpiresAt.Equal(time.Unix(2000000000, 0)) {
		t.Errorf("RegistrationExpiresAt = %v, want %v", token.RegistrationExpiresAt, time.Unix(2000000000, 0))
	}
}

func TestRefresh_Success(t *testing.T) {
	var got *ssooidc.CreateTokenInput
	mock := &mockOIDCClient{
		createToken: func(ctx context.Context, params *ssooidc.CreateTokenInput, optFns ...func(*ssooidc.Options)) (*ssooidc.CreateTokenOutput, error) {
			got = params
			return &ssooidc.CreateTokenOutput{
				AccessToken:  aws.String("new-access"),
				RefreshToken: aws.String("new-refresh"),
				ExpiresIn:    3600,
			}, nil
		},
	}
	regExpiry := time.Now().Add(24 * time.Hour)
	prev := TokenResult{
		RefreshToken:          "old-refresh",
		ClientID:              "cid",
		ClientSecret:          "csecret",
		RegistrationExpiresAt: regExpiry,
	}

	token, err := Refresh(context.Background(), mock, prev)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	if aws.ToString(got.GrantType) != "refresh_token" {
		t.Errorf("GrantType = %q, want refresh_token", aws.ToString(got.GrantType))
	}
	if aws.ToString(got.RefreshToken) != "old-refresh" || aws.ToString(got.ClientId) != "cid" || aws.ToString(got.ClientSecret) != "csecret" {
		t.Errorf("unexpected CreateToken input: %+v", got)
	}
	if token.AccessToken != "new-access" || token.RefreshToken != "new-refresh" {
		t.Errorf("tokens = %q/%q, want new-access/new-refresh", token.AccessToken, token.RefreshToken)
	}
	if token.ClientID != "cid" || token.ClientSecret != "csecret" || !token.RegistrationExpiresAt.Equal(regExpiry) {
		t.Errorf("client registration not carried over: %+v", token)
	}
	if time.Until(token.ExpiresAt) < 59*time.Minute {
		t.Errorf("ExpiresAt = %v, want ~1h from now", token.ExpiresAt)
	}
}

func TestRefresh_KeepsRefreshTokenWhenNotRotated(t *testing.T) {
	mock := &mockOIDCClient{
		createToken: func(ctx context.Context, params *ssooidc.CreateTokenInput, optFns ...func(*ssooidc.Options)) (*ssooidc.CreateTokenOutput, error) {
			return &ssooidc.CreateTokenOutput{AccessToken: aws.String("new-access"), ExpiresIn: 3600}, nil
		},
	}

	token, err := Refresh(context.Background(), mock, TokenResult{RefreshToken: "old-refresh", ClientID: "cid", ClientSecret: "csecret"})
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if token.RefreshToken != "old-refresh" {
		t.Errorf("RefreshToken = %q, want old-refresh", token.RefreshToken)
	}
}

func TestRefresh_Fails(t *testing.T) {
	mock := &mockOIDCClient{
		createToken: func(ctx context.Context, params *ssooidc.CreateTokenInput, optFns ...func(*ssooidc.Options)) (*ssooidc.CreateTokenOutput, error) {
			return nil, fmt.Errorf("InvalidGrantException: refresh token expired")
		},
	}

	if _, err := Refresh(context.Background(), mock, TokenResult{RefreshToken: "r", ClientID: "c", ClientSecret: "s"}); err == nil {
		t.Fatal("expected error, got nil")
	}
}
