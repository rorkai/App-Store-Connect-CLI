package builds

import (
	"errors"
	"flag"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
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

func TestBuildsListCommand_ProcessingStateFlagDescription(t *testing.T) {
	cmd := BuildsListCommand()

	processingStateFlag := cmd.FlagSet.Lookup("processing-state")
	if processingStateFlag == nil {
		t.Fatal("expected --processing-state flag to be defined")
	}
	if !strings.Contains(processingStateFlag.Usage, "VALID") || !strings.Contains(processingStateFlag.Usage, "all") {
		t.Fatalf("expected --processing-state usage to mention supported values, got %q", processingStateFlag.Usage)
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
