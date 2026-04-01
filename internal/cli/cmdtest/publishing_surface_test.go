package cmdtest

import (
	"errors"
	"flag"
	"strings"
	"testing"
)

const (
	releaseRunDeprecationWarning   = "Warning: `asc release run` is deprecated. Use `asc release stage`, then `asc review submissions-create` / `asc review items-add` / `asc review submissions-submit` for metadata workflows, or `asc publish appstore --submit` when local metadata is already synced."
	submitCreateDeprecationWarning = "Warning: `asc submit create` is deprecated. Use `asc versions attach-build` + `asc review submissions-*` for already-uploaded builds, or `asc publish appstore --submit` when starting from an IPA."
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
}

func TestSubmitCreateHelpShowsDeprecatedCompatibilityGuidance(t *testing.T) {
	usage := usageForCommand(t, "submit", "create")

	if !strings.Contains(usage, "DEPRECATED: use `asc versions attach-build` + `asc review submissions-*`.") {
		t.Fatalf("expected deprecated guidance in submit create help, got %q", usage)
	}
	if !strings.Contains(usage, "Deprecated compatibility path") {
		t.Fatalf("expected compatibility guidance in submit create help, got %q", usage)
	}
	for _, expected := range []string{
		"asc versions attach-build",
		`asc review submissions-create --app "APP_ID" --platform "PLATFORM"`,
		"asc review items-add",
		"asc review submissions-submit",
		"asc publish appstore --submit",
	} {
		if !strings.Contains(usage, expected) {
			t.Fatalf("expected submit create help to mention %q, got %q", expected, usage)
		}
	}
	if strings.Contains(usage, "\nFLAGS\n") {
		t.Fatalf("expected deprecated submit create help to hide legacy flag details, got %q", usage)
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
	if !strings.Contains(stderr, "Error: --ipa is required") {
		t.Fatalf("expected validation error for missing ipa, got %q", stderr)
	}
	if strings.Contains(stderr, releaseRunDeprecationWarning) {
		t.Fatalf("expected canonical publish appstore path to avoid release-run deprecation warning, got %q", stderr)
	}
}

func TestDeprecatedSubmitCreateInvocationWarns(t *testing.T) {
	stdout, stderr, runErr := runRootCommand(t, []string{"submit", "create", "--version", "1.0.0", "--version-id", "version-1", "--build", "build-1", "--confirm"})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	requireStderrContainsWarning(t, stderr, submitCreateDeprecationWarning)
	if !strings.Contains(stderr, "--version and --version-id are mutually exclusive") {
		t.Fatalf("expected legacy validation error after deprecation warning, got %q", stderr)
	}
}

func TestDeprecatedReleaseRunInvocationWarns(t *testing.T) {
	stdout, stderr, runErr := runRootCommand(t, []string{"release", "run", "--app", "app-1", "--version", "1.0.0", "--build", "build-1", "--dry-run"})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	requireStderrContainsWarning(t, stderr, releaseRunDeprecationWarning)
	if !strings.Contains(stderr, "--metadata-dir is required") {
		t.Fatalf("expected validation error after deprecation warning, got %q", stderr)
	}
}

func TestDeprecatedReleaseRunHelpShowsMetadataPreservingReplacement(t *testing.T) {
	usage := usageForCommand(t, "release", "run")

	for _, expected := range []string{
		"asc release stage",
		"asc review submissions-create",
		"asc review items-add",
		"asc review submissions-submit",
		"asc publish appstore --submit",
	} {
		if !strings.Contains(usage, expected) {
			t.Fatalf("expected release run help to mention %q, got %q", expected, usage)
		}
	}
}
