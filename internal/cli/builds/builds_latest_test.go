package builds

import (
	"context"
	"errors"
	"flag"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func isolateBuildsAuthEnv(t *testing.T) {
	t.Helper()

	// Keep tests hermetic: avoid loading host keychain/config/env credentials.
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_STRICT_AUTH", "")
}

func TestBuildsLatestCommand_MissingApp(t *testing.T) {
	isolateBuildsAuthEnv(t)

	// Clear env var to ensure --app is required
	t.Setenv("ASC_APP_ID", "")

	cmd := BuildsLatestCommand()

	err := cmd.Exec(context.Background(), []string{})
	if !errors.Is(err, flag.ErrHelp) {
		t.Errorf("expected flag.ErrHelp when --app is missing, got %v", err)
	}
}

func TestBuildsLatestCommand_InvalidPlatform(t *testing.T) {
	isolateBuildsAuthEnv(t)

	cmd := BuildsLatestCommand()

	// Parse flags first
	if err := cmd.FlagSet.Parse([]string{"--app", "123", "--platform", "INVALID"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if !errors.Is(err, flag.ErrHelp) {
		t.Errorf("expected flag.ErrHelp for invalid platform, got %v", err)
	}
}

func TestBuildsLatestCommand_ValidPlatforms(t *testing.T) {
	isolateBuildsAuthEnv(t)

	validPlatforms := []string{"IOS", "MAC_OS", "TV_OS", "VISION_OS", "ios", "mac_os"}

	for _, platform := range validPlatforms {
		t.Run(platform, func(t *testing.T) {
			isolateBuildsAuthEnv(t)

			cmd := BuildsLatestCommand()

			// Parse flags - this should not error for valid platforms
			if err := cmd.FlagSet.Parse([]string{"--app", "123", "--platform", platform}); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			// The command will fail because there's no real client, but it should get past validation
			err := cmd.Exec(context.Background(), []string{})

			// Should not be flag.ErrHelp for valid platforms (will fail later due to no auth)
			if errors.Is(err, flag.ErrHelp) {
				t.Errorf("platform %s should be valid but got flag.ErrHelp", platform)
			}
		})
	}
}

func TestBuildsLatestCommand_InvalidInitialBuildNumber(t *testing.T) {
	isolateBuildsAuthEnv(t)

	cmd := BuildsLatestCommand()

	if err := cmd.FlagSet.Parse([]string{"--app", "123", "--next", "--initial-build-number", "0"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if !errors.Is(err, flag.ErrHelp) {
		t.Errorf("expected flag.ErrHelp for invalid initial build number, got %v", err)
	}
}

func TestBuildsLatestCommand_NotExpiredAndExcludeExpiredTogether(t *testing.T) {
	isolateBuildsAuthEnv(t)

	cmd := BuildsLatestCommand()

	if err := cmd.FlagSet.Parse([]string{"--app", "123", "--exclude-expired", "--not-expired"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if errors.Is(err, flag.ErrHelp) {
		t.Errorf("expected non-help execution path when both alias flags are set, got %v", err)
	}
}

func TestBuildsLatestCommand_FlagDefinitions(t *testing.T) {
	isolateBuildsAuthEnv(t)

	cmd := BuildsLatestCommand()

	// Verify all expected flags exist
	expectedFlags := []string{"app", "version", "platform", "output", "pretty", "next", "initial-build-number", "exclude-expired", "not-expired"}
	for _, name := range expectedFlags {
		f := cmd.FlagSet.Lookup(name)
		if f == nil {
			t.Errorf("expected flag --%s to be defined", name)
		}
	}

	// Verify default values
	if f := cmd.FlagSet.Lookup("output"); f != nil && f.DefValue != "json" {
		t.Errorf("expected --output default to be 'json', got %q", f.DefValue)
	}
	if f := cmd.FlagSet.Lookup("pretty"); f != nil && f.DefValue != "false" {
		t.Errorf("expected --pretty default to be 'false', got %q", f.DefValue)
	}
	if f := cmd.FlagSet.Lookup("next"); f != nil && f.DefValue != "false" {
		t.Errorf("expected --next default to be 'false', got %q", f.DefValue)
	}
	if f := cmd.FlagSet.Lookup("initial-build-number"); f != nil && f.DefValue != "1" {
		t.Errorf("expected --initial-build-number default to be '1', got %q", f.DefValue)
	}
	if f := cmd.FlagSet.Lookup("exclude-expired"); f != nil && f.DefValue != "false" {
		t.Errorf("expected --exclude-expired default to be 'false', got %q", f.DefValue)
	}
	if f := cmd.FlagSet.Lookup("not-expired"); f != nil && f.DefValue != "false" {
		t.Errorf("expected --not-expired default to be 'false', got %q", f.DefValue)
	}
}

func TestBuildsLatestCommand_HelpMentionsExcludeExpired(t *testing.T) {
	cmd := BuildsLatestCommand()
	if !strings.Contains(cmd.LongHelp, "--exclude-expired") {
		t.Fatalf("expected help text to mention --exclude-expired")
	}
}

func TestBuildsLatestCommand_UsesAppIDEnv(t *testing.T) {
	isolateBuildsAuthEnv(t)

	// Set env var
	t.Setenv("ASC_APP_ID", "env-app-id")

	cmd := BuildsLatestCommand()

	// Don't pass --app flag
	if err := cmd.FlagSet.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})

	// Should not be flag.ErrHelp since env var provides the app ID
	if errors.Is(err, flag.ErrHelp) {
		t.Errorf("should use ASC_APP_ID env var but got flag.ErrHelp")
	}
}

// TestSelectNewestBuild verifies that the multi-preReleaseVersion selection
// logic correctly picks the build with the newest uploadedDate.
func TestSelectNewestBuild(t *testing.T) {
	// Simulate builds from different preReleaseVersions with different dates
	builds := []asc.Resource[asc.BuildAttributes]{
		{
			ID: "build-older",
			Attributes: asc.BuildAttributes{
				Version:      "1.0",
				UploadedDate: "2026-01-15T10:00:00Z",
			},
		},
		{
			ID: "build-newest",
			Attributes: asc.BuildAttributes{
				Version:      "2.0",
				UploadedDate: "2026-01-20T10:00:00Z",
			},
		},
		{
			ID: "build-middle",
			Attributes: asc.BuildAttributes{
				Version:      "1.5",
				UploadedDate: "2026-01-18T10:00:00Z",
			},
		},
	}

	// The selection logic: find the build with the newest uploadedDate
	var newestBuild *asc.Resource[asc.BuildAttributes]
	var newestDate string

	for i := range builds {
		if newestBuild == nil || builds[i].Attributes.UploadedDate > newestDate {
			newestBuild = &builds[i]
			newestDate = builds[i].Attributes.UploadedDate
		}
	}

	if newestBuild == nil {
		t.Fatal("expected to find a newest build")
	}
	if newestBuild.ID != "build-newest" {
		t.Errorf("expected build-newest to be selected, got %s", newestBuild.ID)
	}
	if newestDate != "2026-01-20T10:00:00Z" {
		t.Errorf("expected newest date 2026-01-20T10:00:00Z, got %s", newestDate)
	}
}

// TestSelectNewestBuild_OlderVersionCanBeNewer verifies that an older version
// string (e.g., "1.0") can have a newer uploadedDate than a higher version (e.g., "2.0").
// This tests the scenario where someone uploads a hotfix to an older version.
func TestSelectNewestBuild_OlderVersionCanBeNewer(t *testing.T) {
	builds := []asc.Resource[asc.BuildAttributes]{
		{
			ID: "build-v2-old",
			Attributes: asc.BuildAttributes{
				Version:      "2.0",
				UploadedDate: "2026-01-10T10:00:00Z", // Version 2.0 uploaded earlier
			},
		},
		{
			ID: "build-v1-hotfix",
			Attributes: asc.BuildAttributes{
				Version:      "1.0",
				UploadedDate: "2026-01-20T10:00:00Z", // Version 1.0 hotfix uploaded later
			},
		},
	}

	var newestBuild *asc.Resource[asc.BuildAttributes]
	var newestDate string

	for i := range builds {
		if newestBuild == nil || builds[i].Attributes.UploadedDate > newestDate {
			newestBuild = &builds[i]
			newestDate = builds[i].Attributes.UploadedDate
		}
	}

	// The 1.0 hotfix should be selected because it was uploaded more recently
	if newestBuild.ID != "build-v1-hotfix" {
		t.Errorf("expected build-v1-hotfix (newer upload) to be selected, got %s", newestBuild.ID)
	}
}

func TestParseBuildNumberRejectsNonNumeric(t *testing.T) {
	_, err := parseBuildNumber("1a", "processed build")
	if err == nil {
		t.Fatal("expected error for non-numeric build number")
	}
	if !strings.Contains(err.Error(), "processed build") {
		t.Fatalf("expected error to mention source, got %v", err)
	}
}

func TestParseBuildNumberRejectsEmpty(t *testing.T) {
	_, err := parseBuildNumber(" ", "build upload")
	if err == nil {
		t.Fatal("expected error for empty build number")
	}
}

func TestParseBuildNumberAllowsNumeric(t *testing.T) {
	got, err := parseBuildNumber("42", "processed build")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.String() != "42" {
		t.Fatalf("expected 42, got %q", got.String())
	}
}

func TestParseBuildNumberAllowsDotSeparatedNumeric(t *testing.T) {
	got, err := parseBuildNumber("1.2.3", "build upload")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.String() != "1.2.3" {
		t.Fatalf("expected 1.2.3, got %q", got.String())
	}
}

func TestBuildNumberNextIncrementsLastSegment(t *testing.T) {
	parsed, err := parseBuildNumber("1.2.3", "processed build")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	next, err := parsed.Next()
	if err != nil {
		t.Fatalf("unexpected error incrementing build number: %v", err)
	}
	if next.String() != "1.2.4" {
		t.Fatalf("expected next build number 1.2.4, got %q", next.String())
	}
}
