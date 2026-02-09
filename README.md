# saws

**AWS SSO credentials, without the pain.**

A single-binary CLI that authenticates via AWS SSO, discovers all your accounts and roles, and gets credentials into your shell.

## Features

- OIDC device authorization flow (opens browser, you approve)
- Auto-discovers all accounts and roles via the SSO API
- Interactive TUI with filtering and two-level selector (account → role)
- Multi-select which accounts/roles to import as named profiles
- Saves profiles to `~/.aws/config` — standard format, works with AWS CLI
- Writes temporary credentials to `~/.aws/credentials`
- Exports `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `AWS_SESSION_TOKEN` to your shell
- Shell wrapper for bash, zsh, and fish

## Install

### Go

```sh
go install github.com/lvstb/saws@latest
```

### Download a release binary

Grab the latest binary for your platform from [GitHub Releases](https://github.com/lvstb/saws/releases):

```sh
# macOS (Apple Silicon)
curl -Lo saws https://github.com/lvstb/saws/releases/latest/download/saws-darwin-arm64
chmod +x saws
sudo mv saws /usr/local/bin/

# macOS (Intel)
curl -Lo saws https://github.com/lvstb/saws/releases/latest/download/saws-darwin-amd64
chmod +x saws
sudo mv saws /usr/local/bin/

# Linux (x86_64)
curl -Lo saws https://github.com/lvstb/saws/releases/latest/download/saws-linux-amd64
chmod +x saws
sudo mv saws /usr/local/bin/

# Linux (ARM64)
curl -Lo saws https://github.com/lvstb/saws/releases/latest/download/saws-linux-arm64
chmod +x saws
sudo mv saws /usr/local/bin/
```

### Build from source

```sh
git clone https://github.com/lvstb/saws.git
cd saws
go build -o saws .
sudo mv saws /usr/local/bin/
```

### Requirements

- AWS SSO configured in your organization
- A modern terminal

## Quick start

```sh
# Install the shell wrapper (bash/zsh/fish)
saws init zsh

# Restart your shell, then run:
saws
```

On first run, saws will:

1. Ask for your SSO start URL and region
2. Open your browser for device authorization
3. Discover all available accounts and roles
4. Let you multi-select which to import as profiles
5. Save them to `~/.aws/config`
6. Prompt you to select a profile and export credentials

## Usage

```
saws                     # Interactive: select profile or set up new
saws init [shell]        # Install shell wrapper (bash/zsh/fish)
saws --profile <name>    # Use a specific saved profile
saws --configure         # Force new profile setup (discovery flow)
saws --export            # Output export commands on stdout (for eval)
saws --version           # Print version
```

### Examples

Select a profile interactively:

```sh
saws
```

Use a specific profile directly:

```sh
saws --profile my-account-admin
```

Re-run discovery to add more profiles:

```sh
saws --configure
```

## Shell integration

`saws init` installs a shell function that wraps the binary. When you run `saws`, it:

- Runs `saws --export` and captures stdout for `eval`
- Sends TUI output to stderr so you still see it
- Automatically exports credentials to your current shell session

Without the wrapper, you can do this manually:

```sh
eval $(saws --export --profile my-account-admin)
```

## Config format

saws stores profiles in standard AWS config format:

```ini
[profile my-account-admin]
sso_start_url = https://mycompany.awsapps.com/start
sso_region = us-east-1
sso_account_id = 123456789012
sso_account_name = my-account
sso_role_name = AdministratorAccess
```

These profiles are fully compatible with the AWS CLI (`aws --profile my-account-admin`).

## License

MIT
