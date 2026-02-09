// Package credentials handles fetching and formatting AWS temporary credentials.
package credentials

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	"github.com/lvstb/saws/internal/ui"
)

// SSOClient defines the interface for SSO operations (for testability).
type SSOClient interface {
	GetRoleCredentials(ctx context.Context, params *sso.GetRoleCredentialsInput, optFns ...func(*sso.Options)) (*sso.GetRoleCredentialsOutput, error)
	ListAccounts(ctx context.Context, params *sso.ListAccountsInput, optFns ...func(*sso.Options)) (*sso.ListAccountsOutput, error)
	ListAccountRoles(ctx context.Context, params *sso.ListAccountRolesInput, optFns ...func(*sso.Options)) (*sso.ListAccountRolesOutput, error)
}

// DiscoveredAccount holds account info returned from SSO ListAccounts.
type DiscoveredAccount struct {
	AccountID   string
	AccountName string
	Email       string
}

// DiscoveredRole holds role info returned from SSO ListAccountRoles.
type DiscoveredRole struct {
	AccountID string
	RoleName  string
}

// AWSCredentials holds temporary AWS credentials.
type AWSCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Expiration      time.Time
}

// NewSSOClient creates a real SSO client for the given region.
// It loads the default AWS config internally. If you already have a loaded
// aws.Config, use NewSSOClientFromConfig instead to avoid duplicate loads.
func NewSSOClient(ctx context.Context, region string) (SSOClient, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}
	return NewSSOClientFromConfig(cfg), nil
}

// NewSSOClientFromConfig creates a real SSO client from a pre-loaded AWS config.
func NewSSOClientFromConfig(cfg aws.Config) SSOClient {
	return sso.NewFromConfig(cfg)
}

// GetCredentials fetches temporary AWS credentials for the given account and role.
func GetCredentials(
	ctx context.Context,
	client SSOClient,
	accessToken string,
	accountID string,
	roleName string,
) (*AWSCredentials, error) {
	out, err := client.GetRoleCredentials(ctx, &sso.GetRoleCredentialsInput{
		AccessToken: aws.String(accessToken),
		AccountId:   aws.String(accountID),
		RoleName:    aws.String(roleName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get role credentials: %w", err)
	}

	creds := out.RoleCredentials
	return &AWSCredentials{
		AccessKeyID:     aws.ToString(creds.AccessKeyId),
		SecretAccessKey: aws.ToString(creds.SecretAccessKey),
		SessionToken:    aws.ToString(creds.SessionToken),
		Expiration:      time.UnixMilli(creds.Expiration),
	}, nil
}

// FormatExportCommands returns shell export commands for the credentials.
func FormatExportCommands(creds *AWSCredentials) string {
	return fmt.Sprintf(
		"export AWS_ACCESS_KEY_ID=%s\nexport AWS_SECRET_ACCESS_KEY=%s\nexport AWS_SESSION_TOKEN=%s",
		creds.AccessKeyID,
		creds.SecretAccessKey,
		creds.SessionToken,
	)
}

// FormatDisplay returns a styled string showing credentials in a readable format.
func FormatDisplay(creds *AWSCredentials, profileName string) string {
	content := ui.FormatKeyValue("Profile:          ", profileName) + "\n" +
		ui.FormatKeyValue("Access Key ID:    ", creds.AccessKeyID) + "\n" +
		ui.FormatKeyValue("Secret Access Key:", creds.SecretAccessKey) + "\n" +
		ui.FormatKeyValue("Session Token:    ", truncateToken(creds.SessionToken)) + "\n" +
		ui.FormatKeyValue("Expires:          ", creds.Expiration.Format(time.RFC3339))

	return ui.CredentialBoxStyle.Render(content)
}

// truncateToken shortens a session token for display.
func truncateToken(token string) string {
	if len(token) <= 40 {
		return token
	}
	return token[:20] + "..." + token[len(token)-20:]
}

// ListAccounts discovers all AWS accounts accessible with the given SSO token.
// It handles pagination automatically, returning all accounts in a single slice.
func ListAccounts(ctx context.Context, client SSOClient, accessToken string) ([]DiscoveredAccount, error) {
	var accounts []DiscoveredAccount
	var nextToken *string

	for {
		out, err := client.ListAccounts(ctx, &sso.ListAccountsInput{
			AccessToken: aws.String(accessToken),
			NextToken:   nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list accounts: %w", err)
		}

		for _, a := range out.AccountList {
			accounts = append(accounts, DiscoveredAccount{
				AccountID:   aws.ToString(a.AccountId),
				AccountName: aws.ToString(a.AccountName),
				Email:       aws.ToString(a.EmailAddress),
			})
		}

		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	return accounts, nil
}

// ListAccountRoles discovers all roles available for the given account.
// It handles pagination automatically, returning all roles in a single slice.
func ListAccountRoles(ctx context.Context, client SSOClient, accessToken string, accountID string) ([]DiscoveredRole, error) {
	var roles []DiscoveredRole
	var nextToken *string

	for {
		out, err := client.ListAccountRoles(ctx, &sso.ListAccountRolesInput{
			AccessToken: aws.String(accessToken),
			AccountId:   aws.String(accountID),
			NextToken:   nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list account roles: %w", err)
		}

		for _, r := range out.RoleList {
			roles = append(roles, DiscoveredRole{
				AccountID: aws.ToString(r.AccountId),
				RoleName:  aws.ToString(r.RoleName),
			})
		}

		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	return roles, nil
}
