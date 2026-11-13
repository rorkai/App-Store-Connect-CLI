package cmdtest

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

// seedLogoutConfigs writes a "shared" profile plus one unique profile to the
// override and global config files and returns the global file's bytes.
func seedLogoutConfigs(t *testing.T, overridePath, globalPath string) []byte {
	t.Helper()

	if err := config.SaveAt(overridePath, &config.Config{
		DefaultKeyName: "shared",
		Keys: []config.Credential{
			{Name: "shared", KeyID: "OVERRIDE_SHARED", IssuerID: "ISS", PrivateKeyPath: "/tmp/override-shared.p8"},
			{Name: "override-only", KeyID: "OVERRIDE_ONLY", IssuerID: "ISS", PrivateKeyPath: "/tmp/override-only.p8"},
		},
	}); err != nil {
		t.Fatalf("SaveAt(override) error: %v", err)
	}
	if err := config.SaveAt(globalPath, &config.Config{
		DefaultKeyName: "shared",
		Keys: []config.Credential{
			{Name: "shared", KeyID: "GLOBAL_SHARED", IssuerID: "ISS", PrivateKeyPath: "/tmp/global-shared.p8"},
			{Name: "global-only", KeyID: "GLOBAL_ONLY", IssuerID: "ISS", PrivateKeyPath: "/tmp/global-only.p8"},
		},
	}); err != nil {
		t.Fatalf("SaveAt(global) error: %v", err)
	}
	data, err := os.ReadFile(globalPath)
	if err != nil {
		t.Fatalf("ReadFile(global) error: %v", err)
	}
	return data
}

func requireFileUnchanged(t *testing.T, path string, before []byte) {
	t.Helper()

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error: %v", path, err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("%s changed:\nbefore: %s\nafter: %s", path, before, after)
	}
}

func isolateScopedLogout(t *testing.T) (string, string) {
	t.Helper()

	globalPath := isolateAuthConfigWrite(t)
	overridePath := filepath.Join(t.TempDir(), "isolated", "config.json")
	t.Setenv("ASC_CONFIG_PATH", overridePath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	return overridePath, globalPath
}

func TestAuthLogoutAllWithConfigPathOverrideLeavesGlobalConfigAndWarns(t *testing.T) {
	overridePath, globalPath := isolateScopedLogout(t)
	globalBefore := seedLogoutConfigs(t, overridePath, globalPath)

	stdout, stderr := runAuthConfigCommand(t, "auth", "logout", "--all", "--confirm")

	if stdout != "Successfully removed stored credentials\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	requireConfigProfiles(t, overridePath)
	requireFileUnchanged(t, globalPath, globalBefore)
	for _, want := range []string{
		"Warning: ASC_CONFIG_PATH is set, so auth logout did not change " + globalPath,
		"still holds stored credentials",
		"run auth logout with ASC_CONFIG_PATH unset or pass --include-global",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
		}
	}
}

func TestAuthLogoutAllIncludeGlobalClearsBothConfigsWithoutWarning(t *testing.T) {
	overridePath, globalPath := isolateScopedLogout(t)
	seedLogoutConfigs(t, overridePath, globalPath)

	stdout, stderr := runAuthConfigCommand(t, "auth", "logout", "--all", "--include-global", "--confirm")

	if stdout != "Successfully removed stored credentials\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if stderr != authLogoutBypassNote {
		t.Fatalf("expected no warning with --include-global, got %q", stderr)
	}
	requireConfigProfiles(t, overridePath)
	requireConfigProfiles(t, globalPath)
}

func TestAuthLogoutNamedWithConfigPathOverrideLeavesGlobalConfigAndWarns(t *testing.T) {
	overridePath, globalPath := isolateScopedLogout(t)
	globalBefore := seedLogoutConfigs(t, overridePath, globalPath)

	stdout, stderr := runAuthConfigCommand(t, "auth", "logout", "--name", "shared", "--confirm")

	if stdout != "Successfully removed stored credential 'shared'\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	requireConfigProfiles(t, overridePath, "override-only")
	requireFileUnchanged(t, globalPath, globalBefore)
	want := "Warning: ASC_CONFIG_PATH is set, so auth logout did not change " + globalPath + ", which still holds credentials named 'shared'"
	if !strings.Contains(stderr, want) {
		t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
	}
}

func TestAuthLogoutNamedWithConfigPathOverrideSkipsWarningWhenGlobalLacksProfile(t *testing.T) {
	overridePath, globalPath := isolateScopedLogout(t)
	globalBefore := seedLogoutConfigs(t, overridePath, globalPath)

	stdout, stderr := runAuthConfigCommand(t, "auth", "logout", "--name", "override-only", "--confirm")

	if stdout != "Successfully removed stored credential 'override-only'\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if stderr != authLogoutBypassNote {
		t.Fatalf("expected no warning when the global config lacks the profile, got %q", stderr)
	}
	requireConfigProfiles(t, overridePath, "shared")
	requireFileUnchanged(t, globalPath, globalBefore)
}

func TestAuthLogoutNamedGlobalOnlyProfileWithMissingOverrideFileFails(t *testing.T) {
	overridePath, globalPath := isolateScopedLogout(t)
	globalBefore := seedLogoutConfigs(t, overridePath, globalPath)
	if err := os.Remove(overridePath); err != nil {
		t.Fatalf("Remove(override) error: %v", err)
	}

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"auth", "logout", "--name", "global-only", "--confirm"}, "1.0.0")
	})

	if code != cmd.ExitError {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, cmd.ExitError, stderr)
	}
	if stdout != "" {
		t.Fatalf("expected no success message, got %q", stdout)
	}
	for _, want := range []string{
		"which still holds credentials named 'global-only'",
		"auth logout: failed to remove credentials",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
		}
	}
	requireFileUnchanged(t, globalPath, globalBefore)
	requireNoConfigFile(t, overridePath)
}

func TestAuthLogoutNamedIncludeGlobalRemovesFromBothConfigs(t *testing.T) {
	overridePath, globalPath := isolateScopedLogout(t)
	seedLogoutConfigs(t, overridePath, globalPath)

	_, stderr := runAuthConfigCommand(t, "auth", "logout", "--name", "shared", "--include-global", "--confirm")

	if stderr != authLogoutBypassNote {
		t.Fatalf("expected no warning with --include-global, got %q", stderr)
	}
	requireConfigProfiles(t, overridePath, "override-only")
	requireConfigProfiles(t, globalPath, "global-only")
}

func TestAuthLogoutWithoutConfigPathOverrideClearsGlobalConfig(t *testing.T) {
	globalPath := isolateAuthConfigWrite(t)
	t.Setenv("ASC_CONFIG_PATH", "")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	if err := config.SaveAt(globalPath, &config.Config{
		Keys: []config.Credential{{Name: "global-only", KeyID: "GLOBAL_ONLY", IssuerID: "ISS", PrivateKeyPath: "/tmp/global-only.p8"}},
	}); err != nil {
		t.Fatalf("SaveAt(global) error: %v", err)
	}

	stdout, stderr := runAuthConfigCommand(t, "auth", "logout", "--all", "--confirm")

	if stdout != "Successfully removed stored credentials\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if stderr != authLogoutBypassNote {
		t.Fatalf("expected no warning without ASC_CONFIG_PATH, got %q", stderr)
	}
	requireConfigProfiles(t, globalPath)
}
