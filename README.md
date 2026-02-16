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
- Caches SSO tokens in `~/.aws/sso/cache/` so `export AWS_PROFILE=<name>` works with any AWS tool (CLI, SDKs, Terraform, etc.)
- Reuses cached tokens on subsequent runs — skips browser auth if the token is still valid
- Exports `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `AWS_SESSION_TOKEN` to your shell
- Shell wrapper for bash, zsh, and fish

## Install

### Homebrew (macOS / Linux)

```sh
brew install lvstb/tap/saws
```

### Nix (NixOS / Linux / macOS)

```sh
# install in your user profile
nix profile install github:lvstb/saws

# run without installing
nix run github:lvstb/saws
```

#### Flakes (Home Manager or NixOS)

Add the flake input to your `flake.nix`:

```nix
{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable"
    home-manager.url = "github:nix-community/home-manager"
    saws.url = "github:lvstb/saws"
  }
}
```

Then add the package in your Home Manager config:

```nix
{ pkgs, inputs, ... }:

{
  home.packages = [
    inputs.saws.packages.${pkgs.system}.default
  ]
}
```

Or in a NixOS module:

```nix
{ pkgs, inputs, ... }:

{
  environment.systemPackages = [
    inputs.saws.packages.${pkgs.system}.default
  ]
}
```

> **Note:** You must pass `inputs` through to your module via `extraSpecialArgs` (Home Manager) or `specialArgs` (NixOS).

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
6. On next run, select a profile and get credentials

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

### Nix users

`saws init` resolves symlinks and hardcodes the Nix store path, which breaks after updates or garbage collection. Add this snippet to your `~/.zshrc` instead:

```sh
# >>> saws initialize >>>
saws() {
  local SAWS_BIN
  SAWS_BIN="$(which saws)"

  case "$1" in
    init|--version|--configure|configure)
      SAWS_WRAPPER=1 "$SAWS_BIN" "$@"
      return $?
      ;;
  esac

  local export_output
  export_output="$(SAWS_WRAPPER=1 "$SAWS_BIN" --export "$@")"
  local exit_code=$?

  if [ $exit_code -eq 0 ]; then
    eval "$export_output"
  else
    SAWS_WRAPPER=1 "$SAWS_BIN" "$@"
  fi
}
# <<< saws initialize <<<
```

This dynamically resolves the binary path at runtime so it survives Nix updates.

### Using AWS_PROFILE

Since saws populates the SSO token cache, you can skip the shell wrapper and use `AWS_PROFILE` directly:

```sh
export AWS_PROFILE=my-account-admin
aws sts get-caller-identity
terraform plan
```

All AWS tools (CLI, SDKs, Terraform, boto3) will resolve credentials through the cached SSO token.

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

saws also writes SSO tokens to `~/.aws/sso/cache/` in standard AWS CLI format. This means `AWS_PROFILE` works with any AWS tool without needing explicit credentials.

## License

MIT
