package betabuildlocalizations

import (
	"errors"
	"flag"
	"reflect"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestBetaBuildLocalizationsCommandConstructors(t *testing.T) {
	top := BetaBuildLocalizationsCommand()
	if top == nil {
		t.Fatal("expected beta-build-localizations command")
		return
	}
	if top.Name == "" {
		t.Fatal("expected command name")
	}
	if len(top.Subcommands) == 0 {
		t.Fatal("expected subcommands")
	}

	if got := BetaBuildLocalizationsCommand(); got == nil {
		t.Fatal("expected Command wrapper to return command")
	}
	if got := BetaBuildLocalizationsBuildCommand(); got == nil {
		t.Fatal("expected build relationship command")
	}
}

func TestBetaBuildLocalizationsCreateCommandUpsertFlag(t *testing.T) {
	cmd := BetaBuildLocalizationsCreateCommand()
	if cmd == nil {
		t.Fatal("expected create command")
		return
	}

	upsertFlag := cmd.FlagSet.Lookup("upsert")
	if upsertFlag == nil {
		t.Fatal("expected --upsert flag")
		return
	}
	if !strings.Contains(upsertFlag.Usage, "Create-or-update") {
		t.Fatalf("expected --upsert usage text, got %q", upsertFlag.Usage)
	}
}

func TestBetaBuildLocalizationsGetAndCreateLatestFlags(t *testing.T) {
	getCmd := BetaBuildLocalizationsGetCommand()
	if getCmd == nil {
		t.Fatal("expected get command")
	}
	for _, name := range []string{"app", "latest", "state", "locale"} {
		if getCmd.FlagSet.Lookup(name) == nil {
			t.Fatalf("expected get flag --%s", name)
		}
	}

	createCmd := BetaBuildLocalizationsCreateCommand()
	if createCmd == nil {
		t.Fatal("expected create command")
	}
	for _, name := range []string{"app", "latest", "state"} {
		if createCmd.FlagSet.Lookup(name) == nil {
			t.Fatalf("expected create flag --%s", name)
		}
	}
}

func TestNormalizeLatestBuildProcessingStateFilter(t *testing.T) {
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
			name:  "single processing",
			input: "processing",
			want:  []string{asc.BuildProcessingStateProcessing},
		},
		{
			name:  "complete alias maps to valid",
			input: "complete",
			want:  []string{asc.BuildProcessingStateValid},
		},
		{
			name:  "processing and complete",
			input: "processing,complete",
			want:  []string{asc.BuildProcessingStateProcessing, asc.BuildProcessingStateValid},
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
			name:    "all mixed invalid",
			input:   "all,processing",
			wantErr: true,
		},
		{
			name:    "unknown invalid",
			input:   "nope",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeLatestBuildProcessingStateFilter(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("expected flag.ErrHelp usage error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("normalizeLatestBuildProcessingStateFilter()=%v want %v", got, tt.want)
			}
		})
	}
}
