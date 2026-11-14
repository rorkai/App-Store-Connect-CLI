package cmdtest

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// Comma-separated list flags bound with shared.BindOnceCSVFlag must reject a
// repeated occurrence instead of silently keeping only the last one. These
// tests pin that contract end to end through the real command tree so the
// behavior cannot regress into silent data loss on a mutating command.

func TestRepeatedCSVFlagRejectedEndToEnd(t *testing.T) {
	isolateRepeatedCSVFlagEnv(t)

	stdout, stderr := captureOutput(t, func() {
		code := rootcmd.Run([]string{
			"profiles", "create",
			"--name", "Contract",
			"--profile-type", "IOS_APP_DEVELOPMENT",
			"--bundle", "BUNDLE_ID",
			"--certificate", "CERT_A",
			"--certificate", "CERT_B",
		}, "1.2.3")
		if code != rootcmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}

	wantMessage := repeatedCSVFlagMessage("certificate", "CERT_A", "CERT_B")
	if !strings.Contains(stderr, wantMessage) {
		t.Fatalf("stderr = %q, want it to contain %q", stderr, wantMessage)
	}

	// The rejection is a flag parse failure, not a help request: `--help` exits
	// 0 and starts with the usage banner, so the first stderr line has to be the
	// concise flag error itself.
	firstLine, _, _ := strings.Cut(stderr, "\n")
	wantFirstLine := fmt.Sprintf("Error: invalid value %q for flag -certificate: %s", "CERT_B", wantMessage)
	if firstLine != wantFirstLine {
		t.Fatalf("first stderr line = %q, want %q", firstLine, wantFirstLine)
	}
}

func TestRepeatedCSVFlagRejectedAcrossConvertedCommands(t *testing.T) {
	tests := []struct {
		name    string
		command []string
		flag    string
		deps    bool // the command is built on shared.Deps: run it through RunWithDeps
	}{
		{name: "nominations create app", command: []string{"nominations", "create"}, flag: "app", deps: true},
		{name: "nominations create device families", command: []string{"nominations", "create"}, flag: "device-families", deps: true},
		{name: "nominations create locales", command: []string{"nominations", "create"}, flag: "locales", deps: true},
		{name: "nominations create supplemental materials uris", command: []string{"nominations", "create"}, flag: "supplemental-materials-uris", deps: true},
		{name: "nominations create in-app events", command: []string{"nominations", "create"}, flag: "in-app-events", deps: true},
		{name: "nominations create supported territories", command: []string{"nominations", "create"}, flag: "supported-territories", deps: true},
		{name: "nominations update app", command: []string{"nominations", "update"}, flag: "app", deps: true},
		{name: "nominations update device families", command: []string{"nominations", "update"}, flag: "device-families", deps: true},
		{name: "nominations update locales", command: []string{"nominations", "update"}, flag: "locales", deps: true},
		{name: "nominations update supplemental materials uris", command: []string{"nominations", "update"}, flag: "supplemental-materials-uris", deps: true},
		{name: "nominations update in-app events", command: []string{"nominations", "update"}, flag: "in-app-events", deps: true},
		{name: "nominations update supported territories", command: []string{"nominations", "update"}, flag: "supported-territories", deps: true},
		{name: "profiles create certificate", command: []string{"profiles", "create"}, flag: "certificate"},
		{name: "profiles create device", command: []string{"profiles", "create"}, flag: "device"},
		{name: "win-back offers create price", command: []string{"subscriptions", "offers", "win-back", "create"}, flag: "price"},
		{name: "win-back offers create territory", command: []string{"subscriptions", "offers", "win-back", "create"}, flag: "territory"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isolateRepeatedCSVFlagEnv(t)

			args := append([]string{}, test.command...)
			args = append(args, "--"+test.flag, "ONE", "--"+test.flag, "TWO")

			if test.deps {
				assertUsageExitWithDeps(t, nil, args, repeatedCSVFlagMessage(test.flag, "ONE", "TWO"))
			} else {
				assertUsageExit(t, args, repeatedCSVFlagMessage(test.flag, "ONE", "TWO"))
			}
		})
	}
}

// An explicitly empty value stays a single occurrence, so the existing
// per-command validation still owns the outcome. This documents current
// behavior; the once-only binding does not change it.
func TestExplicitEmptyCSVFlagKeepsExistingValidation(t *testing.T) {
	t.Run("required flag reports missing input", func(t *testing.T) {
		isolateRepeatedCSVFlagEnv(t)

		assertUsageExit(t, []string{
			"profiles", "create",
			"--name", "Contract",
			"--profile-type", "IOS_APP_DEVELOPMENT",
			"--bundle", "BUNDLE_ID",
			"--certificate", "",
		}, "Error: --certificate is required")
	})

	t.Run("empty update flag stays a visited flag", func(t *testing.T) {
		isolateRepeatedCSVFlagEnv(t)

		assertUsageExitWithDeps(t, nil, []string{
			"nominations", "update",
			"--id", "NOMINATION_ID",
			"--submitted=false",
			"--locales", "",
		}, "Error: nominations update: --locales is required")
	})
}

func repeatedCSVFlagMessage(name, first, second string) string {
	return fmt.Sprintf(
		"--%s specified multiple times; pass one comma-separated list, for example --%s %q",
		name, name, first+","+second,
	)
}

func isolateRepeatedCSVFlagEnv(t *testing.T) {
	t.Helper()

	setCmdtestHome(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
}
