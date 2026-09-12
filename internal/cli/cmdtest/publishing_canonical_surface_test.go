package cmdtest

import (
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestPublishHelpShowsCanonicalAppStoreAndTestFlightSurfaces(t *testing.T) {
	stdout, stderr, runErr := runRootCommand(t, []string{"publish"})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !usageListsSubcommand(stderr, "testflight") {
		t.Fatalf("expected publish help to list testflight, got %q", stderr)
	}
	if !usageListsSubcommand(stderr, "appstore") {
		t.Fatalf("expected publish help to list canonical appstore path, got %q", stderr)
	}
	if !strings.Contains(stderr, "asc publish appstore") {
		t.Fatalf("expected publish help to point App Store users to asc publish appstore, got %q", stderr)
	}
}

func TestSubmitHelpShowsLifecycleCommandsAndHidesDeprecatedCreate(t *testing.T) {
	stdout, stderr, runErr := runRootCommand(t, []string{"submit"})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	for _, subcommand := range []string{"status", "cancel"} {
		if !usageListsSubcommand(stderr, subcommand) {
			t.Fatalf("expected submit help to list %s, got %q", subcommand, stderr)
		}
	}
	if usageListsSubcommand(stderr, "preflight") {
		t.Fatalf("expected submit help to hide deprecated preflight path, got %q", stderr)
	}
	if !strings.Contains(stderr, "asc validate") {
		t.Fatalf("expected submit help text to mention canonical validate guidance, got %q", stderr)
	}
	if !strings.Contains(stderr, "asc submit status/cancel") {
		t.Fatalf("expected submit help text to mention visible submit lifecycle commands, got %q", stderr)
	}
	if usageListsSubcommand(stderr, "create") {
		t.Fatalf("expected submit help to hide deprecated create path, got %q", stderr)
	}
	if !strings.Contains(stderr, "asc publish appstore --submit") {
		t.Fatalf("expected submit help to point App Store users to asc publish appstore --submit, got %q", stderr)
	}
}

func TestPublishAppStoreHelpShowsCanonicalWorkflowGuidance(t *testing.T) {
	usage := usageForCommand(t, "publish", "appstore")

	if strings.Contains(usage, "DEPRECATED:") {
		t.Fatalf("expected canonical publish appstore help without deprecation banner, got %q", usage)
	}
	if !strings.Contains(usage, "canonical high-level App Store publish command") {
		t.Fatalf("expected canonical guidance in publish appstore help, got %q", usage)
	}
	if !strings.Contains(usage, "--ipa") {
		t.Fatalf("expected publish appstore help to show flag details, got %q", usage)
	}
	if !strings.Contains(usage, "--pkg") {
		t.Fatalf("expected publish appstore help to show macOS package uploads, got %q", usage)
	}
}

func TestPublishAppStoreInvocationDoesNotWarn(t *testing.T) {
	stdout, stderr, runErr := runRootCommand(t, []string{"publish", "appstore", "--app", "app-1", "--version", "1.0.0"})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --ipa or --pkg is required") {
		t.Fatalf("expected validation error for missing upload artifact, got %q", stderr)
	}
	if strings.Contains(stderr, "deprecated") {
		t.Fatalf("expected canonical publish appstore path to avoid deprecation warnings, got %q", stderr)
	}
}
