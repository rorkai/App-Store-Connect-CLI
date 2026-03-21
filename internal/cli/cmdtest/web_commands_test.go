package cmdtest

import (
	"context"
	"errors"
	"flag"
	"io"
	"strings"
	"testing"
)

func TestRootUsageIncludesExperimentalWebGroup(t *testing.T) {
	root := RootCommand("1.2.3")
	usage := root.UsageFunc(root)

	if !strings.Contains(usage, "EXPERIMENTAL COMMANDS") {
		t.Fatalf("expected experimental group in root usage, got %q", usage)
	}
	if !strings.Contains(usage, "  web:") {
		t.Fatalf("expected web command in root usage, got %q", usage)
	}
}

func TestWebCommandIncludesWarningContract(t *testing.T) {
	root := RootCommand("1.2.3")
	webCmd := findSubcommand(root, "web")
	if webCmd == nil {
		t.Fatal("expected web command")
	}

	usage := webCmd.UsageFunc(webCmd)
	for _, token := range []string{"EXPERIMENTAL", "UNOFFICIAL", "DISCOURAGED"} {
		if !strings.Contains(usage, token) {
			t.Fatalf("expected %q token in web usage, got %q", token, usage)
		}
	}
}

func TestWebAppsCreateSubcommandIsRegistered(t *testing.T) {
	root := RootCommand("1.2.3")
	if sub := findSubcommand(root, "web", "apps", "create"); sub == nil {
		t.Fatalf("expected web apps create to be registered")
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
		if err := root.Parse([]string{"web", "apps", "create"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if !strings.Contains(stderr, "Error: --name is required") {
		t.Fatalf("expected missing --name error, got %q", stderr)
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
	}

	twoFactorCodeFlag := cmd.FlagSet.Lookup("two-factor-code")
	if twoFactorCodeFlag == nil {
		t.Fatal("expected deprecated --two-factor-code flag on web apps create")
	}
	if !strings.Contains(twoFactorCodeFlag.Usage, "Deprecated:") {
		t.Fatalf("expected deprecated help text for --two-factor-code, got %q", twoFactorCodeFlag.Usage)
	}
	if cmd.FlagSet.Lookup("two-factor-code-command") == nil {
		t.Fatal("expected --two-factor-code-command flag on web apps create")
	}
}
