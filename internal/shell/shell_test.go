package shell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseShell(t *testing.T) {
	tests := []struct {
		input   string
		want    Shell
		wantErr bool
	}{
		{"bash", Bash, false},
		{"zsh", Zsh, false},
		{"fish", Fish, false},
		{"BASH", Bash, false},
		{" Zsh ", Zsh, false},
		{"sh", "", true},
		{"powershell", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseShell(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseShell(%q) expected error, got %q", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseShell(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseShell(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDetectShell(t *testing.T) {
	// Save and restore SHELL
	orig := os.Getenv("SHELL")
	defer os.Setenv("SHELL", orig)

	tests := []struct {
		name    string
		shell   string
		want    Shell
		wantErr bool
	}{
		{"bash", "/bin/bash", Bash, false},
		{"zsh", "/usr/bin/zsh", Zsh, false},
		{"fish", "/usr/local/bin/fish", Fish, false},
		{"empty", "", "", true},
		{"unsupported", "/bin/sh", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("SHELL", tt.shell)
			got, err := DetectShell()
			if tt.wantErr {
				if err == nil {
					t.Errorf("DetectShell() expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("DetectShell() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("DetectShell() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWrapperScript(t *testing.T) {
	binary := "/usr/local/bin/saws"

	t.Run("bash contains function and markers", func(t *testing.T) {
		script := WrapperScript(Bash, binary)
		if !strings.Contains(script, beginMarker) {
			t.Error("missing begin marker")
		}
		if !strings.Contains(script, endMarker) {
			t.Error("missing end marker")
		}
		if !strings.Contains(script, "saws()") {
			t.Error("missing function definition")
		}
		if !strings.Contains(script, binary) {
			t.Error("missing binary path")
		}
		if !strings.Contains(script, "SAWS_WRAPPER=1") {
			t.Error("missing SAWS_WRAPPER env var")
		}
		if !strings.Contains(script, "--export") {
			t.Error("missing --export flag")
		}
	})

	t.Run("zsh uses same POSIX syntax as bash", func(t *testing.T) {
		bashScript := WrapperScript(Bash, binary)
		zshScript := WrapperScript(Zsh, binary)
		if bashScript != zshScript {
			t.Error("expected bash and zsh wrappers to be identical")
		}
	})

	t.Run("fish uses fish syntax", func(t *testing.T) {
		script := WrapperScript(Fish, binary)
		if !strings.Contains(script, "function saws") {
			t.Error("missing fish function keyword")
		}
		if !strings.Contains(script, "$argv") {
			t.Error("missing fish $argv")
		}
		if !strings.Contains(script, "$status") {
			t.Error("missing fish $status")
		}
		// Should NOT contain bash syntax
		if strings.Contains(script, "saws()") {
			t.Error("fish wrapper should not contain bash function syntax")
		}
	})
}

func TestInstallAndUninstall(t *testing.T) {
	tmpDir := t.TempDir()
	rcPath := filepath.Join(tmpDir, ".bashrc")
	binary := "/usr/local/bin/saws"

	// Install into a new file
	err := Install(Bash, binary, rcPath)
	if err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	// Verify the file was created
	content, err := os.ReadFile(rcPath)
	if err != nil {
		t.Fatalf("failed to read rc file: %v", err)
	}
	contentStr := string(content)

	if !strings.Contains(contentStr, beginMarker) {
		t.Error("installed file missing begin marker")
	}
	if !strings.Contains(contentStr, endMarker) {
		t.Error("installed file missing end marker")
	}
	if !strings.Contains(contentStr, "saws()") {
		t.Error("installed file missing function definition")
	}

	// IsInstalled should return true
	if !IsInstalled(rcPath) {
		t.Error("IsInstalled() returned false after Install()")
	}

	// Uninstall
	err = Uninstall(rcPath)
	if err != nil {
		t.Fatalf("Uninstall() error: %v", err)
	}

	content, err = os.ReadFile(rcPath)
	if err != nil {
		t.Fatalf("failed to read rc file after uninstall: %v", err)
	}
	contentStr = string(content)

	if strings.Contains(contentStr, beginMarker) {
		t.Error("uninstalled file still contains begin marker")
	}
	if strings.Contains(contentStr, "saws()") {
		t.Error("uninstalled file still contains function definition")
	}

	// IsInstalled should return false
	if IsInstalled(rcPath) {
		t.Error("IsInstalled() returned true after Uninstall()")
	}
}

func TestInstallPreservesExistingContent(t *testing.T) {
	tmpDir := t.TempDir()
	rcPath := filepath.Join(tmpDir, ".zshrc")
	binary := "/usr/local/bin/saws"

	// Write some existing content
	existing := "# My zsh config\nexport PATH=$HOME/bin:$PATH\nalias ll='ls -la'\n"
	os.WriteFile(rcPath, []byte(existing), 0644)

	// Install
	err := Install(Zsh, binary, rcPath)
	if err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	content, err := os.ReadFile(rcPath)
	if err != nil {
		t.Fatalf("failed to read rc file: %v", err)
	}
	contentStr := string(content)

	// Existing content should be preserved
	if !strings.Contains(contentStr, "# My zsh config") {
		t.Error("existing content was not preserved")
	}
	if !strings.Contains(contentStr, "alias ll='ls -la'") {
		t.Error("existing alias was not preserved")
	}
	if !strings.Contains(contentStr, beginMarker) {
		t.Error("wrapper block was not added")
	}
}

func TestInstallReplacesExistingBlock(t *testing.T) {
	tmpDir := t.TempDir()
	rcPath := filepath.Join(tmpDir, ".bashrc")

	// Install with one binary path
	err := Install(Bash, "/old/path/saws", rcPath)
	if err != nil {
		t.Fatalf("first Install() error: %v", err)
	}

	// Install again with a different binary path
	err = Install(Bash, "/new/path/saws", rcPath)
	if err != nil {
		t.Fatalf("second Install() error: %v", err)
	}

	content, err := os.ReadFile(rcPath)
	if err != nil {
		t.Fatalf("failed to read rc file: %v", err)
	}
	contentStr := string(content)

	// Should contain the new path, not the old one
	if strings.Contains(contentStr, "/old/path/saws") {
		t.Error("old binary path was not replaced")
	}
	if !strings.Contains(contentStr, "/new/path/saws") {
		t.Error("new binary path was not found")
	}

	// Should only have one begin marker
	count := strings.Count(contentStr, beginMarker)
	if count != 1 {
		t.Errorf("expected 1 begin marker, got %d", count)
	}
}

func TestInstallFishCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	rcPath := filepath.Join(tmpDir, ".config", "fish", "config.fish")
	binary := "/usr/local/bin/saws"

	err := Install(Fish, binary, rcPath)
	if err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	content, err := os.ReadFile(rcPath)
	if err != nil {
		t.Fatalf("failed to read rc file: %v", err)
	}

	if !strings.Contains(string(content), "function saws") {
		t.Error("fish config missing function definition")
	}
}

func TestUninstallNonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	rcPath := filepath.Join(tmpDir, "nonexistent")

	// Should not error
	err := Uninstall(rcPath)
	if err != nil {
		t.Fatalf("Uninstall() on nonexistent file should not error, got: %v", err)
	}
}

func TestIsWrapped(t *testing.T) {
	orig := os.Getenv(WrapperEnvVar)
	defer os.Setenv(WrapperEnvVar, orig)

	os.Setenv(WrapperEnvVar, "1")
	if !IsWrapped() {
		t.Error("IsWrapped() should return true when SAWS_WRAPPER=1")
	}

	os.Setenv(WrapperEnvVar, "0")
	if IsWrapped() {
		t.Error("IsWrapped() should return false when SAWS_WRAPPER=0")
	}

	os.Unsetenv(WrapperEnvVar)
	if IsWrapped() {
		t.Error("IsWrapped() should return false when SAWS_WRAPPER is unset")
	}
}

func TestSupportedShells(t *testing.T) {
	shells := SupportedShells()
	if len(shells) != 3 {
		t.Errorf("expected 3 supported shells, got %d", len(shells))
	}

	expected := map[string]bool{"bash": true, "zsh": true, "fish": true}
	for _, s := range shells {
		if !expected[s] {
			t.Errorf("unexpected shell: %s", s)
		}
	}
}

func TestReplaceOrAppendBlock(t *testing.T) {
	block := beginMarker + "\nnew content\n" + endMarker

	t.Run("append to empty", func(t *testing.T) {
		result := replaceOrAppendBlock("", block)
		if !strings.Contains(result, "new content") {
			t.Error("block not appended")
		}
	})

	t.Run("append to existing content", func(t *testing.T) {
		result := replaceOrAppendBlock("existing\n", block)
		if !strings.HasPrefix(result, "existing\n") {
			t.Error("existing content not preserved")
		}
		if !strings.Contains(result, "new content") {
			t.Error("block not appended")
		}
	})

	t.Run("replace existing block", func(t *testing.T) {
		existing := "before\n" + beginMarker + "\nold content\n" + endMarker + "\nafter\n"
		result := replaceOrAppendBlock(existing, block)
		if strings.Contains(result, "old content") {
			t.Error("old block was not replaced")
		}
		if !strings.Contains(result, "new content") {
			t.Error("new block not inserted")
		}
		if !strings.HasPrefix(result, "before\n") {
			t.Error("content before block not preserved")
		}
		if !strings.HasSuffix(result, "\nafter\n") {
			t.Error("content after block not preserved")
		}
	})
}

func TestRemoveBlock(t *testing.T) {
	t.Run("no block present", func(t *testing.T) {
		content := "some content\n"
		result := removeBlock(content)
		if result != content {
			t.Errorf("content should be unchanged, got %q", result)
		}
	})

	t.Run("block present", func(t *testing.T) {
		content := "before\n" + beginMarker + "\nblock content\n" + endMarker + "\nafter"
		result := removeBlock(content)
		if strings.Contains(result, "block content") {
			t.Error("block was not removed")
		}
		if !strings.Contains(result, "before") {
			t.Error("content before block not preserved")
		}
		if !strings.Contains(result, "after") {
			t.Error("content after block not preserved")
		}
	})
}
