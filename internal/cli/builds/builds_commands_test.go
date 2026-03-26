package builds

import (
	"encoding/json"
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

func TestBuildsUpdateCommand_Shape(t *testing.T) {
	cmd := BuildsUpdateCommand()
	if cmd.Name != "update" {
		t.Fatalf("unexpected command name: %q", cmd.Name)
	}

	buildFlag := cmd.FlagSet.Lookup("build")
	if buildFlag == nil {
		t.Fatal("expected --build flag to be defined")
	}

	encFlag := cmd.FlagSet.Lookup("uses-non-exempt-encryption")
	if encFlag == nil {
		t.Fatal("expected --uses-non-exempt-encryption flag to be defined")
	}
}

func TestBuildsUpdateCommand_HelpContainsExamples(t *testing.T) {
	cmd := BuildsUpdateCommand()
	if !strings.Contains(cmd.LongHelp, "--uses-non-exempt-encryption=false") {
		t.Fatalf("expected long help to include encryption example, got %q", cmd.LongHelp)
	}
}

func TestBuildsUpdateCommand_ShortUsageShowsRequiredFlag(t *testing.T) {
	cmd := BuildsUpdateCommand()
	want := "asc builds update --build BUILD_ID --uses-non-exempt-encryption [true|false] [flags]"
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
