package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestRootUsageIncludesWebSessionGroup(t *testing.T) {
	root := RootCommand("1.2.3")
	usage := root.UsageFunc(root)

	if !strings.Contains(usage, "WEB SESSION COMMANDS") {
		t.Fatalf("expected web session group in root usage, got %q", usage)
	}
	if !strings.Contains(usage, "  web:") {
		t.Fatalf("expected web command in root usage, got %q", usage)
	}
}

func TestWebCommandUsesProductionHelpContract(t *testing.T) {
	root := RootCommand("1.2.3")
	webCmd := findSubcommand(root, "web")
	if webCmd == nil {
		t.Fatal("expected web command")
		return
	}

	usage := webCmd.UsageFunc(webCmd)
	if !strings.Contains(usage, "WEB SESSION WORKFLOWS") {
		t.Fatalf("expected web-session help heading, got %q", usage)
	}
	lowerUsage := strings.ToLower(usage)
	for _, token := range []string{"experimental", "unofficial", "discouraged", "private", "risk"} {
		if !strings.Contains(lowerUsage, token) {
			continue
		}
		t.Fatalf("expected %q not to appear in web usage, got %q", token, usage)
	}
}

func TestWebAppsCreateSubcommandIsRegistered(t *testing.T) {
	root := RootCommand("1.2.3")
	if sub := findSubcommand(root, "web", "apps", "create"); sub == nil {
		t.Fatalf("expected web apps create to be registered")
	}
}

func TestWebRemovedAppsListSubcommandIsRegistered(t *testing.T) {
	root := RootCommand("1.2.3")
	if sub := findSubcommand(root, "web", "removed-apps", "list"); sub == nil {
		t.Fatalf("expected web removed-apps list to be registered")
	}
}

func TestWebAppsDeleteSubcommandIsRegistered(t *testing.T) {
	root := RootCommand("1.2.3")
	if sub := findSubcommand(root, "web", "apps", "delete"); sub == nil {
		t.Fatalf("expected web apps delete to be registered")
	}
}

func TestWebAppsMedicalDeviceSetSubcommandIsRegistered(t *testing.T) {
	root := RootCommand("1.2.3")
	if sub := findSubcommand(root, "web", "apps", "medical-device", "set"); sub == nil {
		t.Fatalf("expected web apps medical-device set to be registered")
	}
}

func TestWebBundleIDCapabilitiesSyncAppClipSubcommandIsRegistered(t *testing.T) {
	root := RootCommand("1.2.3")
	if sub := findSubcommand(root, "web", "bundle-ids", "capabilities", "sync-app-clip"); sub == nil {
		t.Fatalf("expected web bundle-ids capabilities sync-app-clip to be registered")
	}
}

func TestWebBundleIDCapabilitiesEnableSubcommandIsRegistered(t *testing.T) {
	root := RootCommand("1.2.3")
	sub := findSubcommand(root, "web", "bundle-ids", "capabilities", "enable")
	if sub == nil {
		t.Fatalf("expected web bundle-ids capabilities enable to be registered")
	}
	for _, flagName := range []string{"bundle-id", "capability", "confirm", "apple-id", "output"} {
		if sub.FlagSet.Lookup(flagName) == nil {
			t.Fatalf("expected --%s flag", flagName)
		}
	}
}

func TestWebSandboxCreateSubcommandIsRegistered(t *testing.T) {
	root := RootCommand("1.2.3")
	if sub := findSubcommand(root, "web", "sandbox", "create"); sub == nil {
		t.Fatalf("expected web sandbox create to be registered")
	}
}

func TestWebAuthCapabilitiesSubcommandIsRegistered(t *testing.T) {
	root := RootCommand("1.2.3")
	if sub := findSubcommand(root, "web", "auth", "capabilities"); sub == nil {
		t.Fatalf("expected web auth capabilities to be registered")
	}
}

func TestWebXcodeCloudWorkflowsCreateSubcommandIsRegistered(t *testing.T) {
	root := RootCommand("1.2.3")
	if sub := findSubcommand(root, "web", "xcode-cloud", "workflows", "create"); sub == nil {
		t.Fatalf("expected web xcode-cloud workflows create to be registered")
	}
}

func TestWebAppsCreateMissingRequiredFlags(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"web", "apps", "create", "--name", "My App"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "Error: missing required flags: --bundle-id, --sku") {
		t.Fatalf("expected aggregated missing-flags error, got %q", stderr)
	}
}

func TestWebAppsCreateExposesPasswordCompatibilityFlag(t *testing.T) {
	root := RootCommand("1.2.3")
	cmd := findSubcommand(root, "web", "apps", "create")
	if cmd == nil {
		t.Fatal("expected web apps create command")
		return
	}
	if cmd.FlagSet.Lookup("password") == nil {
		t.Fatal("expected temporary password compatibility flag on web apps create")
	}
}

func TestWebSandboxCreateMissingRequiredFlags(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"web", "sandbox", "create"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "Error: --first-name is required") {
		t.Fatalf("expected missing --first-name error, got %q", stderr)
	}
}

func TestWebXcodeCloudWorkflowsCreateMissingRequiredFlags(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	_, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"web", "xcode-cloud", "workflows", "create"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "Error: --product-id is required") {
		t.Fatalf("expected missing --product-id error, got %q", stderr)
	}
}

func TestWebAuthLoginExposesDeprecatedTwoFactorAliasWithoutPlaintextPasswordFlag(t *testing.T) {
	root := RootCommand("1.2.3")
	cmd := findSubcommand(root, "web", "auth", "login")
	if cmd == nil {
		t.Fatal("expected web auth login command")
		return
	}
	if cmd.FlagSet.Lookup("password") != nil {
		t.Fatal("did not expect --password flag on web auth login")
	}
	if cmd.FlagSet.Lookup("password-stdin") != nil {
		t.Fatal("did not expect --password-stdin flag on web auth login")
	}
	twoFactorCodeFlag := cmd.FlagSet.Lookup("two-factor-code")
	if twoFactorCodeFlag == nil {
		t.Fatal("expected deprecated --two-factor-code flag on web auth login")
		return
	}
	if !strings.Contains(twoFactorCodeFlag.Usage, "Deprecated:") {
		t.Fatalf("expected deprecated help text for --two-factor-code, got %q", twoFactorCodeFlag.Usage)
	}
	if cmd.FlagSet.Lookup("two-factor-code-command") == nil {
		t.Fatal("expected --two-factor-code-command flag on web auth login")
	}
}

func TestWebAppsCreateExposesDeprecatedTwoFactorAlias(t *testing.T) {
	root := RootCommand("1.2.3")
	cmd := findSubcommand(root, "web", "apps", "create")
	if cmd == nil {
		t.Fatal("expected web apps create command")
		return
	}

	twoFactorCodeFlag := cmd.FlagSet.Lookup("two-factor-code")
	if twoFactorCodeFlag == nil {
		t.Fatal("expected deprecated --two-factor-code flag on web apps create")
		return
	}
	if !strings.Contains(twoFactorCodeFlag.Usage, "Deprecated:") {
		t.Fatalf("expected deprecated help text for --two-factor-code, got %q", twoFactorCodeFlag.Usage)
	}
	if cmd.FlagSet.Lookup("two-factor-code-command") == nil {
		t.Fatal("expected --two-factor-code-command flag on web apps create")
	}
}

func TestWebSandboxCreateExposesDeprecatedTwoFactorAlias(t *testing.T) {
	root := RootCommand("1.2.3")
	cmd := findSubcommand(root, "web", "sandbox", "create")
	if cmd == nil {
		t.Fatal("expected web sandbox create command")
		return
	}

	twoFactorCodeFlag := cmd.FlagSet.Lookup("two-factor-code")
	if twoFactorCodeFlag == nil {
		t.Fatal("expected deprecated --two-factor-code flag on web sandbox create")
		return
	}
	if !strings.Contains(twoFactorCodeFlag.Usage, "Deprecated:") {
		t.Fatalf("expected deprecated help text for --two-factor-code, got %q", twoFactorCodeFlag.Usage)
	}
	if cmd.FlagSet.Lookup("two-factor-code-command") == nil {
		t.Fatal("expected --two-factor-code-command flag on web sandbox create")
	}
}
