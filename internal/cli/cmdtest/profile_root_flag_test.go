package cmdtest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
)

// writeProfileFixtureConfig creates an isolated config store holding two named
// credentials so that profile selection is observable without a network call.
func writeProfileFixtureConfig(t *testing.T) {
	t.Helper()

	home := setCmdtestHome(t)
	configPath := filepath.Join(home, "config.json")
	prodKeyPath := filepath.Join(home, "AuthKey_PROD.p8")
	stagingKeyPath := filepath.Join(home, "AuthKey_STAGING.p8")
	writeECDSAPEM(t, prodKeyPath)
	writeECDSAPEM(t, stagingKeyPath)

	cfg := &config.Config{
		DefaultKeyName: "prod",
		Keys: []config.Credential{
			{Name: "prod", KeyID: "KEYPROD", IssuerID: "ISSPROD", PrivateKeyPath: prodKeyPath},
			{Name: "staging", KeyID: "KEYSTAGE", IssuerID: "ISSSTAGE", PrivateKeyPath: stagingKeyPath},
		},
	}
	payload, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(configPath, payload, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("ASC_CONFIG_PATH", configPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")

	previousProfile := shared.SelectedProfile()
	shared.SetSelectedProfile("")
	t.Cleanup(func() { shared.SetSelectedProfile(previousProfile) })
}

func runProfileSelection(t *testing.T, args []string) string {
	t.Helper()

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = rootcmd.Run(args, "1.2.3")
	})
	if code != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d (stderr %q)", code, rootcmd.ExitSuccess, stderr)
	}

	var payload struct {
		Profile string `json:"profile"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("decode auth status output %q: %v", stdout, err)
	}
	return payload.Profile
}

// TestProfileFlagResolvesInEitherPosition covers the issue-2601 acceptance
// criterion: the root-owned `--profile` selector resolves identically before
// and after the command name.
func TestProfileFlagResolvesInEitherPosition(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "before command", args: []string{"--profile", "staging", "auth", "status", "--output", "json"}},
		{name: "after command", args: []string{"auth", "status", "--output", "json", "--profile", "staging"}},
		{name: "after command inline", args: []string{"auth", "status", "--output", "json", "--profile=staging"}},
		{name: "between command and flags", args: []string{"auth", "status", "--profile", "staging", "--output", "json"}},
		{name: "after group", args: []string{"auth", "--profile", "staging", "status", "--output", "json"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			writeProfileFixtureConfig(t)

			if profile := runProfileSelection(t, test.args); profile != "staging" {
				t.Fatalf("profile = %q, want %q", profile, "staging")
			}
		})
	}
}

// TestProfileFlagAfterCommandReachesCredentialResolution proves the hoisted
// selector is the one credential lookup uses, not just a parsed value.
func TestProfileFlagAfterCommandReachesCredentialResolution(t *testing.T) {
	for _, args := range [][]string{
		{"--profile", "ghost", "apps", "list"},
		{"apps", "list", "--profile", "ghost"},
		{"apps", "list", "--profile=ghost"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			writeProfileFixtureConfig(t)

			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run(args, "1.2.3")
			})
			if code == rootcmd.ExitSuccess {
				t.Fatalf("exit code = %d, want a failure for a missing profile", code)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, `credentials not found for profile "ghost"`) {
				t.Fatalf("stderr = %q, want the selected profile named", stderr)
			}
		})
	}
}

// TestProfileFlagAfterCommandKeepsLastValue preserves the standard flag
// package's left-to-right precedence when the selector is written twice.
func TestProfileFlagAfterCommandKeepsLastValue(t *testing.T) {
	writeProfileFixtureConfig(t)

	profile := runProfileSelection(t, []string{
		"--profile", "prod", "auth", "status", "--output", "json", "--profile", "staging",
	})
	if profile != "staging" {
		t.Fatalf("profile = %q, want the last value %q", profile, "staging")
	}
}

// TestProfileFlagAfterCommandWithoutValueIsUsageError keeps a value-less
// selector a usage failure instead of letting it consume a later token.
func TestProfileFlagAfterCommandWithoutValueIsUsageError(t *testing.T) {
	for _, args := range [][]string{
		{"auth", "status", "--profile"},
		// `status` names a subcommand of `auth`, so it is a misplaced command
		// name rather than a profile value.
		{"auth", "--profile", "status", "--output", "json"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			writeProfileFixtureConfig(t)

			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run(args, "1.2.3")
			})
			if code != rootcmd.ExitUsage {
				t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			want := "Error: `--profile` needs a profile name; pass `--profile=NAME`.\n" +
				"For help:\n  asc --help\n"
			if stderr != want {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
		})
	}
}

// TestProfileFlagAfterSpacedBooleanFlag keeps the CLI's liberal `--flag false`
// support working on either side of a relocated selector.
func TestProfileFlagAfterSpacedBooleanFlag(t *testing.T) {
	for _, args := range [][]string{
		{"apps", "list", "--paginate", "false", "--profile", "ghost"},
		{"apps", "list", "--profile", "ghost", "--paginate", "false"},
		{"apps", "list", "--paginate", "--profile", "ghost"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			writeProfileFixtureConfig(t)

			var code int
			stdout, stderr := captureOutput(t, func() {
				code = rootcmd.Run(args, "1.2.3")
			})
			if code == rootcmd.ExitSuccess {
				t.Fatalf("exit code = %d, want a failure for a missing profile", code)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, `credentials not found for profile "ghost"`) {
				t.Fatalf("stderr = %q, want the selected profile named", stderr)
			}
		})
	}
}

// TestCommandOwnedProfileFlagIsNotRelocated keeps `asc signing run --profile`
// a provisioning-profile path owned by that command.
func TestCommandOwnedProfileFlagIsNotRelocated(t *testing.T) {
	writeProfileFixtureConfig(t)

	var code int
	_, stderr := captureOutput(t, func() {
		code = rootcmd.Run([]string{"signing", "run", "--profile", "app.mobileprovision"}, "1.2.3")
	})
	if code != rootcmd.ExitUsage {
		t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
	}
	if !strings.Contains(stderr, "Error: --identity is required") {
		t.Fatalf("stderr = %q, want the command's own validation", stderr)
	}
	if strings.Contains(stderr, "credentials not found for profile") {
		t.Fatalf("stderr = %q, want no credential-profile lookup", stderr)
	}
}

// TestEmptyProfileFlagValueMatchesRootPlacement pins the parity the issue asks
// for: an explicitly empty selector clears the profile override in either
// position, so `--profile="$UNSET_VAR"` keeps working in scripts.
func TestEmptyProfileFlagValueMatchesRootPlacement(t *testing.T) {
	for _, args := range [][]string{
		{"--profile=", "auth", "status", "--output", "json"},
		{"auth", "status", "--output", "json", "--profile="},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			writeProfileFixtureConfig(t)

			if profile := runProfileSelection(t, args); profile != "" {
				t.Fatalf("profile = %q, want no profile override", profile)
			}
		})
	}
}
