// Package shell handles shell wrapper generation and installation for saws.
// Since a child process cannot modify its parent's environment, saws uses
// a shell function wrapper that evals the binary's --export output.
// This is the same pattern used by nvm, rbenv, direnv, and aws-vault.
package shell

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Shell represents a supported shell type.
type Shell string

const (
	// Bash is the bash shell.
	Bash Shell = "bash"
	// Zsh is the zsh shell.
	Zsh Shell = "zsh"
	// Fish is the fish shell.
	Fish Shell = "fish"
)

// WrapperEnvVar is the environment variable set by the shell wrapper
// so the binary can detect whether it's running inside the wrapper.
const WrapperEnvVar = "SAWS_WRAPPER"

// beginMarker and endMarker delimit the managed block in rc files.
const (
	beginMarker = "# >>> saws initialize >>>"
	endMarker   = "# <<< saws initialize <<<"
)

// SupportedShells returns the list of supported shell names.
func SupportedShells() []string {
	return []string{string(Bash), string(Zsh), string(Fish)}
}

// ParseShell parses a shell name string into a Shell type.
func ParseShell(name string) (Shell, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "bash":
		return Bash, nil
	case "zsh":
		return Zsh, nil
	case "fish":
		return Fish, nil
	default:
		return "", fmt.Errorf("unsupported shell %q (supported: %s)", name, strings.Join(SupportedShells(), ", "))
	}
}

// DetectShell tries to determine the current shell from the SHELL environment variable.
func DetectShell() (Shell, error) {
	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		return "", fmt.Errorf("SHELL environment variable not set; specify your shell explicitly with: saws init <shell>")
	}
	base := filepath.Base(shellPath)
	return ParseShell(base)
}

// RCFile returns the path to the shell's rc file.
func RCFile(sh Shell) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}

	switch sh {
	case Bash:
		// On macOS, bash uses .bash_profile for login shells.
		// On Linux, .bashrc is more common.
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, ".bash_profile"), nil
		}
		return filepath.Join(home, ".bashrc"), nil
	case Zsh:
		return filepath.Join(home, ".zshrc"), nil
	case Fish:
		return filepath.Join(home, ".config", "fish", "config.fish"), nil
	default:
		return "", fmt.Errorf("unsupported shell: %s", sh)
	}
}

// BinaryPath returns the path to the saws binary.
// It first checks if the binary is in PATH, then falls back to the current executable.
func BinaryPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot determine saws binary path: %w", err)
	}
	// Resolve symlinks for a stable path
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return exe, nil // fallback to unresolved
	}
	return resolved, nil
}

// WrapperScript generates the shell wrapper function for the given shell.
// The wrapper:
//  1. Sets SAWS_WRAPPER=1 so the binary knows it's wrapped
//  2. Runs the binary with --export and any extra args
//  3. Evals the output to set env vars in the parent shell
//  4. Falls through to the real binary for non-credential flows (configure, version, etc.)
func WrapperScript(sh Shell, binaryPath string) string {
	switch sh {
	case Fish:
		return fishWrapper(binaryPath)
	default:
		// bash and zsh use the same POSIX-compatible syntax
		return posixWrapper(binaryPath)
	}
}

func posixWrapper(binaryPath string) string {
	return fmt.Sprintf(`%s
saws() {
  local SAWS_BIN="%s"

  # Pass-through commands that don't need eval
  case "$1" in
    init|--version|--configure|configure)
      SAWS_WRAPPER=1 "$SAWS_BIN" "$@"
      return $?
      ;;
  esac

  # Single invocation: export commands on stdout, display on stderr
  local export_output
  export_output="$(SAWS_WRAPPER=1 "$SAWS_BIN" --export "$@")"
  local exit_code=$?

  if [ $exit_code -eq 0 ]; then
    eval "$export_output"
  else
    # On failure, run interactively so the user sees errors
    SAWS_WRAPPER=1 "$SAWS_BIN" "$@"
  fi
}
%s`, beginMarker, binaryPath, endMarker)
}

func fishWrapper(binaryPath string) string {
	return fmt.Sprintf(`%s
function saws
  set -l SAWS_BIN "%s"

  # Pass-through commands that don't need eval
  switch $argv[1]
    case init --version --configure configure
      SAWS_WRAPPER=1 $SAWS_BIN $argv
      return $status
  end

  # Single invocation: export commands on stdout, display on stderr
  set -l export_output (SAWS_WRAPPER=1 $SAWS_BIN --export $argv)
  set -l exit_code $status

  if test $exit_code -eq 0
    eval $export_output
  else
    # On failure, run interactively so the user sees errors
    SAWS_WRAPPER=1 $SAWS_BIN $argv
  end
end
%s`, beginMarker, binaryPath, endMarker)
}

// Install adds the saws wrapper function to the shell's rc file.
// If the block already exists, it replaces it. Otherwise, it appends it.
func Install(sh Shell, binaryPath string, rcPath string) error {
	wrapper := WrapperScript(sh, binaryPath)

	// Read existing rc file content (might not exist yet)
	content, err := os.ReadFile(rcPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read %s: %w", rcPath, err)
	}

	existingContent := string(content)
	newContent := replaceOrAppendBlock(existingContent, wrapper)

	// Ensure parent directory exists (for fish config)
	dir := filepath.Dir(rcPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	if err := os.WriteFile(rcPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", rcPath, err)
	}

	return nil
}

// Uninstall removes the saws wrapper function from the shell's rc file.
func Uninstall(rcPath string) error {
	content, err := os.ReadFile(rcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // nothing to uninstall
		}
		return fmt.Errorf("failed to read %s: %w", rcPath, err)
	}

	newContent := removeBlock(string(content))
	if err := os.WriteFile(rcPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", rcPath, err)
	}

	return nil
}

// IsInstalled checks if the saws wrapper is present in the given rc file.
func IsInstalled(rcPath string) bool {
	content, err := os.ReadFile(rcPath)
	if err != nil {
		return false
	}
	return strings.Contains(string(content), beginMarker)
}

// IsWrapped returns true if the current process is running inside the shell wrapper.
func IsWrapped() bool {
	return os.Getenv(WrapperEnvVar) == "1"
}

// replaceOrAppendBlock replaces an existing managed block or appends a new one.
func replaceOrAppendBlock(content, block string) string {
	start := strings.Index(content, beginMarker)
	end := strings.Index(content, endMarker)

	if start >= 0 && end >= 0 {
		// Replace existing block
		return content[:start] + block + content[end+len(endMarker):]
	}

	// Append with a blank line separator
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if content != "" {
		content += "\n"
	}
	return content + block + "\n"
}

// removeBlock removes the managed block from content.
func removeBlock(content string) string {
	start := strings.Index(content, beginMarker)
	end := strings.Index(content, endMarker)

	if start < 0 || end < 0 {
		return content
	}

	before := content[:start]
	after := content[end+len(endMarker):]

	// Clean up extra blank lines
	result := before + after
	result = strings.TrimRight(result, "\n") + "\n"
	return result
}
