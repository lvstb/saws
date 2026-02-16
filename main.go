package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/sync/errgroup"

	"github.com/lvstb/saws/internal/auth"
	"github.com/lvstb/saws/internal/config"
	"github.com/lvstb/saws/internal/credentials"
	"github.com/lvstb/saws/internal/profile"
	"github.com/lvstb/saws/internal/shell"
	"github.com/lvstb/saws/internal/ui"
)

var (
	version = "dev"

	flagProfile   = flag.String("profile", "", "Use a specific saved profile by name")
	flagConfigure = flag.Bool("configure", false, "Force new profile setup")
	flagExport    = flag.Bool("export", false, "Output only export commands (for eval)")
	flagVersion   = flag.Bool("version", false, "Print version and exit")
)

func main() {
	// Handle subcommands before flag parsing
	if len(os.Args) >= 2 && os.Args[1] == "init" {
		if err := runInit(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, ui.ErrorStyle.Render("Error: "+err.Error()))
			os.Exit(1)
		}
		return
	}

	flag.Parse()

	if *flagVersion {
		fmt.Printf("saws %s\n", version)
		os.Exit(0)
	}

	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, ui.ErrorStyle.Render("Error: "+err.Error()))
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	// In export mode, redirect all display output to stderr so stdout
	// stays clean for shell eval. TUI components use ui.Output.
	// Also set lipgloss renderer to stderr so it detects colors from the
	// TTY (stderr) rather than the pipe (stdout).
	if *flagExport {
		ui.Output = os.Stderr
		lipgloss.SetDefaultRenderer(lipgloss.NewRenderer(os.Stderr))
	}

	fmt.Fprint(ui.Output, ui.Banner())

	// Determine which profile to use
	p, token, err := resolveProfile(ctx)
	if err != nil {
		return err
	}

	// nil profile with nil error means discovery just saved profiles — nothing more to do
	if p == nil {
		return nil
	}

	// If no token yet, check the SSO cache for a valid one
	if token == nil {
		if cached := config.ReadSSOCache(p.StartURL); cached != nil {
			fmt.Fprintln(ui.Output, ui.SuccessStyle.Render("  Using cached SSO token (still valid)"))
			fmt.Fprintln(ui.Output)
			token = &auth.TokenResult{
				AccessToken: cached.AccessToken,
				ExpiresAt:   cached.ExpiresAt,
			}
		}
	}

	// Authenticate via SSO OIDC if we still don't have a token
	if token == nil {
		// Load AWS config once for both auth and credential fetching
		cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(p.Region))
		if err != nil {
			return fmt.Errorf("failed to load AWS config: %w", err)
		}

		token, err = authenticate(ctx, cfg, p)
		if err != nil {
			return err
		}

		// Cache the token for other AWS tools
		if cacheErr := config.WriteSSOCache(p.StartURL, p.Region, token.AccessToken, token.ExpiresAt); cacheErr != nil {
			fmt.Fprintln(os.Stderr, ui.WarningStyle.Render("Warning: could not write SSO cache: "+cacheErr.Error()))
		}

		// Fetch temporary credentials (reuse same config)
		creds, err := fetchCredentials(ctx, cfg, p, token)
		if err != nil {
			return err
		}

		return exportCredentials(p, creds)
	}

	// Token came from cache or discovery flow — need a config for this profile's region
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(p.Region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Fetch temporary credentials
	creds, err := fetchCredentials(ctx, cfg, p, token)
	if err != nil {
		return err
	}

	// Export credentials
	return exportCredentials(p, creds)
}

// resolveProfile determines which SSO profile to use.
// It may also return a token if authentication happened during discovery.
func resolveProfile(ctx context.Context) (*profile.SSOProfile, *auth.TokenResult, error) {
	// --configure flag: run discovery flow
	if *flagConfigure {
		return runDiscoveryFlow(ctx)
	}

	// --profile flag: look up by name
	if *flagProfile != "" {
		p, err := lookupProfile(*flagProfile)
		if err != nil {
			return nil, nil, err
		}
		return p, nil, nil
	}

	// Default: load saved profiles and let user pick
	profiles, err := config.LoadProfiles()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load profiles: %w", err)
	}

	// No saved profiles: run discovery flow
	if len(profiles) == 0 {
		fmt.Fprintln(ui.Output, ui.WarningStyle.Render("No saved SSO profiles found. Let's discover your accounts!"))
		fmt.Fprintln(ui.Output)
		return runDiscoveryFlow(ctx)
	}

	// Single profile: ask to use it or run discovery
	if len(profiles) == 1 {
		p, err := handleSingleProfile(profiles[0])
		if err != nil {
			return nil, nil, err
		}
		if p == nil {
			return runDiscoveryFlow(ctx)
		}
		return p, nil, nil
	}

	// Multiple profiles: fuzzy selector
	p, err := selectProfile(profiles)
	if err != nil {
		return nil, nil, err
	}

	// If user chose "new", run discovery
	if p == nil {
		return runDiscoveryFlow(ctx)
	}
	return p, nil, nil
}

// lookupProfile finds a saved profile by name.
func lookupProfile(name string) (*profile.SSOProfile, error) {
	profiles, err := config.LoadProfiles()
	if err != nil {
		return nil, fmt.Errorf("failed to load profiles: %w", err)
	}

	for _, p := range profiles {
		if p.Name == name {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("profile %q not found in ~/.aws/config", name)
}

// handleSingleProfile handles the case where exactly one profile exists.
func handleSingleProfile(p profile.SSOProfile) (*profile.SSOProfile, error) {
	fmt.Fprintf(ui.Output, "%s %s\n\n",
		ui.SubtitleStyle.Render("Found profile:"),
		ui.SuccessStyle.Render(p.DisplayName()),
	)

	useExisting, err := ui.Confirm("Use this profile?")
	if err != nil {
		return nil, err
	}

	if useExisting {
		return &p, nil
	}
	// Return nil to signal "configure new" — caller handles discovery
	return nil, nil
}

// selectProfile runs the fuzzy selector for multiple profiles.
// Returns nil profile if user chose "configure new".
func selectProfile(profiles []profile.SSOProfile) (*profile.SSOProfile, error) {
	result, err := ui.RunProfileSelector(profiles)
	if err != nil {
		return nil, err
	}

	if result.IsNew {
		return nil, nil
	}
	return result.Profile, nil
}

// runDiscoveryFlow guides the user through SSO setup using auto-discovery.
// It asks for minimal info (URL + region), authenticates, discovers ALL accounts
// and roles, lets the user multi-select which to import, saves them all, then
// drops into the normal profile selector to pick one to use now.
func runDiscoveryFlow(ctx context.Context) (*profile.SSOProfile, *auth.TokenResult, error) {
	// Step 1: Ask for SSO Start URL and Region
	conn, err := ui.RunSSOConnectionForm(nil)
	if err != nil {
		return nil, nil, err
	}

	// Load AWS config once for both OIDC and SSO clients
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(conn.Region))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Step 2: Authenticate via SSO OIDC
	oidcClient := auth.NewOIDCClientFromConfig(cfg)

	token, err := auth.Authenticate(
		ctx,
		oidcClient,
		conn.StartURL,
		func(info auth.DeviceAuthInfo) {
			fmt.Fprintln(ui.Output)
			fmt.Fprintln(ui.Output, ui.BoxStyle.Render(
				ui.FormatKeyValue("Verification URL: ", info.VerificationURI)+"\n"+
					ui.FormatKeyValue("User Code:        ", info.UserCode)+"\n\n"+
					ui.MutedStyle.Render("A browser window should open automatically.\nIf not, open the URL above and enter the code."),
			))
			fmt.Fprintln(ui.Output)
		},
		func(status string) {
			fmt.Fprintln(ui.Output, ui.MutedStyle.Render("  "+status))
		},
	)
	if err != nil {
		return nil, nil, err
	}

	fmt.Fprintln(ui.Output, ui.SuccessStyle.Render("  Authentication successful!"))
	fmt.Fprintln(ui.Output)

	// Cache the token for other AWS tools
	if cacheErr := config.WriteSSOCache(conn.StartURL, conn.Region, token.AccessToken, token.ExpiresAt); cacheErr != nil {
		fmt.Fprintln(os.Stderr, ui.WarningStyle.Render("Warning: could not write SSO cache: "+cacheErr.Error()))
	}

	// Step 3: Discover all accounts
	ssoClient := credentials.NewSSOClientFromConfig(cfg)

	fmt.Fprintln(ui.Output, ui.MutedStyle.Render("  Discovering accounts..."))

	discoveredAccounts, err := credentials.ListAccounts(ctx, ssoClient, token.AccessToken)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to discover accounts: %w", err)
	}

	if len(discoveredAccounts) == 0 {
		return nil, nil, fmt.Errorf("no AWS accounts found for this SSO user")
	}

	fmt.Fprintln(ui.Output, ui.SuccessStyle.Render(fmt.Sprintf("  Found %d account(s)", len(discoveredAccounts))))

	// Step 4: Discover roles for ALL accounts (in parallel)
	fmt.Fprintln(ui.Output, ui.MutedStyle.Render("  Discovering roles..."))

	type accountRoles struct {
		account credentials.DiscoveredAccount
		roles   []credentials.DiscoveredRole
	}

	results := make([]accountRoles, len(discoveredAccounts))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(5) // keep below SSO API rate limits

	for i, acct := range discoveredAccounts {
		results[i].account = acct
		g.Go(func() error {
			roles, err := credentials.ListAccountRoles(gctx, ssoClient, token.AccessToken, acct.AccountID)
			if err != nil {
				return fmt.Errorf("failed to discover roles for account %s: %w", acct.AccountID, err)
			}
			results[i].roles = roles
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, nil, err
	}

	var allProfiles []profile.SSOProfile
	for _, r := range results {
		for _, role := range r.roles {
			allProfiles = append(allProfiles, profile.SSOProfile{
				StartURL:    conn.StartURL,
				Region:      conn.Region,
				AccountID:   r.account.AccountID,
				AccountName: r.account.AccountName,
				RoleName:    role.RoleName,
			})
		}
	}

	if len(allProfiles) == 0 {
		return nil, nil, fmt.Errorf("no roles found across any accounts")
	}

	// Generate unique profile names
	names := ui.GenerateUniqueProfileNames(allProfiles)
	for i := range allProfiles {
		allProfiles[i].Name = names[i]
	}

	fmt.Fprintln(ui.Output, ui.SuccessStyle.Render(fmt.Sprintf("  Found %d profile(s) across %d account(s)", len(allProfiles), len(discoveredAccounts))))
	fmt.Fprintln(ui.Output)

	// Step 5: Let user multi-select which profiles to import
	discovered := make([]ui.DiscoveredProfile, len(allProfiles))
	for i, p := range allProfiles {
		discovered[i] = ui.DiscoveredProfile{Profile: p, Name: p.Name}
	}

	selected, err := ui.RunProfileImportSelector(discovered)
	if err != nil {
		return nil, nil, err
	}

	// Step 6: Save all selected profiles in one batch
	profilesToSave := make([]profile.SSOProfile, len(selected))
	for i, d := range selected {
		p := d.Profile
		p.Name = d.Name
		profilesToSave[i] = p
	}
	if err := config.SaveProfiles(profilesToSave); err != nil {
		return nil, nil, fmt.Errorf("failed to save profiles: %w", err)
	}

	fmt.Fprintln(ui.Output)
	fmt.Fprintln(ui.Output, ui.SuccessStyle.Render(fmt.Sprintf("  Saved %d profile(s) to ~/.aws/config", len(selected))))
	fmt.Fprintln(ui.Output)
	fmt.Fprintln(ui.Output, ui.SubtitleStyle.Render("Run saws again to select a profile and log in."))
	fmt.Fprintln(ui.Output)

	// Return nil profile + nil error to signal "done, nothing more to do"
	return nil, nil, nil
}

// authenticate performs the SSO OIDC device auth flow using a pre-loaded AWS config.
func authenticate(ctx context.Context, cfg aws.Config, p *profile.SSOProfile) (*auth.TokenResult, error) {
	oidcClient := auth.NewOIDCClientFromConfig(cfg)

	token, err := auth.Authenticate(
		ctx,
		oidcClient,
		p.StartURL,
		func(info auth.DeviceAuthInfo) {
			fmt.Fprintln(ui.Output)
			fmt.Fprintln(ui.Output, ui.BoxStyle.Render(
				ui.FormatKeyValue("Verification URL: ", info.VerificationURI)+"\n"+
					ui.FormatKeyValue("User Code:        ", info.UserCode)+"\n\n"+
					ui.MutedStyle.Render("A browser window should open automatically.\nIf not, open the URL above and enter the code."),
			))
			fmt.Fprintln(ui.Output)
		},
		func(status string) {
			fmt.Fprintln(ui.Output, ui.MutedStyle.Render("  "+status))
		},
	)
	if err != nil {
		return nil, err
	}

	fmt.Fprintln(ui.Output, ui.SuccessStyle.Render("  Authentication successful!"))
	fmt.Fprintln(ui.Output)
	return token, nil
}

// fetchCredentials retrieves temporary AWS credentials using a pre-loaded AWS config.
func fetchCredentials(ctx context.Context, cfg aws.Config, p *profile.SSOProfile, token *auth.TokenResult) (*credentials.AWSCredentials, error) {
	ssoClient := credentials.NewSSOClientFromConfig(cfg)

	creds, err := credentials.GetCredentials(ctx, ssoClient, token.AccessToken, p.AccountID, p.RoleName)
	if err != nil {
		return nil, err
	}

	return creds, nil
}

// exportCredentials writes credentials to the credentials file and outputs them.
// In --export mode, export commands go to stdout (for eval) and display goes to
// ui.Output (which is stderr in export mode).
func exportCredentials(p *profile.SSOProfile, creds *credentials.AWSCredentials) error {
	// Always write to ~/.aws/credentials
	if err := config.WriteCredentials(p.Name, creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken); err != nil {
		fmt.Fprintln(os.Stderr, ui.WarningStyle.Render("Warning: could not write to ~/.aws/credentials: "+err.Error()))
	} else {
		fmt.Fprintln(ui.Output, ui.SuccessStyle.Render("  Credentials written to ~/.aws/credentials"))
	}

	// Export mode: export commands on stdout, styled display on stderr
	if *flagExport {
		fmt.Println(credentials.FormatExportCommands(creds))
		fmt.Fprintln(ui.Output, credentials.FormatDisplay(creds, p.Name))
		fmt.Fprintln(ui.Output)
		fmt.Fprintln(ui.Output, ui.SuccessStyle.Render("  Credentials exported to shell environment"))
		fmt.Fprintln(ui.Output)
		return nil
	}

	// Interactive mode: show styled output
	fmt.Fprintln(ui.Output, credentials.FormatDisplay(creds, p.Name))
	fmt.Fprintln(ui.Output)

	if shell.IsWrapped() {
		fmt.Fprintln(ui.Output, ui.SuccessStyle.Render("  Credentials exported to shell environment"))
		fmt.Fprintln(ui.Output)
		return nil
	}

	// Not wrapped: suggest using AWS_PROFILE (works now that SSO cache is populated)
	fmt.Fprintln(ui.Output, ui.SubtitleStyle.Render("To use this profile in other tools:"))
	fmt.Fprintln(ui.Output)
	fmt.Fprintln(ui.Output, ui.MutedStyle.Render("  export AWS_PROFILE="+p.Name))
	fmt.Fprintln(ui.Output)
	fmt.Fprintln(ui.Output, ui.SubtitleStyle.Render("Or set up auto-export with:"))
	fmt.Fprintln(ui.Output)
	fmt.Fprintln(ui.Output, ui.MutedStyle.Render("  saws init"))
	fmt.Fprintln(ui.Output)

	return nil
}

// runInit handles the `saws init [shell]` subcommand.
func runInit(args []string) error {
	fmt.Print(ui.Banner())

	var sh shell.Shell
	var err error

	if len(args) > 0 {
		sh, err = shell.ParseShell(args[0])
	} else {
		sh, err = shell.DetectShell()
	}
	if err != nil {
		return err
	}

	binaryPath, err := shell.BinaryPath()
	if err != nil {
		return err
	}

	rcPath, err := shell.RCFile(sh)
	if err != nil {
		return err
	}

	// Check if already installed
	if shell.IsInstalled(rcPath) {
		fmt.Println(ui.WarningStyle.Render("Shell wrapper already installed in " + rcPath))
		fmt.Println(ui.MutedStyle.Render("Updating to latest version..."))
		fmt.Println()
	}

	if err := shell.Install(sh, binaryPath, rcPath); err != nil {
		return err
	}

	fmt.Println(ui.SuccessStyle.Render("Shell wrapper installed in " + rcPath))
	fmt.Println()
	fmt.Println(ui.SubtitleStyle.Render("To activate, restart your shell or run:"))
	fmt.Println()

	switch sh {
	case shell.Fish:
		fmt.Println(ui.MutedStyle.Render("  source " + rcPath))
	default:
		fmt.Println(ui.MutedStyle.Render("  source " + rcPath))
	}
	fmt.Println()
	fmt.Println(ui.MutedStyle.Render("After that, just type " +
		ui.ValueStyle.Render("saws") +
		ui.MutedStyle.Render(" and credentials will be auto-exported to your shell.")))
	fmt.Println()

	return nil
}
