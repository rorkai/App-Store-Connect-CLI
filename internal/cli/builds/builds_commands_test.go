package builds

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestBuildsListCommand_VersionAndBuildNumberDescriptions(t *testing.T) {
	cmd := BuildsListCommand()

	versionFlag := cmd.FlagSet.Lookup("version")
	if versionFlag == nil {
		t.Fatal("expected --version flag to be defined")
		return
	}
	if !strings.Contains(versionFlag.Usage, "CFBundleShortVersionString") {
		t.Fatalf("expected --version usage to mention marketing version, got %q", versionFlag.Usage)
	}

	buildNumberFlag := cmd.FlagSet.Lookup("build-number")
	if buildNumberFlag == nil {
		t.Fatal("expected --build-number flag to be defined")
		return
	}
	if !strings.Contains(buildNumberFlag.Usage, "CFBundleVersion") {
		t.Fatalf("expected --build-number usage to mention build number, got %q", buildNumberFlag.Usage)
	}
}

func TestBuildsAddGroupsCommandDryRunHelpDescribesNoMutation(t *testing.T) {
	cmd := BuildsAddGroupsCommand()
	dryRunFlag := cmd.FlagSet.Lookup("dry-run")
	if dryRunFlag == nil {
		t.Fatal("expected --dry-run flag to be registered")
	}
	if !strings.Contains(dryRunFlag.Usage, "without adding groups") {
		t.Fatalf("expected --dry-run usage to describe no mutation, got %q", dryRunFlag.Usage)
	}
	for _, want := range []string{"--dry-run", "without adding groups", "observational and advisory", "does not predict"} {
		if !strings.Contains(cmd.LongHelp, want) {
			t.Fatalf("expected add-groups help to contain %q, got %q", want, cmd.LongHelp)
		}
	}
}

func TestBuildsListCommand_HelpMentionsCombinedFilters(t *testing.T) {
	cmd := BuildsListCommand()
	if !strings.Contains(cmd.LongHelp, `--version "1.2.3" --build-number "123"`) {
		t.Fatalf("expected long help to include combined version/build-number example, got %q", cmd.LongHelp)
	}
}

func TestBuildsListQueryFlagsAreRegistered(t *testing.T) {
	for _, test := range []struct {
		name string
		cmd  *ffcli.Command
		flag string
	}{
		{name: "list beta review state", cmd: BuildsListCommand(), flag: "beta-review-state"},
		{name: "list include", cmd: BuildsListCommand(), flag: "include"},
		{name: "count beta review state", cmd: BuildsCountCommand(), flag: "beta-review-state"},
	} {
		t.Run(test.name, func(t *testing.T) {
			queryFlag := test.cmd.FlagSet.Lookup(test.flag)
			if queryFlag == nil {
				t.Fatalf("%s flag is not registered", test.flag)
			}
		})
	}
}

func TestBuildsUploadCommand_HelpShowsConcurrencyDefaultOnce(t *testing.T) {
	cmd := BuildsUploadCommand()
	usage := cmd.UsageFunc(cmd)

	var concurrencyLine string
	for line := range strings.SplitSeq(usage, "\n") {
		if strings.Contains(line, "--concurrency") {
			concurrencyLine = line
			break
		}
	}
	if concurrencyLine == "" {
		t.Fatalf("expected rendered help to include --concurrency, got %q", usage)
	}
	if count := strings.Count(concurrencyLine, "default"); count != 1 {
		t.Fatalf("expected one concurrency default annotation, got %q", concurrencyLine)
	}
	wantDefault := fmt.Sprintf("(default: %d)", asc.DefaultUploadConcurrency)
	if !strings.Contains(concurrencyLine, wantDefault) {
		t.Fatalf("expected concurrency default annotation %q, got %q", wantDefault, concurrencyLine)
	}
}

func TestBuildsUploadCommandHasExplicitSensitiveOutputOptIn(t *testing.T) {
	cmd := BuildsUploadCommand()
	flag := cmd.FlagSet.Lookup("include-sensitive")
	if flag == nil {
		t.Fatal("expected --include-sensitive to be registered")
	}
	if !strings.Contains(flag.Usage, "secret") {
		t.Fatalf("expected sensitive-output warning in flag usage, got %q", flag.Usage)
	}
}

func TestBuildsUploadCommandAppHelpDescribesExactIPAIdentityLookup(t *testing.T) {
	cmd := BuildsUploadCommand()
	appFlag := cmd.FlagSet.Lookup("app")
	if appFlag == nil {
		t.Fatal("expected --app flag to be registered")
	}
	for _, want := range []string{"IPA uploads", "exact bundle ID", "exact name"} {
		if !strings.Contains(appFlag.Usage, want) {
			t.Fatalf("expected --app usage to include %q, got %q", want, appFlag.Usage)
		}
	}
}

func TestBuildsUploadCommandTestNotesHelpDescribesDiscoveryOnlyWait(t *testing.T) {
	cmd := BuildsUploadCommand()
	testNotesFlag := cmd.FlagSet.Lookup("test-notes")
	if testNotesFlag == nil {
		t.Fatal("expected --test-notes flag to be registered")
	}
	if !strings.Contains(testNotesFlag.Usage, "build discovery") {
		t.Fatalf("expected --test-notes usage to describe discovery wait, got %q", testNotesFlag.Usage)
	}
	if !strings.Contains(cmd.LongHelp, "Add --wait") {
		t.Fatalf("expected long help to explain the optional processing wait, got %q", cmd.LongHelp)
	}
}

func TestBuildsListCommand_ProcessingStateFlagDescription(t *testing.T) {
	cmd := BuildsListCommand()

	processingStateFlag := cmd.FlagSet.Lookup("processing-state")
	if processingStateFlag == nil {
		t.Fatal("expected --processing-state flag to be defined")
		return
	}
	if !strings.Contains(processingStateFlag.Usage, "VALID") || !strings.Contains(processingStateFlag.Usage, "all") {
		t.Fatalf("expected --processing-state usage to mention supported values, got %q", processingStateFlag.Usage)
	}
}

func TestBuildsListCommand_ExcludeExpiredFlagsExist(t *testing.T) {
	cmd := BuildsListCommand()

	excludeExpiredFlag := cmd.FlagSet.Lookup("exclude-expired")
	if excludeExpiredFlag == nil {
		t.Fatal("expected --exclude-expired flag to be defined")
	}
	if !strings.Contains(excludeExpiredFlag.Usage, "expired") {
		t.Fatalf("expected --exclude-expired usage to mention expired builds, got %q", excludeExpiredFlag.Usage)
	}

	if cmd.FlagSet.Lookup("not-expired") == nil {
		t.Fatal("expected --not-expired alias to be defined")
	}
	if !strings.Contains(cmd.LongHelp, "--exclude-expired") {
		t.Fatalf("expected long help to include --exclude-expired example, got %q", cmd.LongHelp)
	}
}

func TestNormalizeBuildProcessingStateFilter(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			name:  "empty",
			input: "",
			want:  nil,
		},
		{
			name:  "single state",
			input: "processing",
			want:  []string{asc.BuildProcessingStateProcessing},
		},
		{
			name:  "all expands",
			input: "all",
			want: []string{
				asc.BuildProcessingStateProcessing,
				asc.BuildProcessingStateFailed,
				asc.BuildProcessingStateInvalid,
				asc.BuildProcessingStateValid,
			},
		},
		{
			name:    "all combined invalid",
			input:   "all,valid",
			wantErr: true,
		},
		{
			name:    "unknown invalid",
			input:   "foo",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeBuildProcessingStateFilter(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected flag.ErrHelp usage error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeBuildProcessingStateFilter() error: %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("normalizeBuildProcessingStateFilter() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestBuildsUpdateCommand_Shape(t *testing.T) {
	cmd := BuildsUpdateCommand()
	if cmd.Name != "update" {
		t.Fatalf("unexpected command name: %q", cmd.Name)
	}

	buildIDFlag := cmd.FlagSet.Lookup("build-id")
	if buildIDFlag == nil {
		t.Fatal("expected --build-id flag to be defined")
	}

	if buildFlag := cmd.FlagSet.Lookup("build"); buildFlag != nil {
		t.Fatal("expected removed --build alias to be undefined")
	}

	appFlag := cmd.FlagSet.Lookup("app")
	if appFlag == nil {
		t.Fatal("expected --app flag to be defined")
	}

	latestFlag := cmd.FlagSet.Lookup("latest")
	if latestFlag == nil {
		t.Fatal("expected --latest flag to be defined")
	}

	buildNumberFlag := cmd.FlagSet.Lookup("build-number")
	if buildNumberFlag == nil {
		t.Fatal("expected --build-number flag to be defined")
	}

	encFlag := cmd.FlagSet.Lookup("uses-non-exempt-encryption")
	if encFlag == nil {
		t.Fatal("expected --uses-non-exempt-encryption flag to be defined")
	}
}

func TestBuildSelectorCommandsExposeExcludeExpiredFlags(t *testing.T) {
	tests := []struct {
		name string
		cmd  func() *ffcli.Command
	}{
		{name: "add-groups", cmd: BuildsAddGroupsCommand},
		{name: "dsyms", cmd: BuildsDsymsCommand},
		{name: "update", cmd: BuildsUpdateCommand},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			flags := test.cmd().FlagSet
			if flags.Lookup("exclude-expired") == nil {
				t.Fatal("expected --exclude-expired flag to be defined")
			}
			if flags.Lookup("not-expired") == nil {
				t.Fatal("expected --not-expired flag to be defined")
			}
		})
	}
}

func TestBuildsUpdateCommand_ShortUsageShowsRequiredFlag(t *testing.T) {
	cmd := BuildsUpdateCommand()
	want := "asc builds update (--build-id BUILD_ID | --app APP --latest | --app APP --build-number BUILD_NUMBER --platform PLATFORM [--version VERSION]) --uses-non-exempt-encryption [true|false] [flags]"
	if cmd.ShortUsage != want {
		t.Fatalf("expected ShortUsage %q, got %q", want, cmd.ShortUsage)
	}
}

func TestBuildUpdateAttributes_JSONMarshal(t *testing.T) {
	v := false
	attrs := asc.BuildUpdateAttributes{
		UsesNonExemptEncryption: &v,
	}
	data, err := json.Marshal(attrs)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, `"usesNonExemptEncryption":false`) {
		t.Fatalf("expected usesNonExemptEncryption in JSON, got %q", got)
	}

	// Nil fields should be omitted
	empty := asc.BuildUpdateAttributes{}
	data, err = json.Marshal(empty)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	if string(data) != "{}" {
		t.Fatalf("expected empty JSON object, got %q", string(data))
	}
}
