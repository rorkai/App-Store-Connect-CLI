package builds

import (
	"context"
	"strings"
	"testing"
)

func TestBuildsListCommand_AllowsIndependentVersionAndBuildNumber(t *testing.T) {
	isolateBuildsAuthEnv(t)

	cmd := BuildsListCommand()
	if err := cmd.FlagSet.Parse([]string{"--app", "123456789", "--version", "1.2.3", "--build-number", "456"}); err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	err := cmd.Exec(context.Background(), []string{})
	if err != nil && strings.Contains(err.Error(), "--version and --build-number must match when both are set") {
		t.Fatalf("expected --version and --build-number to be independent filters, got %v", err)
	}
}

func TestBuildsListCommand_VersionFlagDescriptions(t *testing.T) {
	cmd := BuildsListCommand()

	versionFlag := cmd.FlagSet.Lookup("version")
	if versionFlag == nil {
		t.Fatal("expected --version flag to be defined")
	}
	if !strings.Contains(versionFlag.Usage, "CFBundleShortVersionString") {
		t.Fatalf("expected --version usage to describe marketing version, got %q", versionFlag.Usage)
	}

	buildNumberFlag := cmd.FlagSet.Lookup("build-number")
	if buildNumberFlag == nil {
		t.Fatal("expected --build-number flag to be defined")
	}
	if !strings.Contains(buildNumberFlag.Usage, "CFBundleVersion") {
		t.Fatalf("expected --build-number usage to describe build number, got %q", buildNumberFlag.Usage)
	}
}
