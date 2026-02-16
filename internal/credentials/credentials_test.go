package credentials

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	"github.com/aws/aws-sdk-go-v2/service/sso/types"
)

// mockSSOClient implements SSOClient for testing.
type mockSSOClient struct {
	getRoleCredentials func(ctx context.Context, params *sso.GetRoleCredentialsInput, optFns ...func(*sso.Options)) (*sso.GetRoleCredentialsOutput, error)
	listAccounts       func(ctx context.Context, params *sso.ListAccountsInput, optFns ...func(*sso.Options)) (*sso.ListAccountsOutput, error)
	listAccountRoles   func(ctx context.Context, params *sso.ListAccountRolesInput, optFns ...func(*sso.Options)) (*sso.ListAccountRolesOutput, error)
}

func (m *mockSSOClient) GetRoleCredentials(ctx context.Context, params *sso.GetRoleCredentialsInput, optFns ...func(*sso.Options)) (*sso.GetRoleCredentialsOutput, error) {
	if m.getRoleCredentials != nil {
		return m.getRoleCredentials(ctx, params, optFns...)
	}
	return &sso.GetRoleCredentialsOutput{
		RoleCredentials: &types.RoleCredentials{
			AccessKeyId:     aws.String("AKIAIOSFODNN7EXAMPLE"),
			SecretAccessKey: aws.String("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"),
			SessionToken:    aws.String("FwoGZXIvYXdzEBYaDHN0YXJ0X3VybF9oZXJlIiwicHJvdmlkZXIiOiIxMjM0NTY3ODkwIn0="),
			Expiration:      time.Now().Add(time.Hour).UnixMilli(),
		},
	}, nil
}

func (m *mockSSOClient) ListAccounts(ctx context.Context, params *sso.ListAccountsInput, optFns ...func(*sso.Options)) (*sso.ListAccountsOutput, error) {
	if m.listAccounts != nil {
		return m.listAccounts(ctx, params, optFns...)
	}
	return &sso.ListAccountsOutput{}, nil
}

func (m *mockSSOClient) ListAccountRoles(ctx context.Context, params *sso.ListAccountRolesInput, optFns ...func(*sso.Options)) (*sso.ListAccountRolesOutput, error) {
	if m.listAccountRoles != nil {
		return m.listAccountRoles(ctx, params, optFns...)
	}
	return &sso.ListAccountRolesOutput{}, nil
}

func TestGetCredentials_Success(t *testing.T) {
	mock := &mockSSOClient{}

	creds, err := GetCredentials(context.Background(), mock, "test-token", "123456789012", "TestRole")
	if err != nil {
		t.Fatalf("GetCredentials() error = %v", err)
	}

	if creds.AccessKeyID != "AKIAIOSFODNN7EXAMPLE" {
		t.Errorf("AccessKeyID = %q, want %q", creds.AccessKeyID, "AKIAIOSFODNN7EXAMPLE")
	}
	if creds.SecretAccessKey != "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY" {
		t.Errorf("SecretAccessKey mismatch")
	}
	if creds.SessionToken == "" {
		t.Error("SessionToken is empty")
	}
	if creds.Expiration.IsZero() {
		t.Error("Expiration is zero")
	}
}

func TestGetCredentials_Failure(t *testing.T) {
	mock := &mockSSOClient{
		getRoleCredentials: func(ctx context.Context, params *sso.GetRoleCredentialsInput, optFns ...func(*sso.Options)) (*sso.GetRoleCredentialsOutput, error) {
			return nil, fmt.Errorf("UnauthorizedException: token expired")
		},
	}

	_, err := GetCredentials(context.Background(), mock, "expired-token", "123456789012", "TestRole")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetCredentials_PassesCorrectParams(t *testing.T) {
	var gotToken, gotAccountID, gotRoleName string

	mock := &mockSSOClient{
		getRoleCredentials: func(ctx context.Context, params *sso.GetRoleCredentialsInput, optFns ...func(*sso.Options)) (*sso.GetRoleCredentialsOutput, error) {
			gotToken = aws.ToString(params.AccessToken)
			gotAccountID = aws.ToString(params.AccountId)
			gotRoleName = aws.ToString(params.RoleName)
			return &sso.GetRoleCredentialsOutput{
				RoleCredentials: &types.RoleCredentials{
					AccessKeyId:     aws.String("AKIA..."),
					SecretAccessKey: aws.String("secret"),
					SessionToken:    aws.String("token"),
					Expiration:      time.Now().Add(time.Hour).UnixMilli(),
				},
			}, nil
		},
	}

	_, err := GetCredentials(context.Background(), mock, "my-token", "999888777666", "AdminRole")
	if err != nil {
		t.Fatalf("GetCredentials() error = %v", err)
	}

	if gotToken != "my-token" {
		t.Errorf("AccessToken = %q, want %q", gotToken, "my-token")
	}
	if gotAccountID != "999888777666" {
		t.Errorf("AccountId = %q, want %q", gotAccountID, "999888777666")
	}
	if gotRoleName != "AdminRole" {
		t.Errorf("RoleName = %q, want %q", gotRoleName, "AdminRole")
	}
}

func TestFormatExportCommands(t *testing.T) {
	creds := &AWSCredentials{
		AccessKeyID:     "AKIAEXAMPLE",
		SecretAccessKey: "SECRETEXAMPLE",
		SessionToken:    "TOKENEXAMPLE",
		Expiration:      time.Now().Add(time.Hour),
	}

	result := FormatExportCommands(creds, "my-profile")

	expected := []string{
		"export AWS_ACCESS_KEY_ID=AKIAEXAMPLE",
		"export AWS_SECRET_ACCESS_KEY=SECRETEXAMPLE",
		"export AWS_SESSION_TOKEN=TOKENEXAMPLE",
		"export AWS_PROFILE=my-profile",
	}

	for _, exp := range expected {
		if !strings.Contains(result, exp) {
			t.Errorf("FormatExportCommands() missing %q\ngot: %s", exp, result)
		}
	}
}

func TestFormatDisplay(t *testing.T) {
	creds := &AWSCredentials{
		AccessKeyID:     "AKIAEXAMPLE",
		SecretAccessKey: "SECRETEXAMPLE",
		SessionToken:    "SHORT",
		Expiration:      time.Date(2026, 2, 6, 12, 0, 0, 0, time.UTC),
	}

	result := FormatDisplay(creds, "test-profile")

	// Check that key pieces of info are present
	for _, want := range []string{"AKIAEXAMPLE", "SECRETEXAMPLE", "test-profile", "2026"} {
		if !strings.Contains(result, want) {
			t.Errorf("FormatDisplay() missing %q", want)
		}
	}
}

func TestTruncateToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  string
	}{
		{"short token", "abc", "abc"},
		{"exactly 40", strings.Repeat("a", 40), strings.Repeat("a", 40)},
		{"long token", strings.Repeat("a", 20) + strings.Repeat("b", 20) + strings.Repeat("c", 20), strings.Repeat("a", 20) + "..." + strings.Repeat("c", 20)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateToken(tt.token)
			if got != tt.want {
				t.Errorf("truncateToken() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestListAccounts_Success(t *testing.T) {
	mock := &mockSSOClient{
		listAccounts: func(ctx context.Context, params *sso.ListAccountsInput, optFns ...func(*sso.Options)) (*sso.ListAccountsOutput, error) {
			return &sso.ListAccountsOutput{
				AccountList: []types.AccountInfo{
					{AccountId: aws.String("111111111111"), AccountName: aws.String("Development"), EmailAddress: aws.String("dev@example.com")},
					{AccountId: aws.String("222222222222"), AccountName: aws.String("Production"), EmailAddress: aws.String("prod@example.com")},
				},
			}, nil
		},
	}

	accounts, err := ListAccounts(context.Background(), mock, "test-token")
	if err != nil {
		t.Fatalf("ListAccounts() error = %v", err)
	}

	if len(accounts) != 2 {
		t.Fatalf("ListAccounts() returned %d accounts, want 2", len(accounts))
	}
	if accounts[0].AccountID != "111111111111" {
		t.Errorf("accounts[0].AccountID = %q, want %q", accounts[0].AccountID, "111111111111")
	}
	if accounts[0].AccountName != "Development" {
		t.Errorf("accounts[0].AccountName = %q, want %q", accounts[0].AccountName, "Development")
	}
	if accounts[0].Email != "dev@example.com" {
		t.Errorf("accounts[0].Email = %q, want %q", accounts[0].Email, "dev@example.com")
	}
	if accounts[1].AccountID != "222222222222" {
		t.Errorf("accounts[1].AccountID = %q, want %q", accounts[1].AccountID, "222222222222")
	}
}

func TestListAccounts_Pagination(t *testing.T) {
	callCount := 0
	mock := &mockSSOClient{
		listAccounts: func(ctx context.Context, params *sso.ListAccountsInput, optFns ...func(*sso.Options)) (*sso.ListAccountsOutput, error) {
			callCount++
			if callCount == 1 {
				return &sso.ListAccountsOutput{
					AccountList: []types.AccountInfo{
						{AccountId: aws.String("111111111111"), AccountName: aws.String("Dev")},
					},
					NextToken: aws.String("page2"),
				}, nil
			}
			return &sso.ListAccountsOutput{
				AccountList: []types.AccountInfo{
					{AccountId: aws.String("222222222222"), AccountName: aws.String("Prod")},
				},
			}, nil
		},
	}

	accounts, err := ListAccounts(context.Background(), mock, "test-token")
	if err != nil {
		t.Fatalf("ListAccounts() error = %v", err)
	}

	if len(accounts) != 2 {
		t.Fatalf("ListAccounts() returned %d accounts, want 2", len(accounts))
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls, got %d", callCount)
	}
}

func TestListAccounts_Failure(t *testing.T) {
	mock := &mockSSOClient{
		listAccounts: func(ctx context.Context, params *sso.ListAccountsInput, optFns ...func(*sso.Options)) (*sso.ListAccountsOutput, error) {
			return nil, fmt.Errorf("UnauthorizedException: token expired")
		},
	}

	_, err := ListAccounts(context.Background(), mock, "expired-token")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListAccounts_PassesToken(t *testing.T) {
	var gotToken string
	mock := &mockSSOClient{
		listAccounts: func(ctx context.Context, params *sso.ListAccountsInput, optFns ...func(*sso.Options)) (*sso.ListAccountsOutput, error) {
			gotToken = aws.ToString(params.AccessToken)
			return &sso.ListAccountsOutput{}, nil
		},
	}

	_, err := ListAccounts(context.Background(), mock, "my-special-token")
	if err != nil {
		t.Fatalf("ListAccounts() error = %v", err)
	}
	if gotToken != "my-special-token" {
		t.Errorf("AccessToken = %q, want %q", gotToken, "my-special-token")
	}
}

func TestListAccountRoles_Success(t *testing.T) {
	mock := &mockSSOClient{
		listAccountRoles: func(ctx context.Context, params *sso.ListAccountRolesInput, optFns ...func(*sso.Options)) (*sso.ListAccountRolesOutput, error) {
			return &sso.ListAccountRolesOutput{
				RoleList: []types.RoleInfo{
					{AccountId: aws.String("111111111111"), RoleName: aws.String("AdministratorAccess")},
					{AccountId: aws.String("111111111111"), RoleName: aws.String("ReadOnlyAccess")},
				},
			}, nil
		},
	}

	roles, err := ListAccountRoles(context.Background(), mock, "test-token", "111111111111")
	if err != nil {
		t.Fatalf("ListAccountRoles() error = %v", err)
	}

	if len(roles) != 2 {
		t.Fatalf("ListAccountRoles() returned %d roles, want 2", len(roles))
	}
	if roles[0].RoleName != "AdministratorAccess" {
		t.Errorf("roles[0].RoleName = %q, want %q", roles[0].RoleName, "AdministratorAccess")
	}
	if roles[1].RoleName != "ReadOnlyAccess" {
		t.Errorf("roles[1].RoleName = %q, want %q", roles[1].RoleName, "ReadOnlyAccess")
	}
}

func TestListAccountRoles_Pagination(t *testing.T) {
	callCount := 0
	mock := &mockSSOClient{
		listAccountRoles: func(ctx context.Context, params *sso.ListAccountRolesInput, optFns ...func(*sso.Options)) (*sso.ListAccountRolesOutput, error) {
			callCount++
			if callCount == 1 {
				return &sso.ListAccountRolesOutput{
					RoleList:  []types.RoleInfo{{AccountId: aws.String("111111111111"), RoleName: aws.String("Admin")}},
					NextToken: aws.String("page2"),
				}, nil
			}
			return &sso.ListAccountRolesOutput{
				RoleList: []types.RoleInfo{{AccountId: aws.String("111111111111"), RoleName: aws.String("ReadOnly")}},
			}, nil
		},
	}

	roles, err := ListAccountRoles(context.Background(), mock, "test-token", "111111111111")
	if err != nil {
		t.Fatalf("ListAccountRoles() error = %v", err)
	}

	if len(roles) != 2 {
		t.Fatalf("ListAccountRoles() returned %d roles, want 2", len(roles))
	}
	if callCount != 2 {
		t.Errorf("expected 2 API calls, got %d", callCount)
	}
}

func TestListAccountRoles_Failure(t *testing.T) {
	mock := &mockSSOClient{
		listAccountRoles: func(ctx context.Context, params *sso.ListAccountRolesInput, optFns ...func(*sso.Options)) (*sso.ListAccountRolesOutput, error) {
			return nil, fmt.Errorf("ResourceNotFoundException")
		},
	}

	_, err := ListAccountRoles(context.Background(), mock, "test-token", "111111111111")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListAccountRoles_PassesParams(t *testing.T) {
	var gotToken, gotAccountID string
	mock := &mockSSOClient{
		listAccountRoles: func(ctx context.Context, params *sso.ListAccountRolesInput, optFns ...func(*sso.Options)) (*sso.ListAccountRolesOutput, error) {
			gotToken = aws.ToString(params.AccessToken)
			gotAccountID = aws.ToString(params.AccountId)
			return &sso.ListAccountRolesOutput{}, nil
		},
	}

	_, err := ListAccountRoles(context.Background(), mock, "my-token", "999888777666")
	if err != nil {
		t.Fatalf("ListAccountRoles() error = %v", err)
	}
	if gotToken != "my-token" {
		t.Errorf("AccessToken = %q, want %q", gotToken, "my-token")
	}
	if gotAccountID != "999888777666" {
		t.Errorf("AccountId = %q, want %q", gotAccountID, "999888777666")
	}
}

func TestListAccounts_Empty(t *testing.T) {
	mock := &mockSSOClient{
		listAccounts: func(ctx context.Context, params *sso.ListAccountsInput, optFns ...func(*sso.Options)) (*sso.ListAccountsOutput, error) {
			return &sso.ListAccountsOutput{}, nil
		},
	}

	accounts, err := ListAccounts(context.Background(), mock, "test-token")
	if err != nil {
		t.Fatalf("ListAccounts() error = %v", err)
	}
	if len(accounts) != 0 {
		t.Errorf("ListAccounts() returned %d accounts, want 0", len(accounts))
	}
}
