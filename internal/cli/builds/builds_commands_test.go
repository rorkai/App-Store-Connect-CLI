package builds

import (
	"strings"
	"testing"
)

func TestBuildsListCommand_VersionAndBuildNumberDescriptions(t *testing.T) {
	cmd := BuildsListCommand()

	versionFlag := cmd.FlagSet.Lookup("version")
	if versionFlag == nil {
		t.Fatal("expected --version flag to be defined")
	}
	if !strings.Contains(versionFlag.Usage, "CFBundleShortVersionString") {
		t.Fatalf("expected --version usage to mention marketing version, got %q", versionFlag.Usage)
	}

	buildNumberFlag := cmd.FlagSet.Lookup("build-number")
	if buildNumberFlag == nil {
		t.Fatal("expected --build-number flag to be defined")
	}
	if !strings.Contains(buildNumberFlag.Usage, "CFBundleVersion") {
		t.Fatalf("expected --build-number usage to mention build number, got %q", buildNumberFlag.Usage)
	}
}

func TestBuildsListCommand_HelpMentionsCombinedFilters(t *testing.T) {
	cmd := BuildsListCommand()
	if !strings.Contains(cmd.LongHelp, `--version "1.2.3" --build-number "123"`) {
		t.Fatalf("expected long help to include combined version/build-number example, got %q", cmd.LongHelp)
	}
}
